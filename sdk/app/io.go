package app

import (
	"encoding/json"

	"github.com/wazeos/wazeos/sdk/driver"
)

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

// IOOperation represents a pending I/O operation with required permissions.
type IOOperation struct {
	ctx         *Context
	uri         string
	permissions []string
}

// Call executes the I/O operation with the given arguments.
// Args are driver-specific and passed as a map for flexibility.
//
// The method/operation should be specified in the args map when needed by the driver.
// For example, HTTP drivers may expect a "method" field, while file drivers infer
// operations from the permissions and args provided.
//
// Example usage:
//
//	// File read - uses all available permissions
//	result, err := ctx.IO("file:///tmp/config.txt").Call(map[string]interface{}{
//	    "operation": "read",
//	})
//
//	// File write - explicitly limited to write permission
//	err := ctx.IO("file:///tmp/config.txt", "write").Call(map[string]interface{}{
//	    "operation": "write",
//	    "data": []byte("content"),
//	})
//
//	// HTTP request - method specified in args
//	result, err := ctx.IO("https://api.example.com/data").Call(map[string]interface{}{
//	    "method": "POST",
//	    "body": []byte("data"),
//	    "headers": map[string]string{"Content-Type": "application/json"},
//	})
//
//	// App call
//	result, err := ctx.IO("fn://wazeos/logger", "invoke").Call(map[string]interface{}{
//	    "level": "info",
//	    "message": "test",
//	})
func (op *IOOperation) Call(args map[string]interface{}) (map[string]interface{}, error) {
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

	// Make the driver call with permission strings
	result, err := driver.CallResourceCall(&driver.ResourceCall{
		URI:         op.uri,
		Headers:     headers,
		Body:        body,
		Permissions: op.permissions,
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
