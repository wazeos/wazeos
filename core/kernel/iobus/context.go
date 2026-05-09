package iobus

import (
	"context"
	"strings"
	"time"
)

// ============================================================================
// Context Implementation
// ============================================================================

// contextImpl implements the Context interface
type contextImpl struct {
	context.Context
	principal   string
	requestID   string
	traceID     string
	permissions []PermissionEntry
	iobus       *IOBus
	values      map[any]any
}

// PermissionEntry represents a URI pattern with allowed permissions
type PermissionEntry struct {
	URIPattern  string
	Permissions []string
}

// NewContext creates a new context
func NewContext(
	parent context.Context,
	principal, requestID, traceID string,
	permissions []PermissionEntry,
	iobus *IOBus,
) Context {
	return &contextImpl{
		Context:     parent,
		principal:   principal,
		requestID:   requestID,
		traceID:     traceID,
		permissions: permissions,
		iobus:       iobus,
		values:      make(map[any]any),
	}
}

func (c *contextImpl) Principal() string {
	return c.principal
}

func (c *contextImpl) RequestID() string {
	return c.requestID
}

func (c *contextImpl) TraceID() string {
	return c.traceID
}

func (c *contextImpl) HasPermission(uri string, perms ...string) bool {
	// Check each permission entry
	for _, entry := range c.permissions {
		// Check if URI matches pattern
		if matchesPattern(entry.URIPattern, uri) {
			// Check if all required permissions are granted
			hasAll := true
			for _, reqPerm := range perms {
				found := false
				for _, grantedPerm := range entry.Permissions {
					if grantedPerm == reqPerm || grantedPerm == "*" {
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

func (c *contextImpl) IOBus() *IOBus {
	return c.iobus
}

func (c *contextImpl) WithValue(key, value any) Context {
	newCtx := &contextImpl{
		Context:     c.Context,
		principal:   c.principal,
		requestID:   c.requestID,
		traceID:     c.traceID,
		permissions: c.permissions,
		iobus:       c.iobus,
		values:      make(map[any]any),
	}

	// Copy existing values
	for k, v := range c.values {
		newCtx.values[k] = v
	}

	// Add new value
	newCtx.values[key] = value

	return newCtx
}

func (c *contextImpl) Value(key any) any {
	// Check custom values first
	if val, ok := c.values[key]; ok {
		return val
	}

	// Fall back to parent context
	return c.Context.Value(key)
}

func (c *contextImpl) Deadline() (deadline time.Time, ok bool) {
	return c.Context.Deadline()
}

func (c *contextImpl) Done() <-chan struct{} {
	return c.Context.Done()
}

func (c *contextImpl) Err() error {
	return c.Context.Err()
}

// ============================================================================
// Pattern Matching
// ============================================================================

// matchesPattern checks if a URI matches a pattern
//
// Patterns:
//   - "file://**"           matches any file:// URI
//   - "file:///tmp/**"      matches files under /tmp
//   - "s3://my-bucket/**"   matches any key in my-bucket
//   - "http://api.example.com/**" matches any path on that host
func matchesPattern(pattern, uri string) bool {
	// Exact match
	if pattern == uri {
		return true
	}

	// Wildcard match
	if strings.HasSuffix(pattern, "**") {
		prefix := strings.TrimSuffix(pattern, "**")
		return strings.HasPrefix(uri, prefix)
	}

	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		// Match single segment after prefix
		if !strings.HasPrefix(uri, prefix) {
			return false
		}
		remainder := strings.TrimPrefix(uri, prefix)
		// Remainder should not contain slashes (single segment)
		return !strings.Contains(remainder, "/")
	}

	return false
}
