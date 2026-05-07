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
	Log *Logger // Structured logger
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

	// Parse mode string into permission names
	var requiredPerms []string
	for _, char := range mode {
		switch char {
		case 'r':
			requiredPerms = append(requiredPerms, "read")
		case 'w':
			requiredPerms = append(requiredPerms, "write")
		case 'x':
			requiredPerms = append(requiredPerms, "execute")
		}
	}

	// Check if any permission entry matches the pattern and has all required permissions
	for _, entry := range c.Permissions.Entries {
		if matchesPattern(entry.URIPattern, uriPattern) {
			hasAll := true
			for _, req := range requiredPerms {
				found := false
				for _, perm := range entry.Permissions {
					if perm == req {
						found = true
						break
					}
				}
				if !found {
					hasAll = false
					break
				}
			}
			if hasAll {
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

	// Initialize Logger
	ctx.Log = &Logger{ctx: ctx}

	return ctx
}
