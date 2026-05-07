package app

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/wazeos/wazeos/sdk/driver"
)

// IOClient provides high-level I/O operations for apps.
type IOClient interface {
	// Deprecated: Use IO() instead
	ReadFile(path string) ([]byte, error)
	// Deprecated: Use IO() instead
	WriteFile(path string, data []byte) error
	// Deprecated: Use IO() instead
	DeleteFile(path string) error
	// Deprecated: Use IO() instead
	ListFiles(dir string) ([]string, error)
	// Deprecated: Use IO() instead
	Get(url string) (*HTTPResponse, error)
	// Deprecated: Use IO() instead
	Post(url string, body []byte, headers map[string]string) (*HTTPResponse, error)
	// Deprecated: Use IO() instead
	Request(method, url string, body []byte, headers map[string]string) (*HTTPResponse, error)
	// Deprecated: Use IO() instead
	CallApp(appName string, args ...string) (*Response, error)
	// Deprecated: Use IO() instead
	CallAppWithInput(appName string, input []byte, args ...string) (*Response, error)
	// Deprecated: Use IO() instead
	Publish(topic string, message []byte) error
	// Deprecated: Use IO() instead
	PublishWithKey(topic, key string, message []byte) error
	// Deprecated: Use IO() instead
	Consume(topic string, opts *ConsumeOptions) ([]*Message, error)
	// Deprecated: Use IO() instead
	Call(uri, method string, body []byte, headers map[string]string) (*driver.ResourceResult, error)
}

// IOOperation represents a pending I/O operation with required permissions.
type IOOperation struct {
	ctx         *Context
	uri         string
	permissions []string
}

// Call executes the I/O operation with the given arguments.
// Args are driver-specific and passed as a map for flexibility.
//
// Example usage:
//
//	// File read
//	result, err := ctx.IO("file:///tmp/config.txt", []string{"read"}).Call(nil)
//
//	// File write
//	err := ctx.IO("file:///tmp/config.txt", []string{"write"}).Call(map[string]interface{}{
//	    "data": []byte("content"),
//	})
//
//	// HTTP request
//	result, err := ctx.IO("https://api.example.com/data", []string{"POST"}).Call(map[string]interface{}{
//	    "body": []byte("data"),
//	    "headers": map[string]string{"Content-Type": "application/json"},
//	})
//
//	// App call
//	result, err := ctx.IO("fn://wazeos/logger", []string{"invoke"}).Call(map[string]interface{}{
//	    "level": "info",
//	    "message": "test",
//	})
func (op *IOOperation) Call(args map[string]interface{}) (map[string]interface{}, error) {
	// Convert permissions to AccessBits for the driver call
	var accessMode driver.AccessBits
	for _, perm := range op.permissions {
		// Map permission strings to access bits
		switch strings.ToLower(perm) {
		case "read", "get", "list", "consume":
			accessMode |= driver.AccessRead
		case "write", "post", "put", "patch", "create", "produce", "delete":
			accessMode |= driver.AccessWrite
		case "invoke", "execute":
			accessMode |= driver.AccessExecute
		default:
			// For driver-specific permissions (like HTTP methods), use read by default
			// The driver will validate the specific permission
			accessMode |= driver.AccessRead
		}
	}

	// Encode args as JSON for the driver call
	var body []byte
	var headers map[string]string
	if args != nil {
		// Extract common fields if present
		if h, ok := args["headers"].(map[string]string); ok {
			headers = h
		} else {
			headers = make(map[string]string)
		}

		// Extract body if present
		if b, ok := args["body"].([]byte); ok {
			body = b
		} else if b, ok := args["data"].([]byte); ok {
			body = b
		} else {
			// Encode entire args as JSON
			var err error
			body, err = json.Marshal(args)
			if err != nil {
				return nil, WrapError(err, "IO_ERROR", "failed to encode arguments", 500)
			}
		}
	} else {
		headers = make(map[string]string)
	}

	// Determine method from permissions (first permission is typically the method)
	method := "CALL"
	if len(op.permissions) > 0 {
		method = strings.ToUpper(op.permissions[0])
	}

	// Make the driver call
	result, err := driver.CallResourceCall(&driver.ResourceCall{
		URI:        op.uri,
		Method:     method,
		Headers:    headers,
		Body:       body,
		AccessMode: accessMode,
	})

	if err != nil {
		return nil, WrapError(err, "IO_ERROR", "failed to execute I/O operation", 500)
	}

	// Return error if status indicates failure
	if result.StatusCode >= 400 {
		return nil, NewError("IO_ERROR", getErrorMessage(result), result.StatusCode)
	}

	// Parse result body as JSON if possible
	var resultMap map[string]interface{}
	if len(result.Body) > 0 {
		if err := json.Unmarshal(result.Body, &resultMap); err != nil {
			// If not JSON, return raw body
			resultMap = map[string]interface{}{
				"body":       result.Body,
				"statusCode": result.StatusCode,
				"headers":    result.Headers,
			}
		} else {
			// Add metadata to result
			resultMap["statusCode"] = result.StatusCode
			resultMap["headers"] = result.Headers
		}
	} else {
		resultMap = map[string]interface{}{
			"statusCode": result.StatusCode,
			"headers":    result.Headers,
		}
	}

	return resultMap, nil
}

