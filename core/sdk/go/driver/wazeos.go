// Package wazeos provides a Go SDK for building WazeOS I/O drivers.
//
// This SDK allows you to create WASM-based I/O drivers that extend the
// WazeOS kernel's capabilities by handling specific URI patterns.
//
// Example usage:
//
//	package main
//
//	import (
//	    wazeos "github.com/wazeos/wazeos/core/sdk/go/driver"
//	)
//
//	//export driver_metadata
//	func driverMetadata() uint32 {
//	    return wazeos.ReturnMetadata(wazeos.Metadata{
//	        Name:         "echo-driver-go",
//	        Version:      "1.0.0",
//	        Class:        "io.connect",
//	        URIPattern:   "echo://**",
//	        Capabilities: []string{"call"},
//	    })
//	}
//
//	//export driver_init
//	func driverInit(configPtr, configLen uint32) uint32 {
//	    // Initialize driver with config
//	    return 0 // Success
//	}
//
//	//export driver_call
//	func driverCall(requestPtr, requestLen uint32) uint32 {
//	    req, _ := wazeos.ParseRequest(requestPtr, requestLen)
//	    // Handle request
//	    return wazeos.ReturnResponse(200, nil, []byte("Echo: "+req.URI))
//	}
//
//	func main() {}
package wazeos

import (
	"encoding/base64"
	"encoding/json"
	"unsafe"
)

// ============================================================================
// Host Function Imports
// ============================================================================

//go:wasmimport env host_iobus_call
func hostIOBusCall(ptr, length uint32) uint64

//go:wasmimport env host_iobus_create_handle
func hostIOBusCreateHandle(ptr, length uint32) uint64

//go:wasmimport env host_iobus_close_handle
func hostIOBusCloseHandle(ptr, length uint32) uint32

// ============================================================================
// Driver Types
// ============================================================================

// Metadata describes the driver's capabilities and configuration
type Metadata struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Class        string   `json:"class"`           // "io.connect", "runtime", etc.
	URIPattern   string   `json:"uri_pattern"`     // "file://**", "http://**", etc.
	Capabilities []string `json:"capabilities"`    // ["call", "stream"], etc.
}

// Request represents an incoming driver request
type Request struct {
	URI       string                 `json:"uri"`
	Operation string                 `json:"operation"`
	Args      map[string]interface{} `json:"args,omitempty"`
	Headers   map[string]string      `json:"headers,omitempty"`
	Body      []byte                 `json:"-"`
	BodyB64   string                 `json:"body,omitempty"`
}

// Response represents a driver response
type Response struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       []byte            `json:"-"`
	BodyB64    string            `json:"body,omitempty"`
	Error      *string           `json:"error,omitempty"`
}

// ============================================================================
// Driver Context
// ============================================================================

// Context provides access to host functions for drivers
type Context struct{}

// NewContext creates a new driver context
func NewContext() *Context {
	return &Context{}
}

// Call makes an IO Bus call to another driver
//
// This allows drivers to compose and call other drivers.
//
// Example:
//
//	ctx := wazeos.NewContext()
//	resp, err := ctx.Call("file:///tmp/config.json", nil, nil)
func (c *Context) Call(uri string, headers map[string]string, body []byte) (*Response, error) {
	// Build request
	req := Request{
		URI:       uri,
		Operation: "call",
		Headers:   headers,
		Body:      body,
	}

	// Encode body as base64 if present
	if len(body) > 0 {
		req.BodyB64 = base64.StdEncoding.EncodeToString(body)
	}

	// Marshal request to JSON
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	// Call host function
	reqPtr := uint32(uintptr(unsafe.Pointer(&reqJSON[0])))
	reqLen := uint32(len(reqJSON))
	result := hostIOBusCall(reqPtr, reqLen)

	if result == 0 {
		return nil, &DriverError{Message: "IO Bus call failed"}
	}

	// Extract response pointer and length from packed u64
	respPtr := uint32(result >> 32)
	respLen := uint32(result & 0xFFFFFFFF)

	// Read response from memory
	respJSON := ReadBytes(respPtr, respLen)

	// Unmarshal response
	var resp Response
	if err := json.Unmarshal(respJSON, &resp); err != nil {
		return nil, err
	}

	// Decode base64 body if present
	if resp.BodyB64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(resp.BodyB64)
		if err != nil {
			return nil, err
		}
		resp.Body = decoded
	}

	return &resp, nil
}

