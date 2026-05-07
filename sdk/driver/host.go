//go:build tinygo.wasm

package driver

import (
	"encoding/json"
	"fmt"
	"unsafe"
)

// Host function imports from the WazeOS kernel.
// These functions are provided by RuntimeExec and registered during kernel startup.

//go:wasmimport kernel resource_call
func hostResourceCall(paramsPtr, paramsLen, resultBufPtr, resultBufCap uint32) uint32

//go:wasmimport kernel authz_check
func hostAuthzCheck(paramsPtr, paramsLen uint32) uint64

//go:wasmimport kernel pkg_resolve
func hostPkgResolve(paramsPtr, paramsLen uint32) uint64

// CallResourceCall makes a resource call to another driver via the kernel.
//
// Example:
//
//	call := &driver.ResourceCall{
//	    URI:    "file:///tmp/data.txt",
//	    Method: "READ",
//	}
//	result, err := driver.CallResourceCall(call)
func CallResourceCall(call *ResourceCall) (*ResourceResult, error) {
	// Marshal call to JSON
	callJSON, err := json.Marshal(call)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resource call: %w", err)
	}

	// Allocate input buffer
	paramsPtr := CopyToMemory(callJSON)

	// Allocate output buffer (1MB max)
	const maxResultSize = 1024 * 1024
	resultBuf := make([]byte, maxResultSize)
	resultBufPtr := uint32(uintptr(unsafe.Pointer(&resultBuf[0])))

	// Call host function with input and output buffers
	resultLen := hostResourceCall(paramsPtr, uint32(len(callJSON)), resultBufPtr, maxResultSize)

	// Clean up input buffer
	Deallocate()

	// Check for error (length 0 means error or empty result)
	if resultLen == 0 {
		return nil, fmt.Errorf("host function returned no data")
	}

	// Read result from output buffer
	resultData := resultBuf[:resultLen]

	// Unmarshal result
	var result ResourceResult
	if err := json.Unmarshal(resultData, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal resource result: %w", err)
	}

	return &result, nil
}

// CheckAuthorization verifies if a URI access is permitted.
//
// Example:
//
//	allowed, err := driver.CheckAuthorization(
//	    "file:///tmp/test.txt",
//	    "rw",
//	    permissionContext,
//	)
func CheckAuthorization(uri, mode string, permissions *PermissionContext) (bool, error) {
	input := struct {
		URI         string             `json:"uri"`
		Mode        string             `json:"mode"`
		Permissions *PermissionContext `json:"permissions"`
	}{
		URI:         uri,
		Mode:        mode,
		Permissions: permissions,
	}

	// Marshal input to JSON
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return false, fmt.Errorf("failed to marshal authz check: %w", err)
	}

	// Copy to memory and call host
	paramsPtr := CopyToMemory(inputJSON)
	resultPacked := hostAuthzCheck(paramsPtr, uint32(len(inputJSON)))

	// Extract result pointer and length
	resultPtr := uint32(resultPacked >> 32)
	resultLen := uint32(resultPacked & 0xFFFFFFFF)

	// Read result from memory
	resultData := CopyFromMemory(resultPtr, resultLen)
	Deallocate()

	// Unmarshal result
	var output struct {
		Allowed bool   `json:"allowed"`
		Error   string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(resultData, &output); err != nil {
		return false, fmt.Errorf("failed to parse auth response: %w", err)
	}

	if output.Error != "" {
		return false, fmt.Errorf("%s", output.Error)
	}

	return output.Allowed, nil
}

// ResolvePackage resolves an app name to its full ID.
//
// Example:
//
//	appID, err := driver.ResolvePackage("myapp")
//	// Returns: "author/myapp_1.0.0"
func ResolvePackage(name string) (string, error) {
	input := struct {
		Name string `json:"name"`
	}{
		Name: name,
	}

	// Marshal input to JSON
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("failed to marshal pkg resolve: %w", err)
	}

	// Copy to memory and call host
	paramsPtr := CopyToMemory(inputJSON)
	resultPacked := hostPkgResolve(paramsPtr, uint32(len(inputJSON)))

	// Extract result pointer and length
	resultPtr := uint32(resultPacked >> 32)
	resultLen := uint32(resultPacked & 0xFFFFFFFF)

	// Read result from memory
	resultData := CopyFromMemory(resultPtr, resultLen)
	Deallocate()

	// Unmarshal result
	var output struct {
		AppID string `json:"appID,omitempty"`
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(resultData, &output); err != nil {
		return "", fmt.Errorf("failed to parse pkg response: %w", err)
	}

	if output.Error != "" {
		return "", fmt.Errorf("%s", output.Error)
	}

	return output.AppID, nil
}