// realIOClient is the production implementation of IOClient.
type realIOClient struct {
	ctx *Context
}

// HTTPResponse represents an HTTP response.
type HTTPResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

// Message represents a message from a queue or topic.
type Message struct {
	ID        string
	Topic     string
	Key       string
	Headers   map[string]string
	Body      []byte
	Timestamp string
}

// ConsumeOptions configures message consumption.
type ConsumeOptions struct {
	MaxCount int    // Maximum number of messages to consume
	Timeout  int    // Timeout in seconds
	Group    string // Consumer group name
}

// File operations

// getErrorMessage extracts the best error message from a ResourceResult.
// Prefers the Body field when it contains more detailed information.
func getErrorMessage(result *driver.ResourceResult) string {
	// If Body contains a more detailed message, use it
	if len(result.Body) > 0 {
		bodyMsg := string(result.Body)
		// Use Body if it's different and more informative than Error
		if bodyMsg != result.Error && len(bodyMsg) > len(result.Error) {
			return bodyMsg
		}
	}
	return result.Error
}

// ReadFile reads the contents of a file.
func (io *realIOClient) ReadFile(path string) ([]byte, error) {
	result, err := driver.CallResourceCall(&driver.ResourceCall{
		URI:        fmt.Sprintf("file://%s", path),
		Method:     "READ",
		Headers:    make(map[string]string),
		Body:       nil,
		AccessMode: driver.AccessRead,
	})

	if err != nil {
		return nil, WrapError(err, "FILE_READ_ERROR", "failed to read file", 500)
	}

	if result.StatusCode != 200 {
		return nil, NewError("FILE_READ_ERROR", getErrorMessage(result), result.StatusCode)
	}

	return result.Body, nil
}

// WriteFile writes data to a file.
func (io *realIOClient) WriteFile(path string, data []byte) error {
	result, err := driver.CallResourceCall(&driver.ResourceCall{
		URI:        fmt.Sprintf("file://%s", path),
		Method:     "WRITE",
		Headers:    make(map[string]string),
		Body:       data,
		AccessMode: driver.AccessWrite,
	})

	if err != nil {
		return WrapError(err, "FILE_WRITE_ERROR", "failed to write file", 500)
	}

	if result.StatusCode != 200 {
		return NewError("FILE_WRITE_ERROR", getErrorMessage(result), result.StatusCode)
	}

	return nil
}

