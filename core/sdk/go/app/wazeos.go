// Package wazeos provides a Go SDK for building WazeOS MCP tools.
//
// This SDK allows you to create WASM-based MCP tools that run in the WazeOS
// sandbox and can access I/O drivers through the IO Bus.
//
// Example usage:
//
//	package main
//
//	import (
//	    "encoding/json"
//	    wazeos "github.com/wazeos/wazeos/core/sdk/go/app"
//	)
//
//	//export wazeos_tool_invoke
//	func toolInvoke(argsPtr, argsLen uint32) uint32 {
//	    ctx := wazeos.NewContext()
//
//	    // Parse arguments
//	    args := wazeos.ReadString(argsPtr, argsLen)
//	    var input map[string]interface{}
//	    json.Unmarshal([]byte(args), &input)
//
//	    // Call a driver
//	    resp, err := ctx.Call("file:///tmp/data.txt", nil, nil)
//	    if err != nil {
//	        return wazeos.ReturnError(err.Error())
//	    }
//
//	    // Return result
//	    result := map[string]interface{}{
//	        "content": string(resp.Body),
//	    }
//	    return wazeos.ReturnSuccess(result)
//	}
//
//	//export wazeos_tool_metadata
//	func toolMetadata() uint32 {
//	    return wazeos.ReturnMetadata("my-tool", "1.0.0")
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

//go:wasm-module env
//export host_iobus_call
func hostIOBusCall(ptr, length uint32) uint64

// ============================================================================
// Core Types
// ============================================================================

// Context provides access to the WazeOS IO Bus
type Context struct{}

// NewContext creates a new application context
func NewContext() *Context {
	return &Context{}
}

// Request represents an IO Bus request
type Request struct {
	URI       string                 `json:"uri"`
	Operation string                 `json:"operation"`
	Args      map[string]interface{} `json:"args,omitempty"`
	Headers   map[string]string      `json:"headers,omitempty"`
	Body      []byte                 `json:"-"`
	BodyB64   string                 `json:"body,omitempty"`
}

// Response represents an IO Bus response
type Response struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       []byte            `json:"-"`
	BodyB64    string            `json:"body,omitempty"`
	Error      *string           `json:"error,omitempty"`
}

// ============================================================================
// Context Methods
// ============================================================================

// Call makes a generic IO Bus call to any driver
//
// This is the low-level method that works with any driver. Driver-specific
// helper packages can provide more ergonomic wrappers.
//
// Parameters:
//   - uri: The resource URI (e.g., "file:///tmp/data.txt", "shell://exec")
//   - headers: Optional headers for the request (driver-specific)
//   - body: Optional request body
//
// Returns the response or an error if the call failed.
//
// Example:
//
//	resp, err := ctx.Call("file:///tmp/data.txt", nil, nil)
//	if err != nil {
//	    return err
//	}
//	content := string(resp.Body)
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
		return nil, &IOBusError{Message: "IO Bus call failed"}
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

	// Check for error in response
	if resp.Error != nil {
		return &resp, &IOBusError{
			Message:    *resp.Error,
			StatusCode: resp.StatusCode,
		}
	}

	return &resp, nil
}

// ============================================================================
// Error Types
// ============================================================================

// IOBusError represents an error from the IO Bus
type IOBusError struct {
	Message    string
	StatusCode int
}

func (e *IOBusError) Error() string {
	if e.StatusCode != 0 {
		return e.Message
	}
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
// The caller is responsible for managing the lifetime of the returned pointer
func WriteString(s string) uint32 {
	if s == "" {
		return 0
	}
	bytes := []byte(s)
	return uint32(uintptr(unsafe.Pointer(&bytes[0])))
}

// ============================================================================
// Response Helpers
// ============================================================================

// ToolResponse represents a tool invocation response
type ToolResponse struct {
	Success bool        `json:"success"`
	Result  interface{} `json:"result,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ReturnSuccess returns a success response to the WazeOS runtime
func ReturnSuccess(result interface{}) uint32 {
	resp := ToolResponse{
		Success: true,
		Result:  result,
	}
	return returnJSON(resp)
}

// ReturnError returns an error response to the WazeOS runtime
func ReturnError(message string) uint32 {
	resp := ToolResponse{
		Success: false,
		Error:   message,
	}
	return returnJSON(resp)
}

// ReturnMetadata returns tool metadata to the WazeOS runtime
func ReturnMetadata(name, version string) uint32 {
	metadata := map[string]string{
		"name":    name,
		"version": version,
	}
	return returnJSON(metadata)
}

// returnJSON marshals a value to JSON and returns a pointer to it
func returnJSON(v interface{}) uint32 {
	data, err := json.Marshal(v)
	if err != nil {
		// Fallback error response
		errorJSON := `{"success":false,"error":"Failed to marshal response"}`
		return WriteString(errorJSON)
	}
	return WriteString(string(data))
}

// ============================================================================
// Convenience Functions
// ============================================================================

// ParseArgs parses JSON arguments from the tool invocation
func ParseArgs(argsPtr, argsLen uint32) (map[string]interface{}, error) {
	argsJSON := ReadString(argsPtr, argsLen)
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nil, err
	}
	return args, nil
}

// MustParseArgs parses arguments and panics on error
// Useful for tools where argument parsing failure is unexpected
func MustParseArgs(argsPtr, argsLen uint32) map[string]interface{} {
	args, err := ParseArgs(argsPtr, argsLen)
	if err != nil {
		panic("Failed to parse arguments: " + err.Error())
	}
	return args
}
