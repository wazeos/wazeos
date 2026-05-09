# WazeOS App SDK for Go

Build MCP tools for WazeOS using Go and TinyGo.

**Status**: Production-ready
**Language**: Go (compiled to WASM via TinyGo)

## Overview

This SDK lets you build MCP tools in Go that:
- Run as sandboxed WASM modules
- Access I/O drivers through the IO Bus
- Integrate with Claude Desktop and other MCP clients
- Use familiar Go idioms and patterns

## Prerequisites

### 1. Install TinyGo

TinyGo is required to compile Go to WASM with WASI support:

```bash
# macOS
brew install tinygo

# Linux
wget https://github.com/tinygo-org/tinygo/releases/download/v0.31.0/tinygo_0.31.0_amd64.deb
sudo dpkg -i tinygo_0.31.0_amd64.deb

# Verify installation
tinygo version
```

## Quick Start

### 1. Create a new tool

```go
package main

import (
	"encoding/json"
	wazeos "github.com/wazeos/wazeos/v2/core/sdk/go/app"
)

//export wazeos_tool_invoke
func toolInvoke(argsPtr, argsLen uint32) uint32 {
	ctx := wazeos.NewContext()

	// Parse arguments
	args, err := wazeos.ParseArgs(argsPtr, argsLen)
	if err != nil {
		return wazeos.ReturnError(err.Error())
	}

	path := args["path"].(string)

	// Read a file
	resp, err := ctx.Call("file://"+path, nil, nil)
	if err != nil {
		return wazeos.ReturnError(err.Error())
	}

	// Return the result
	result := map[string]interface{}{
		"content": string(resp.Body),
		"size":    len(resp.Body),
	}
	return wazeos.ReturnSuccess(result)
}

//export wazeos_tool_metadata
func toolMetadata() uint32 {
	return wazeos.ReturnMetadata("file-reader", "1.0.0")
}

func main() {}
```

### 2. Create go.mod

```bash
go mod init my-tool
go mod edit -replace github.com/wazeos/wazeos/v2=../../..
go mod tidy
```

### 3. Build with TinyGo

```bash
tinygo build -o tool.wasm \
    -target=wasi \
    -no-debug \
    -opt=2 \
    main.go
```

### 4. Test with WazeOS

```bash
wazeos app install ./tool.wasm
```

## Examples

### File Operations

```go
//export wazeos_tool_invoke
func toolInvoke(argsPtr, argsLen uint32) uint32 {
	ctx := wazeos.NewContext()
	args := wazeos.MustParseArgs(argsPtr, argsLen)

	// Read file
	resp, err := ctx.Call("file:///tmp/data.txt", nil, nil)
	if err != nil {
		return wazeos.ReturnError(err.Error())
	}

	// Write file
	headers := map[string]string{"operation": "write"}
	body := []byte("Hello, World!")
	_, err = ctx.Call("file:///tmp/output.txt", headers, body)
	if err != nil {
		return wazeos.ReturnError(err.Error())
	}

	return wazeos.ReturnSuccess(map[string]interface{}{
		"message": "Files processed successfully",
	})
}
```

### Shell Commands

```go
//export wazeos_tool_invoke
func toolInvoke(argsPtr, argsLen uint32) uint32 {
	ctx := wazeos.NewContext()

	// Execute shell command
	headers := map[string]string{
		"command": "date '+%Y-%m-%d %H:%M:%S'",
	}
	resp, err := ctx.Call("shell://exec", headers, nil)
	if err != nil {
		return wazeos.ReturnError(err.Error())
	}

	return wazeos.ReturnSuccess(map[string]interface{}{
		"timestamp": string(resp.Body),
	})
}
```

### HTTP Requests

```go
//export wazeos_tool_invoke
func toolInvoke(argsPtr, argsLen uint32) uint32 {
	ctx := wazeos.NewContext()

	// Make HTTP GET request
	resp, err := ctx.Call("https://api.example.com/data", nil, nil)
	if err != nil {
		return wazeos.ReturnError(err.Error())
	}

	// Parse JSON response
	var data map[string]interface{}
	json.Unmarshal(resp.Body, &data)

	return wazeos.ReturnSuccess(data)
}
```

### Error Handling

