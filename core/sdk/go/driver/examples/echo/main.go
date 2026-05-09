// Echo Driver - WazeOS Driver Example
//
// This is a minimal example showing how to create a WazeOS driver in Go.
// It echoes back whatever is sent to it.
//
// Build:
//   tinygo build -o echo.wasm -target=wasi -no-debug -opt=2 main.go
//
// Install:
//   wazeos driver install echo.wasm
//
// Usage:
//   Any URI matching echo://** will be handled by this driver
package main

import (
	"encoding/json"
	"fmt"
	wazeos "github.com/wazeos/wazeos/core/sdk/go/driver"
)

// driverMetadata returns information about this driver
//
//export driver_metadata
func driverMetadata() uint32 {
	return wazeos.ReturnMetadata(wazeos.Metadata{
		Name:         "echo-driver-go",
		Version:      "1.0.0",
		Class:        "io.connect",
		URIPattern:   "echo://**",
		Capabilities: []string{"call"},
	})
}

// driverInit initializes the driver
//
//export driver_init
func driverInit(configPtr, configLen uint32) uint32 {
	// For echo driver, we don't need any initialization
	// In a real driver, you'd parse config and set up resources

	config, err := wazeos.ParseConfig(configPtr, configLen)
	if err != nil {
		// Config parsing failed, but we don't need config for echo
		// so we'll just continue
	}

	// Could log or store config if needed
	_ = config

	return 0 // Success
}

// driverCall handles incoming requests
//
//export driver_call
func driverCall(requestPtr, requestLen uint32) uint32 {
	// Parse the request
	req, err := wazeos.ParseRequest(requestPtr, requestLen)
	if err != nil {
		return wazeos.ReturnError(400, fmt.Sprintf("Invalid request: %s", err))
	}

	// Build echo response
	echoData := map[string]interface{}{
		"uri":       req.URI,
		"operation": req.Operation,
		"headers":   req.Headers,
		"body_size": len(req.Body),
	}

	// If body is small enough, include it in the echo
	if len(req.Body) > 0 && len(req.Body) < 1024 {
		echoData["body"] = string(req.Body)
	}

	// Marshal to JSON
	responseBody, err := json.Marshal(echoData)
	if err != nil {
		return wazeos.ReturnError(500, "Failed to marshal echo response")
	}

	// Return success response
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	return wazeos.ReturnResponse(200, headers, responseBody)
}

// main is required but not used in WASM
func main() {}
