package app

import (
	"encoding/json"
	"fmt"

	"github.com/wazeos/wazeos/sdk/driver"
)

// TestContext creates a context for testing.
func TestContext() *Context {
	ctx := &Context{
		RequestID:   "test-request-1",
		TraceID:     "test-trace-1",
		Principal:   "user:test",
		Permissions: driver.NewPermissionContext(nil),
		Metadata:    make(map[string]string),
	}

	ctx.Log = &Logger{ctx: ctx}

	return ctx
}

// TestContextWithPermissions creates a test context with specific permissions.
func TestContextWithPermissions(entries []driver.PermissionEntry) *Context {
	ctx := TestContext()
	ctx.Permissions = driver.NewPermissionContext(entries)
	return ctx
}

// Allow creates a permission entry with explicit permissions.
// Use this to grant specific permissions for a URI pattern in tests.
// Example: Allow("file:///tmp/*", "read", "write")
func Allow(uriPattern string, permissions ...string) driver.PermissionEntry {
	return driver.PermissionEntry{
		URIPattern:  uriPattern,
		Permissions: permissions,
	}
}

// MarshalJSON helper for test data.
func MustMarshalJSON(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal JSON: %v", err))
	}
	return data
}