```go
//export wazeos_tool_invoke
func toolInvoke(argsPtr, argsLen uint32) uint32 {
	ctx := wazeos.NewContext()

	resp, err := ctx.Call("file:///nonexistent.txt", nil, nil)
	if err != nil {
		// Check if it's an IO Bus error with status code
		if ioBusErr, ok := err.(*wazeos.IOBusError); ok {
			return wazeos.ReturnError(
				fmt.Sprintf("File error (status %d): %s",
					ioBusErr.StatusCode, ioBusErr.Message))
		}
		return wazeos.ReturnError(err.Error())
	}

	return wazeos.ReturnSuccess(map[string]interface{}{
		"content": string(resp.Body),
	})
}
```

## API Reference

### Context

```go
type Context struct{}

// NewContext creates a new application context
func NewContext() *Context

// Call makes a generic IO Bus call to any driver
func (c *Context) Call(uri string, headers map[string]string, body []byte) (*Response, error)
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

### Helper Functions

```go
// Parse arguments from tool invocation
func ParseArgs(argsPtr, argsLen uint32) (map[string]interface{}, error)

// Return success response
func ReturnSuccess(result interface{}) uint32

// Return error response
func ReturnError(message string) uint32

// Return tool metadata
func ReturnMetadata(name, version string) uint32

// Memory helpers
func ReadString(ptr, length uint32) string
func ReadBytes(ptr, length uint32) []byte
func WriteString(s string) uint32
```

## Building

### Development Build

```bash
tinygo build -o tool.wasm -target=wasi main.go
```

### Production Build (Optimized)

```bash
tinygo build -o tool.wasm \
    -target=wasi \
    -no-debug \
    -opt=2 \
    -scheduler=none \
    main.go
```

### Build Flags Explained

- `-target=wasi`: Target WASI (WebAssembly System Interface)
- `-no-debug`: Strip debug information
- `-opt=2`: Optimization level (0-2, where 2 is most optimized)
- `-scheduler=none`: Disable goroutine scheduler (reduces binary size)

## Limitations

### TinyGo vs Standard Go

TinyGo has some limitations compared to standard Go:

**Supported**:
- ✅ Standard library basics (strings, encoding/json, fmt)
- ✅ Structs, interfaces, methods
- ✅ Slices, maps, pointers
- ✅ JSON marshal/unmarshal
- ✅ Error handling

**Limited**:
- ⚠️ Reflection (limited support)
- ⚠️ Goroutines (use `-scheduler=asyncify` if needed)
- ⚠️ CGo (not supported in WASM)
- ⚠️ Some stdlib packages may not work

**Not Supported**:
- ❌ Go plugins
- ❌ `net` package in WASM
- ❌ OS-specific features

See [TinyGo documentation](https://tinygo.org/docs/reference/lang-support/) for full details.

## Troubleshooting

### Build Errors

**"undefined: encoding/json"**
```bash
# Some packages may need explicit import
go get encoding/json
```

**"wasm-ld: error: unknown file type"**
```bash
# Make sure you're using TinyGo, not regular Go
which tinygo
tinygo version
```

### Runtime Errors

**"host function not found"**
- Ensure WazeOS runtime version matches SDK
- Check that exports are defined correctly

**"invalid memory reference"**
- Check pointer arithmetic
- Verify string/byte slice lifetimes

## Testing

```bash
# Unit test your business logic
go test ./...

# Test WASM module with WazeOS
wazeos app test tool.wasm

# Integration test
wazeos mcp start --test-mode
```

## Examples

See [examples/](examples/) directory for complete examples:
- [hello/](examples/hello/) - Basic "Hello World" tool
- More examples coming soon

## Best Practices

1. **Keep it simple**: Avoid complex reflection and concurrency
2. **Handle errors**: Always check errors from IO Bus calls
3. **Validate inputs**: Parse and validate all user inputs
4. **Small binaries**: Use `-opt=2` and `-no-debug` for production
5. **Test thoroughly**: Test with real WazeOS runtime, not just unit tests

## See Also

- [Multi-Language Support Guide](../../../docs/LANGUAGE_SUPPORT.md)
- [TinyGo Documentation](https://tinygo.org/docs/)
- [WASI Specification](https://github.com/WebAssembly/WASI)
- [Rust App SDK](../rust/app/) - Alternative SDK