// CreateHandle creates a persistent handle to another driver
//
// Handles allow efficient repeated calls to the same resource.
//
// Returns a handle URI that can be used for subsequent calls.
func (c *Context) CreateHandle(uri string) (string, error) {
	uriBytes := []byte(uri)
	ptr := uint32(uintptr(unsafe.Pointer(&uriBytes[0])))
	length := uint32(len(uriBytes))

	result := hostIOBusCreateHandle(ptr, length)
	if result == 0 {
		return "", &DriverError{Message: "Failed to create handle"}
	}

	// Extract handle ID pointer and length
	handlePtr := uint32(result >> 32)
	handleLen := uint32(result & 0xFFFFFFFF)

	// Read handle ID from memory
	handleID := ReadString(handlePtr, handleLen)
	return handleID, nil
}

// CloseHandle closes a persistent handle
func (c *Context) CloseHandle(handleURI string) error {
	uriBytes := []byte(handleURI)
	ptr := uint32(uintptr(unsafe.Pointer(&uriBytes[0])))
	length := uint32(len(uriBytes))

	result := hostIOBusCloseHandle(ptr, length)
	if result != 0 {
		return &DriverError{Message: "Failed to close handle"}
	}
	return nil
}

// ============================================================================
// Error Types
// ============================================================================

// DriverError represents a driver error
type DriverError struct {
	Message    string
	StatusCode int
}

func (e *DriverError) Error() string {
	return e.Message
}

// ============================================================================
// Memory Helpers
// ============================================================================

// ReadString reads a string from WASM memory
func ReadString(ptr, length uint32) string {
	return string(ReadBytes(ptr, length))
}

// ReadBytes reads bytes from WASM memory
func ReadBytes(ptr, length uint32) []byte {
	if length == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), length)
}

// WriteString writes a string to WASM memory and returns its pointer
func WriteString(s string) uint32 {
	if s == "" {
		return 0
	}
	bytes := []byte(s)
	return uint32(uintptr(unsafe.Pointer(&bytes[0])))
}

// ============================================================================
// Driver Helpers
// ============================================================================

// ParseRequest parses an incoming driver request
func ParseRequest(requestPtr, requestLen uint32) (*Request, error) {
	requestJSON := ReadString(requestPtr, requestLen)

	var req Request
	if err := json.Unmarshal([]byte(requestJSON), &req); err != nil {
		return nil, err
	}

	// Decode base64 body if present
	if req.BodyB64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(req.BodyB64)
		if err != nil {
			return nil, err
		}
		req.Body = decoded
	}

	return &req, nil
}

// ReturnMetadata marshals metadata to JSON and returns a pointer
func ReturnMetadata(meta Metadata) uint32 {
	data, err := json.Marshal(meta)
	if err != nil {
		// Return minimal valid metadata on error
		fallback := `{"name":"unknown","version":"0.0.0","class":"io.connect","uri_pattern":"unknown://**","capabilities":[]}`
		return WriteString(fallback)
	}
	return WriteString(string(data))
}

// ReturnResponse creates a response and returns a pointer to its JSON
func ReturnResponse(statusCode int, headers map[string]string, body []byte) uint32 {
	resp := Response{
		StatusCode: statusCode,
		Headers:    headers,
		Body:       body,
	}

	// Encode body as base64 if present
	if len(body) > 0 {
		resp.BodyB64 = base64.StdEncoding.EncodeToString(body)
	}

	data, err := json.Marshal(resp)
	if err != nil {
		// Return error response
		errorResp := Response{
			StatusCode: 500,
			Error:      stringPtr("Failed to marshal response"),
		}
		errorData, _ := json.Marshal(errorResp)
		return WriteString(string(errorData))
	}

	return WriteString(string(data))
}

// ReturnError creates an error response and returns a pointer to its JSON
func ReturnError(statusCode int, message string) uint32 {
	resp := Response{
		StatusCode: statusCode,
		Error:      &message,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		fallback := `{"status_code":500,"error":"Internal error"}`
		return WriteString(fallback)
	}

	return WriteString(string(data))
}

// ParseConfig parses JSON configuration from driver_init
func ParseConfig(configPtr, configLen uint32) (map[string]interface{}, error) {
	configJSON := ReadString(configPtr, configLen)
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return nil, err
	}
	return config, nil
}

// ============================================================================
// Utility Functions
// ============================================================================

func stringPtr(s string) *string {
	return &s
}
