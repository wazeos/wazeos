package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/wazeos/wazeos/internal/types"
)

// NativeS3Driver is a native Go implementation of the S3 driver
type NativeS3Driver struct{}

// NewNativeS3Driver creates a new native S3 driver
func NewNativeS3Driver() *NativeS3Driver {
	return &NativeS3Driver{}
}

// Name returns the driver name
func (d *NativeS3Driver) Name() string {
	return "io.resource"
}

// Patterns returns URI patterns this driver handles
func (d *NativeS3Driver) Patterns() []string {
	return []string{"s3://**"}
}

// HandleCall processes S3 resource calls
func (d *NativeS3Driver) HandleCall(ctx context.Context, call *types.ResourceCall) (*types.ResourceResult, error) {
	// Extract credentials from X-WazeOS-Credentials header
	credsJSON := call.Headers["X-WazeOS-Credentials"]
	if credsJSON == "" {
		return &types.ResourceResult{
			StatusCode: 401,
			Body:       []byte(`{"error":"missing credentials"}`),
		}, nil
	}

	var creds map[string]string
	if err := json.Unmarshal([]byte(credsJSON), &creds); err != nil {
		return &types.ResourceResult{
			StatusCode: 400,
			Body:       []byte(fmt.Sprintf(`{"error":"invalid credentials: %v"}`, err)),
		}, nil
	}

	accessKeyID := creds["access_key_id"]
	secretAccessKey := creds["secret_access_key"]
	if accessKeyID == "" || secretAccessKey == "" {
		return &types.ResourceResult{
			StatusCode: 401,
			Body:       []byte(`{"error":"missing access_key_id or secret_access_key"}`),
		}, nil
	}

	// Parse S3 URI: s3://bucket.s3.region.amazonaws.com/key
	uri := strings.TrimPrefix(call.URI, "s3://")
	parts := strings.SplitN(uri, "/", 2)
	if len(parts) != 2 {
		return &types.ResourceResult{
			StatusCode: 400,
			Body:       []byte(`{"error":"invalid S3 URI format, expected s3://bucket/key"}`),
		}, nil
	}

	bucketFQDN := parts[0]
	key := parts[1]

	// Parse bucket FQDN to extract bucket and region
	bucket, region := parseBucketFQDN(bucketFQDN)
	if region == "" {
		region = "us-east-1" // Default region
	}

	// Create AWS config with credentials
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accessKeyID,
			secretAccessKey,
			"", // session token (optional)
		)),
	)
	if err != nil {
		return &types.ResourceResult{
			StatusCode: 500,
			Body:       []byte(fmt.Sprintf(`{"error":"failed to load AWS config: %v"}`, err)),
		}, nil
	}

	// Create S3 client
	client := s3.NewFromConfig(cfg)

	// Determine operation from permissions
	// Check permissions to determine which operation to perform
	hasWrite := false
	hasRead := false
	hasDelete := false
	hasList := false

	for _, perm := range call.Permissions {
		switch strings.ToLower(perm) {
		case "write", "put":
			hasWrite = true
		case "read", "get":
			hasRead = true
		case "delete":
			hasDelete = true
		case "list":
			hasList = true
		}
	}

	// Priority order: write > delete > list > read (most specific to least specific)
	if hasWrite {
		return d.handleWrite(client, bucket, key, call.Body)
	} else if hasDelete {
		return d.handleDelete(client, bucket, key)
	} else if hasList {
		return d.handleList(client, bucket, key)
	} else if hasRead {
		return d.handleRead(client, bucket, key)
	}

	return &types.ResourceResult{
		StatusCode: 403,
		Body:       []byte(`{"error":"no valid operation permission provided"}`),
	}, nil
}

// parseBucketFQDN extracts bucket name and region from FQDN
func parseBucketFQDN(fqdn string) (bucket, region string) {
	parts := strings.Split(fqdn, ".")

	if len(parts) == 1 {
		// Simple bucket name
		return fqdn, ""
	}

	bucket = parts[0]

	// Format 1: bucket.s3.region.amazonaws.com
	if len(parts) >= 4 && parts[1] == "s3" && strings.Contains(parts[3], "amazonaws") {
		region = parts[2]
		return bucket, region
	}

	// Format 2: bucket.s3-region.amazonaws.com
	if len(parts) >= 3 && strings.HasPrefix(parts[1], "s3-") {
		region = strings.TrimPrefix(parts[1], "s3-")
		return bucket, region
	}

	// Format 3: bucket.s3.amazonaws.com (us-east-1)
	return bucket, ""
}

func (d *NativeS3Driver) handleWrite(client *s3.Client, bucket, key string, data []byte) (*types.ResourceResult, error) {
	_, err := client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		return &types.ResourceResult{
			StatusCode: 500,
			Body:       []byte(fmt.Sprintf(`{"error":"S3 PutObject failed: %v"}`, err)),
		}, nil
	}

	response := map[string]interface{}{
		"success": true,
		"bucket":  bucket,
		"key":     key,
		"size":    len(data),
	}
	responseJSON, _ := json.Marshal(response)

	return &types.ResourceResult{
		StatusCode: 200,
		Body:       responseJSON,
	}, nil
}

func (d *NativeS3Driver) handleRead(client *s3.Client, bucket, key string) (*types.ResourceResult, error) {
	result, err := client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return &types.ResourceResult{
			StatusCode: 404,
			Body:       []byte(fmt.Sprintf(`{"error":"S3 GetObject failed: %v"}`, err)),
		}, nil
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return &types.ResourceResult{
			StatusCode: 500,
			Body:       []byte(fmt.Sprintf(`{"error":"failed to read object: %v"}`, err)),
		}, nil
	}

	return &types.ResourceResult{
		StatusCode: 200,
		Body:       data,
	}, nil
}

func (d *NativeS3Driver) handleDelete(client *s3.Client, bucket, key string) (*types.ResourceResult, error) {
	_, err := client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return &types.ResourceResult{
			StatusCode: 500,
			Body:       []byte(fmt.Sprintf(`{"error":"S3 DeleteObject failed: %v"}`, err)),
		}, nil
	}

	response := map[string]interface{}{
		"success": true,
		"bucket":  bucket,
		"key":     key,
		"deleted": true,
	}
	responseJSON, _ := json.Marshal(response)

	return &types.ResourceResult{
		StatusCode: 200,
		Body:       responseJSON,
	}, nil
}

func (d *NativeS3Driver) handleList(client *s3.Client, bucket, prefix string) (*types.ResourceResult, error) {
	result, err := client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return &types.ResourceResult{
			StatusCode: 500,
			Body:       []byte(fmt.Sprintf(`{"error":"S3 ListObjects failed: %v"}`, err)),
		}, nil
	}

	objects := make([]map[string]interface{}, 0, len(result.Contents))
	for _, obj := range result.Contents {
		objects = append(objects, map[string]interface{}{
			"key":          *obj.Key,
			"size":         *obj.Size,
			"lastModified": obj.LastModified.String(),
		})
	}

	response := map[string]interface{}{
		"success": true,
		"bucket":  bucket,
		"objects": objects,
		"count":   len(objects),
	}
	responseJSON, _ := json.Marshal(response)

	return &types.ResourceResult{
		StatusCode: 200,
		Body:       responseJSON,
	}, nil
}
