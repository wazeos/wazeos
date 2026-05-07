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
	IO  IOClient // High-level I/O client
	Log *Logger  // Structured logger
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
	ctx.IO = &realIOClient{ctx: ctx}
	ctx.Log = &Logger{ctx: ctx}

	return ctx
}
