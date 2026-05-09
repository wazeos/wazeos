# WazeOS Driver SDK for Go

Build I/O drivers for WazeOS using Go and TinyGo.

**Status**: Production-ready
**Language**: Go (compiled to WASM via TinyGo)

## Overview

This SDK lets you build I/O drivers in Go that:
- Extend the WazeOS kernel with new capabilities
- Handle specific URI patterns (e.g., `redis://`, `postgres://`)
- Run as sandboxed WASM modules
- Compose with other drivers

## Prerequisites

Install TinyGo (see [App SDK README](../app/README.md) for instructions).

## Quick Start

### 1. Create a driver

```go
package main

import (
	wazeos "github.com/wazeos/wazeos/v2/core/sdk/go/driver"
)

// driver_metadata returns driver information
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

// driver_init initializes the driver
//
//export driver_init
func driverInit(configPtr, configLen uint32) uint32 {
	// Parse config if needed
	config, err := wazeos.ParseConfig(configPtr, configLen)
	if err != nil {
		return 1 // Error
	}

	// Initialize driver state
	// ...

	return 0 // Success
}

// driver_call handles requests
//
//export driver_call
func driverCall(requestPtr, requestLen uint32) uint32 {
	// Parse request
	req, err := wazeos.ParseRequest(requestPtr, requestLen)
	if err != nil {
		return wazeos.ReturnError(400, "Invalid request")
	}

	// Handle the request
	response := "Echo: " + req.URI
	return wazeos.ReturnResponse(200, nil, []byte(response))
}

func main() {}
```

### 2. Build

```bash
tinygo build -o driver.wasm \
    -target=wasi \
    -no-debug \
    -opt=2 \
    main.go
```

### 3. Install

```bash
wazeos driver install driver.wasm
```

## Examples

### Echo Driver

```go
//export driver_call
func driverCall(requestPtr, requestLen uint32) uint32 {
	req, _ := wazeos.ParseRequest(requestPtr, requestLen)

	// Echo back the request URI
	response := map[string]interface{}{
		"echo":      req.URI,
		"operation": req.Operation,
	}
	responseJSON, _ := json.Marshal(response)

	return wazeos.ReturnResponse(200, nil, responseJSON)
}
```

### Key-Value Store Driver

```go
var store = make(map[string][]byte)

//export driver_call
func driverCall(requestPtr, requestLen uint32) uint32 {
	req, _ := wazeos.ParseRequest(requestPtr, requestLen)

	// Parse URI: kv://key
	key := strings.TrimPrefix(req.URI, "kv://")

	operation := req.Headers["operation"]
	switch operation {
	case "get":
		value, exists := store[key]
		if !exists {
			return wazeos.ReturnError(404, "Key not found")
		}
		return wazeos.ReturnResponse(200, nil, value)

	case "set":
		store[key] = req.Body
		return wazeos.ReturnResponse(200, nil, []byte("OK"))

	case "delete":
		delete(store, key)
		return wazeos.ReturnResponse(200, nil, []byte("OK"))

	default:
		return wazeos.ReturnError(400, "Invalid operation")
	}
}
```

### HTTP Proxy Driver

```go
//export driver_call
func driverCall(requestPtr, requestLen uint32) uint32 {
	ctx := wazeos.NewContext()
	req, _ := wazeos.ParseRequest(requestPtr, requestLen)

	// Forward to native HTTP driver
	nativeURI := strings.Replace(req.URI, "myhttp://", "http://", 1)
	resp, err := ctx.Call(nativeURI, req.Headers, req.Body)
	if err != nil {
		return wazeos.ReturnError(500, err.Error())
	}

	// Add custom header
	headers := resp.Headers
	if headers == nil {
		headers = make(map[string]string)
	}
	headers["X-Proxy"] = "myhttp-driver"

	return wazeos.ReturnResponse(resp.StatusCode, headers, resp.Body)
}
```

### Composable Driver (Calling Other Drivers)