// DeleteFile deletes a file.
func (io *realIOClient) DeleteFile(path string) error {
	result, err := driver.CallResourceCall(&driver.ResourceCall{
		URI:        fmt.Sprintf("file://%s", path),
		Method:     "DELETE",
		Headers:    make(map[string]string),
		Body:       nil,
		AccessMode: driver.AccessWrite,
	})

	if err != nil {
		return WrapError(err, "FILE_DELETE_ERROR", "failed to delete file", 500)
	}

	if result.StatusCode != 200 {
		return NewError("FILE_DELETE_ERROR", getErrorMessage(result), result.StatusCode)
	}

	return nil
}

// ListFiles lists files in a directory.
func (io *realIOClient) ListFiles(dir string) ([]string, error) {
	result, err := driver.CallResourceCall(&driver.ResourceCall{
		URI:        fmt.Sprintf("file://%s", dir),
		Method:     "LIST",
		Headers:    make(map[string]string),
		Body:       nil,
		AccessMode: driver.AccessRead,
	})

	if err != nil {
		return nil, WrapError(err, "FILE_LIST_ERROR", "failed to list files", 500)
	}

	if result.StatusCode != 200 {
		return nil, NewError("FILE_LIST_ERROR", getErrorMessage(result), result.StatusCode)
	}

	// Parse the result body as JSON array
	var files []string
	if err := json.Unmarshal(result.Body, &files); err != nil {
		return nil, WrapError(err, "FILE_LIST_ERROR", "failed to parse file list", 500)
	}

	return files, nil
}

// HTTP operations

// Get makes an HTTP GET request.
func (io *realIOClient) Get(url string) (*HTTPResponse, error) {
	return io.Request("GET", url, nil, nil)
}

// Post makes an HTTP POST request.
func (io *realIOClient) Post(url string, body []byte, headers map[string]string) (*HTTPResponse, error) {
	return io.Request("POST", url, body, headers)
}

// Request makes an HTTP request with the given method, URL, body, and headers.
func (io *realIOClient) Request(method, url string, body []byte, headers map[string]string) (*HTTPResponse, error) {
	if headers == nil {
		headers = make(map[string]string)
	}

	result, err := driver.CallResourceCall(&driver.ResourceCall{
		URI:        url,
		Method:     method,
		Headers:    headers,
		Body:       body,
		AccessMode: driver.AccessRead, // HTTP requests use read permission
	})

	if err != nil {
		return nil, WrapError(err, "HTTP_REQUEST_ERROR", "failed to make HTTP request", 500)
	}

	return &HTTPResponse{
		StatusCode: result.StatusCode,
		Headers:    result.Headers,
		Body:       result.Body,
	}, nil
}

// App-to-app calls

// CallApp makes an fn:// call to another app.
func (io *realIOClient) CallApp(appName string, args ...string) (*Response, error) {
	return io.CallAppWithInput(appName, nil, args...)
}

// CallAppWithInput makes an fn:// call to another app with input data.
func (io *realIOClient) CallAppWithInput(appName string, input []byte, args ...string) (*Response, error) {
	// Construct fn:// URI
	uri := fmt.Sprintf("fn://%s", appName)
	if len(args) > 0 {
		uri = fmt.Sprintf("%s/%s", uri, strings.Join(args, "/"))
	}

	result, err := driver.CallResourceCall(&driver.ResourceCall{
		URI:        uri,
		Method:     "INVOKE",
		Headers:    make(map[string]string),
		Body:       input,
		AccessMode: driver.AccessExecute,
	})

	if err != nil {
		return nil, WrapError(err, "APP_CALL_ERROR", "failed to call app", 500)
	}

	// Parse exit code from headers
	exitCode := 0
	if exitCodeStr, ok := result.Headers["X-Exit-Code"]; ok {
		if parsed, err := strconv.Atoi(exitCodeStr); err == nil {
			exitCode = parsed
		}
	}

	return &Response{
		StatusCode: result.StatusCode,
		Headers:    result.Headers,
		Body:       result.Body,
		ExitCode:   exitCode,
	}, nil
}

