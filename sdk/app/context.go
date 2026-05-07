package app

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/wazeos/wazeos/sdk/driver"
)

// Context provides access to execution metadata and I/O operations for apps.
type Context struct {
	// Execution metadata (read-only)
	RequestID   string                    // Unique request identifier
	TraceID     string                    // Distributed tracing ID
	Principal   string                    // Authenticated user/service (e.g., "user:alice")
	Permissions *driver.PermissionContext // URI-based access control permissions
	Metadata    map[string]string         // Additional metadata

	// I/O operations
	io  IOClient // Internal I/O client for deprecated methods
	Log *Logger  // Structured logger
}

// IO creates a new I/O operation for the given URI with required permissions.
// This is the unified API for all I/O operations (files, HTTP, apps, queues, etc.).
//
// Example usage:
//
//	// File operations
//	data, err := ctx.IO("file:///tmp/config.txt", []string{"read"}).Call(nil)
//	err := ctx.IO("file:///tmp/config.txt", []string{"write"}).Call(map[string]interface{}{
//	    "data": []byte("content"),
//	})
//
//	// HTTP requests
//	result, err := ctx.IO("https://api.example.com/data", []string{"GET"}).Call(nil)
//	result, err := ctx.IO("https://api.example.com/data", []string{"POST"}).Call(map[string]interface{}{
//	    "body": []byte("data"),
//	    "headers": map[string]string{"Content-Type": "application/json"},
//	})
//
//	// App-to-app calls
//	result, err := ctx.IO("fn://wazeos/logger", []string{"invoke"}).Call(map[string]interface{}{
//	    "level": "info",
//	    "message": "test",
//	})
//
//	// Queue operations
//	err := ctx.IO("queue://events", []string{"write"}).Call(map[string]interface{}{
//	    "message": []byte("event data"),
//	})
func (c *Context) IO(uri string, permissions []string) *IOOperation {
	return &IOOperation{
		ctx:         c,
		uri:         uri,
		permissions: permissions,
	}
}

// Deprecated bridge methods for backward compatibility
// These delegate to the internal io client

// ReadFile reads the contents of a file.
// Deprecated: Use ctx.IO("file://path", []string{"read"}).Call(nil) instead.
func (c *Context) ReadFile(path string) ([]byte, error) {
	return c.io.ReadFile(path)
}

// WriteFile writes data to a file.
// Deprecated: Use ctx.IO("file://path", []string{"write"}).Call(map[string]interface{}{"data": data}) instead.
func (c *Context) WriteFile(path string, data []byte) error {
	return c.io.WriteFile(path, data)
}

// DeleteFile deletes a file.
// Deprecated: Use ctx.IO("file://path", []string{"delete"}).Call(nil) instead.
func (c *Context) DeleteFile(path string) error {
	return c.io.DeleteFile(path)
}

// ListFiles lists files in a directory.
// Deprecated: Use ctx.IO("file://path", []string{"list"}).Call(nil) instead.
func (c *Context) ListFiles(dir string) ([]string, error) {
	return c.io.ListFiles(dir)
}

// Get makes an HTTP GET request.
// Deprecated: Use ctx.IO("https://url", []string{"GET"}).Call(nil) instead.
func (c *Context) Get(url string) (*HTTPResponse, error) {
	return c.io.Get(url)
}

// Post makes an HTTP POST request.
// Deprecated: Use ctx.IO("https://url", []string{"POST"}).Call(map[string]interface{}{"body": body, "headers": headers}) instead.
func (c *Context) Post(url string, body []byte, headers map[string]string) (*HTTPResponse, error) {
	return c.io.Post(url, body, headers)
}

// Request makes an HTTP request.
// Deprecated: Use ctx.IO("https://url", []string{method}).Call(map[string]interface{}{"body": body, "headers": headers}) instead.
func (c *Context) Request(method, url string, body []byte, headers map[string]string) (*HTTPResponse, error) {
	return c.io.Request(method, url, body, headers)
}

// CallApp makes an fn:// call to another app.
// Deprecated: Use ctx.IO("fn://appName", []string{"invoke"}).Call(args) instead.
func (c *Context) CallApp(appName string, args ...string) (*Response, error) {
	return c.io.CallApp(appName, args...)
}

