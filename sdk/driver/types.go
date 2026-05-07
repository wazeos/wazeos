// Package driver provides shared types and utilities for WazeOS WASM drivers.
//
// This SDK is used by WASM drivers compiled with TinyGo to interact with the
// WazeOS kernel. It provides:
// - Common type definitions (ResourceCall, AuthPayload, etc.)
// - Memory management helpers (allocate, deallocate)
// - Host function call helpers
// - JSON serialization utilities
package driver

// AccessBits represents read, write, and execute permissions as a bitfield.
type AccessBits uint8

const (
	AccessRead    AccessBits = 1 << 0 // 0x01
	AccessWrite   AccessBits = 1 << 1 // 0x02
	AccessExecute AccessBits = 1 << 2 // 0x04
)

// String returns a string representation of access bits (e.g., "rw", "rx", "rwx").
func (a AccessBits) String() string {
	var result string
	if a&AccessRead != 0 {
		result += "r"
	}
	if a&AccessWrite != 0 {
		result += "w"
	}
	if a&AccessExecute != 0 {
		result += "x"
	}
	if result == "" {
		return "-"
	}
	return result
}

// Has checks if the access bits include the specified permission.
func (a AccessBits) Has(permission AccessBits) bool {
	return a&permission == permission
}

// ResourceCall represents an IO call from a wasm app to a resource driver.
type ResourceCall struct {
	URI        string            `json:"uri"`
	Method     string            `json:"method"`
	Headers    map[string]string `json:"headers"`
	Body       []byte            `json:"body"`
	AccessMode AccessBits        `json:"accessMode"`
}

// ResourceResult represents the result of a resource call.
type ResourceResult struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers"`
	Body       []byte            `json:"body"`
	Error      string            `json:"error,omitempty"`
}

// NewResourceResult creates a successful resource result.
func NewResourceResult(statusCode int, body []byte) *ResourceResult {
	return &ResourceResult{
		StatusCode: statusCode,
		Headers:    make(map[string]string),
		Body:       body,
	}
}

// NewErrorResult creates an error resource result.
func NewErrorResult(statusCode int, message string) *ResourceResult {
	return &ResourceResult{
		StatusCode: statusCode,
		Headers:    make(map[string]string),
		Body:       []byte(message),
		Error:      message,
	}
}

// AuthPayload represents authentication input from a request.
type AuthPayload struct {
	Headers map[string]string `json:"headers"`
	Body    []byte            `json:"body"`
}

// AuthResult represents the authentication result.
type AuthResult struct {
	Principal string `json:"principal,omitempty"` // e.g. "user:alice"
	Error     string `json:"error,omitempty"`     // Error message or "abstain"
}

// NewAuthResult creates a successful authentication result.
func NewAuthResult(principal string) *AuthResult {
	return &AuthResult{
		Principal: principal,
	}
}

// NewAuthError creates an error authentication result.
func NewAuthError(message string) *AuthResult {
	return &AuthResult{
		Error: message,
	}
}

// NewAuthAbstain creates an abstain authentication result.
func NewAuthAbstain() *AuthResult {
	return &AuthResult{
		Error: "abstain",
	}
}

// IsAbstain checks if the auth result is an abstain.
func (r *AuthResult) IsAbstain() bool {
	return r.Error == "abstain"
}

// PermissionContext represents the set of permissions for a principal.
type PermissionContext struct {
	Entries []PermissionEntry `json:"entries"`
}

// PermissionEntry represents a single URI-based access control entry.
type PermissionEntry struct {
	URIPattern string     `json:"uriPattern"` // URI pattern with wildcard support
	Access     AccessBits `json:"access"`     // Allowed access bits
}

// NewPermissionContext creates a new permission context.
func NewPermissionContext(entries []PermissionEntry) *PermissionContext {
	return &PermissionContext{Entries: entries}
}