// Message queue operations

// Publish sends a message to a queue or topic.
func (io *realIOClient) Publish(topic string, message []byte) error {
	return io.PublishWithKey(topic, "", message)
}

// PublishWithKey sends a message to a queue with a partition key.
func (io *realIOClient) PublishWithKey(topic, key string, message []byte) error {
	uri := fmt.Sprintf("queue://%s", topic)

	// Construct IPC produce request
	req := map[string]interface{}{
		"topic": topic,
		"body":  message,
	}
	if key != "" {
		req["key"] = key
	}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return WrapError(err, "QUEUE_PUBLISH_ERROR", "failed to marshal request", 500)
	}

	result, err := driver.CallResourceCall(&driver.ResourceCall{
		URI:        uri,
		Method:     "PRODUCE",
		Headers:    make(map[string]string),
		Body:       reqJSON,
		AccessMode: driver.AccessWrite,
	})

	if err != nil {
		return WrapError(err, "QUEUE_PUBLISH_ERROR", "failed to publish message", 500)
	}

	if result.StatusCode != 200 {
		return NewError("QUEUE_PUBLISH_ERROR", getErrorMessage(result), result.StatusCode)
	}

	return nil
}

// Consume reads messages from a queue or topic.
func (io *realIOClient) Consume(topic string, opts *ConsumeOptions) ([]*Message, error) {
	if opts == nil {
		opts = &ConsumeOptions{MaxCount: 10, Timeout: 5}
	}

	uri := fmt.Sprintf("queue://%s", topic)

	// Construct IPC consume request
	req := map[string]interface{}{
		"topic":    topic,
		"maxCount": opts.MaxCount,
		"timeout":  opts.Timeout,
	}
	if opts.Group != "" {
		req["group"] = opts.Group
	}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, WrapError(err, "QUEUE_CONSUME_ERROR", "failed to marshal request", 500)
	}

	result, err := driver.CallResourceCall(&driver.ResourceCall{
		URI:        uri,
		Method:     "CONSUME",
		Headers:    make(map[string]string),
		Body:       reqJSON,
		AccessMode: driver.AccessRead,
	})

	if err != nil {
		return nil, WrapError(err, "QUEUE_CONSUME_ERROR", "failed to consume messages", 500)
	}

	if result.StatusCode != 200 {
		return nil, NewError("QUEUE_CONSUME_ERROR", getErrorMessage(result), result.StatusCode)
	}

	// Parse messages from response
	var messages []*Message
	if err := json.Unmarshal(result.Body, &messages); err != nil {
		return nil, WrapError(err, "QUEUE_CONSUME_ERROR", "failed to parse messages", 500)
	}

	return messages, nil
}

// Call makes a generic resource call (escape hatch for custom operations).
func (io *realIOClient) Call(uri, method string, body []byte, headers map[string]string) (*driver.ResourceResult, error) {
	if headers == nil {
		headers = make(map[string]string)
	}

	// Determine access mode based on method
	var accessMode driver.AccessBits
	switch strings.ToUpper(method) {
	case "READ", "GET", "LIST":
		accessMode = driver.AccessRead
	case "WRITE", "POST", "PUT", "PATCH", "CREATE":
		accessMode = driver.AccessWrite
	case "DELETE":
		accessMode = driver.AccessWrite
	case "INVOKE", "EXECUTE":
		accessMode = driver.AccessExecute
	default:
		accessMode = driver.AccessRead // Default to read
	}

	result, err := driver.CallResourceCall(&driver.ResourceCall{
		URI:        uri,
		Method:     method,
		Headers:    headers,
		Body:       body,
		AccessMode: accessMode,
	})

	if err != nil {
		return nil, WrapError(err, "RESOURCE_CALL_ERROR", "failed to make resource call", 500)
	}

	return result, nil
}