// CallAppWithInput makes an fn:// call to another app with input data.
// Deprecated: Use ctx.IO("fn://appName", []string{"invoke"}).Call(args) instead.
func (c *Context) CallAppWithInput(appName string, input []byte, args ...string) (*Response, error) {
	return c.io.CallAppWithInput(appName, input, args...)
}

// Publish sends a message to a queue.
// Deprecated: Use ctx.IO("queue://topic", []string{"write"}).Call(map[string]interface{}{"message": message}) instead.
func (c *Context) Publish(topic string, message []byte) error {
	return c.io.Publish(topic, message)
}

// PublishWithKey sends a message to a queue with a partition key.
// Deprecated: Use ctx.IO("queue://topic", []string{"write"}).Call(map[string]interface{}{"message": message, "key": key}) instead.
func (c *Context) PublishWithKey(topic, key string, message []byte) error {
	return c.io.PublishWithKey(topic, key, message)
}

// Consume reads messages from a queue.
// Deprecated: Use ctx.IO("queue://topic", []string{"read"}).Call(opts) instead.
func (c *Context) Consume(topic string, opts *ConsumeOptions) ([]*Message, error) {
	return c.io.Consume(topic, opts)
}

// Call makes a generic resource call.
// Deprecated: Use ctx.IO(uri, []string{method}).Call(map[string]interface{}{"body": body, "headers": headers}) instead.
func (c *Context) Call(uri, method string, body []byte, headers map[string]string) (*driver.ResourceResult, error) {
	return c.io.Call(uri, method, body, headers)
}

// HasPermission checks if a URI pattern is accessible with the given mode.
//
// Example:
//
//	if ctx.HasPermission("file:///tmp/*", "rw") {
//	    // Can read and write files in /tmp
//	}
func (c *Context) HasPermission(uriPattern string, mode string) bool {
	if c.Permissions == nil {
		return false
	}

	// Parse mode string into AccessBits
	var required driver.AccessBits
	for _, char := range mode {
		switch char {
		case 'r':
			required |= driver.AccessRead
		case 'w':
			required |= driver.AccessWrite
		case 'x':
			required |= driver.AccessExecute
		}
	}

	// Check if any permission entry matches the pattern
	for _, entry := range c.Permissions.Entries {
		if matchesPattern(entry.URIPattern, uriPattern) {
			if entry.Access.Has(required) {
				return true
			}
		}
	}

	return false
}

// matchesPattern checks if a URI matches a pattern (simple glob matching).
func matchesPattern(pattern, uri string) bool {
	// Simple wildcard matching (*).
	// For now, just check if pattern ends with * and matches prefix.
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(uri, prefix)
	}
	return pattern == uri
}

// buildContext creates a Context from environment variables set by RuntimeExec.
func buildContext() *Context {
	// Extract execution context from environment variables
	requestID := os.Getenv("WAZEOS_REQUEST_ID")
	traceID := os.Getenv("WAZEOS_TRACE_ID")
	principal := os.Getenv("WAZEOS_PRINCIPAL")
	permissionsJSON := os.Getenv("WAZEOS_PERMISSIONS")

	// Parse permissions if present
	var permissions *driver.PermissionContext
	if permissionsJSON != "" {
		var perms driver.PermissionContext
		if err := json.Unmarshal([]byte(permissionsJSON), &perms); err == nil {
			permissions = &perms
		}
	}

	// Parse metadata from WAZEOS_METADATA_* environment variables
	metadata := make(map[string]string)
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "WAZEOS_METADATA_") {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimPrefix(parts[0], "WAZEOS_METADATA_")
				metadata[key] = parts[1]
			}
		}
	}

	ctx := &Context{
		RequestID:   requestID,
		TraceID:     traceID,
		Principal:   principal,
		Permissions: permissions,
		Metadata:    metadata,
	}

	// Initialize I/O and Logger
	ctx.io = &realIOClient{ctx: ctx}
	ctx.Log = &Logger{ctx: ctx}

	return ctx
}