```go
//export driver_call
func driverCall(requestPtr, requestLen uint32) uint32 {
	ctx := wazeos.NewContext()
	req, _ := wazeos.ParseRequest(requestPtr, requestLen)

	// Read config from file
	configResp, err := ctx.Call("file:///etc/mydriver.json", nil, nil)
	if err != nil {
		return wazeos.ReturnError(500, "Config not found")
	}

	var config map[string]interface{}
	json.Unmarshal(configResp.Body, &config)

	// Use config to handle request
	// ...

	return wazeos.ReturnResponse(200, nil, []byte("OK"))
}
```

## API Reference

### Metadata

```go
type Metadata struct {
	Name         string   // "my-driver"
	Version      string   // "1.0.0"
	Class        string   // "io.connect", "runtime", etc.
	URIPattern   string   // "mydriver://**"
	Capabilities []string // ["call", "stream"]
}
```

### Request & Response

```go
type Request struct {
	URI       string
	Operation string
	Args      map[string]interface{}
	Headers   map[string]string
	Body      []byte
}

type Response struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
	Error      *string
}
```

### Context (for composing drivers)

```go
type Context struct{}

// Call another driver
func (c *Context) Call(uri string, headers map[string]string, body []byte) (*Response, error)

// Create persistent handle
func (c *Context) CreateHandle(uri string) (string, error)

// Close handle
func (c *Context) CloseHandle(handleURI string) error
```

### Helper Functions

```go
// Parse incoming request
func ParseRequest(requestPtr, requestLen uint32) (*Request, error)

// Parse driver configuration
func ParseConfig(configPtr, configLen uint32) (map[string]interface{}, error)

// Return metadata
func ReturnMetadata(meta Metadata) uint32

// Return success response
func ReturnResponse(statusCode int, headers map[string]string, body []byte) uint32

// Return error response
func ReturnError(statusCode int, message string) uint32

// Memory helpers
func ReadString(ptr, length uint32) string
func ReadBytes(ptr, length uint32) []byte
func WriteString(s string) uint32
```

## Driver Contract

All drivers must export these three functions:

### driver_metadata()

Returns JSON metadata about the driver.

```go
//export driver_metadata
func driverMetadata() uint32 {
	return wazeos.ReturnMetadata(wazeos.Metadata{
		Name:         "my-driver",
		Version:      "1.0.0",
		Class:        "io.connect",
		URIPattern:   "mydriver://**",
		Capabilities: []string{"call"},
	})
}
```

### driver_init(configPtr, configLen uint32) uint32

Initializes the driver with configuration. Returns 0 on success, non-zero on error.

```go
//export driver_init
func driverInit(configPtr, configLen uint32) uint32 {
	config, err := wazeos.ParseConfig(configPtr, configLen)
	if err != nil {
		return 1
	}
	// Initialize driver...
	return 0
}
```

### driver_call(requestPtr, requestLen uint32) uint32

Handles a request and returns a response pointer.

```go
//export driver_call
func driverCall(requestPtr, requestLen uint32) uint32 {
	req, _ := wazeos.ParseRequest(requestPtr, requestLen)
	// Handle request...
	return wazeos.ReturnResponse(200, nil, []byte("OK"))
}
```

## Building

### Development

```bash
tinygo build -o driver.wasm -target=wasi main.go
```

### Production (Optimized)

```bash
tinygo build -o driver.wasm \
    -target=wasi \
    -no-debug \
    -opt=2 \
    -scheduler=none \
    main.go
```

## Testing

```bash
# Verify exports
wasm-objdump -x driver.wasm | grep "export.*driver_"

# Test with WazeOS
wazeos driver test driver.wasm

# Integration test
go test ./...
```

## Examples

See [examples/](examples/) directory:
- [echo/](examples/echo/) - Simple echo driver

## Best Practices

1. **Validate inputs**: Always parse and validate requests
2. **Handle errors**: Return proper error responses with status codes
3. **Keep state minimal**: Drivers should be mostly stateless
4. **Compose when possible**: Use ctx.Call() to leverage other drivers
5. **Document your URIs**: Clearly document the URI patterns you handle

## See Also

- [Multi-Language Support Guide](../../../docs/LANGUAGE_SUPPORT.md)
- [WASM Driver Contract](../../../../drivers/runtime/wasm/loader.go)
- [TinyGo Documentation](https://tinygo.org/docs/)
- [Rust Driver SDK](../../rust/driver/) - Alternative SDK
