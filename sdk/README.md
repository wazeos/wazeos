# WazeOS Driver SDK

Shared types and utilities for building WazeOS WASM drivers.

## Overview

The WazeOS Driver SDK provides a common foundation for building WASM drivers in Go using TinyGo. It eliminates boilerplate code and ensures consistency across all drivers.

## Features

- **Shared Type Definitions**: ResourceCall, ResourceResult, AuthPayload, AuthResult, etc.
- **Memory Management**: Automatic allocate/deallocate exports
- **Host Function Helpers**: Easy-to-use wrappers for kernel services
- **Handler Interfaces**: Clean abstraction for driver logic
- **JSON Serialization**: Built-in marshaling/unmarshaling

## Installation

### Add to your driver project

```bash
cd examples/drivers/my-driver
go get github.com/wazeos/wazeos/sdk/driver
```

### In your `main.go`

```go
import "github.com/wazeos/wazeos/sdk/driver"
```

## Quick Start

### Resource Driver

```go
package main

import (
    "github.com/wazeos/wazeos/sdk/driver"
)

// Implement the ResourceHandler interface
type MyDriver struct{}

func (d *MyDriver) HandleCall(call *driver.ResourceCall) (*driver.ResourceResult, error) {
    switch call.Method {
    case "READ":
        // Handle read operation
        return driver.NewResourceResult(200, []byte("data")), nil
    default:
        return driver.NewErrorResult(405, "method not allowed"), nil
    }
}

// Export the handle_call function
//
//export handle_call
func handleCall(callPtr, callLen uint32) uint32 {
    handler := &MyDriver{}
    return driver.ServeResource(handler, callPtr, callLen)
}

func main() {
    // Driver initialization
    println("Driver initialized")
    select {}
}
```

### Security (Authentication) Driver

```go
package main

import (
    "github.com/wazeos/wazeos/sdk/driver"
    "strings"
)

// Implement the AuthHandler interface
type MyAuthDriver struct{}

func (d *MyAuthDriver) Authenticate(payload *driver.AuthPayload) (*driver.AuthResult, error) {
    authHeader, ok := payload.Headers["Authorization"]
    if !ok {
        return driver.NewAuthAbstain(), nil
    }

    if strings.HasPrefix(authHeader, "Bearer ") {
        token := strings.TrimPrefix(authHeader, "Bearer ")
        // Validate token...
        return driver.NewAuthResult("user:alice"), nil
    }

    return driver.NewAuthAbstain(), nil
}

// Export the authenticate function
//
//export authenticate
func authenticate(payloadPtr, payloadLen uint32) uint32 {
    handler := &MyAuthDriver{}
    return driver.ServeAuth(handler, payloadPtr, payloadLen)
}

func main() {
    println("Auth driver initialized")
    select {}
}
```

## API Reference

### Types

#### ResourceCall

Represents an IO call from a WASM app to a resource driver.

```go
type ResourceCall struct {
    URI        string
    Method     string
    Headers    map[string]string
    Body       []byte
    AccessMode AccessBits
}
```

#### ResourceResult

Represents the result of a resource call.

```go
type ResourceResult struct {
    StatusCode int
    Headers    map[string]string
    Body       []byte
    Error      string
}
```

**Constructors**:
- `NewResourceResult(statusCode int, body []byte) *ResourceResult`
- `NewErrorResult(statusCode int, message string) *ResourceResult`

#### AuthPayload

Represents authentication input from a request.

```go
type AuthPayload struct {
    Headers map[string]string
    Body    []byte
}
```

#### AuthResult

Represents the authentication result.

```go
type AuthResult struct {
    Principal string // e.g. "user:alice"
    Error     string // Error message or "abstain"
}
```

**Constructors**:
- `NewAuthResult(principal string) *AuthResult`
- `NewAuthError(message string) *AuthResult`
- `NewAuthAbstain() *AuthResult`

**Methods**:
- `IsAbstain() bool` - Check if result is an abstain

#### AccessBits

Bitfield for read/write/execute permissions.

```go
type AccessBits uint8

const (
    AccessRead    AccessBits = 1 << 0 // 0x01
    AccessWrite   AccessBits = 1 << 1 // 0x02
    AccessExecute AccessBits = 1 << 2 // 0x04
)
```

**Methods**:
- `String() string` - Returns "r", "rw", "rwx", etc.
- `Has(permission AccessBits) bool` - Check if permission included

### Memory Management

The SDK automatically exports memory management functions:

```go
//export allocate
func Allocate(size uint32) uint32

//export deallocate
func Deallocate()
```

**Helper Functions**:
- `GetMemory() []byte` - Get current memory buffer
- `CopyFromMemory(offset, length uint32) []byte` - Copy data from buffer
- `CopyToMemory(data []byte) uint32` - Copy data to buffer, returns pointer
- `ReadString(offset, length uint32) string` - Read string from buffer

### Host Functions

#### CallResourceCall

Make a resource call to another driver.

```go
result, err := driver.CallResourceCall(&driver.ResourceCall{
    URI:    "file:///tmp/data.txt",
    Method: "READ",
    Headers: make(map[string]string),
    Body:   nil,
    AccessMode: driver.AccessRead,
})

if err != nil {
    // Handle error
}

data := result.Body
```

#### CheckAuthorization

Verify if a URI access is permitted.

```go
allowed, err := driver.CheckAuthorization(
    "file:///tmp/test.txt",
    "rw",
    permissionContext,
)

if !allowed {
    return driver.NewErrorResult(403, "access denied")
}
```

#### ResolvePackage

Resolve an app name to its full ID.

```go
appID, err := driver.ResolvePackage("myapp")
// Returns: "author/myapp_1.0.0"
```

### Handler Interfaces

#### ResourceHandler

Interface for resource drivers.

```go
type ResourceHandler interface {
    HandleCall(call *ResourceCall) (*ResourceResult, error)
}
```

**Usage**:
```go
//export handle_call
func handleCall(callPtr, callLen uint32) uint32 {
    return driver.ServeResource(myHandler, callPtr, callLen)
}
```

#### AuthHandler

Interface for authentication drivers.

```go
type AuthHandler interface {
    Authenticate(payload *AuthPayload) (*AuthResult, error)
}
```

**Usage**:
```go
//export authenticate
func authenticate(payloadPtr, payloadLen uint32) uint32 {
    return driver.ServeAuth(myHandler, payloadPtr, payloadLen)
}
```

## Building with the SDK

### go.mod

```go
module example.com/my-driver

go 1.21

require github.com/wazeos/wazeos/sdk v0.0.0

// For local development
replace github.com/wazeos/wazeos/sdk => ../../sdk
```

### Build Command

```bash
tinygo build -o app.wasm -target=wasi main.go
```

## Best Practices

### 1. Use Handler Interfaces

Don't manually handle JSON serialization - use the `ServeResource` and `ServeAuth` helpers:

**Bad**:
```go
//export handle_call
func handleCall(callPtr, callLen uint32) uint32 {
    // Manual JSON handling...
    callData := driver.GetMemory()[callPtr:callPtr+callLen]
    var call driver.ResourceCall
    json.Unmarshal(callData, &call)
    // ...
}
```

**Good**:
```go
//export handle_call
func handleCall(callPtr, callLen uint32) uint32 {
    return driver.ServeResource(&MyDriver{}, callPtr, callLen)
}
```

### 2. Use Constructors

Always use constructors for creating results:

**Bad**:
```go
return &driver.ResourceResult{
    StatusCode: 200,
    Headers:    make(map[string]string),
    Body:       data,
    Error:      "",
}
```

**Good**:
```go
return driver.NewResourceResult(200, data)
```

### 3. Handle Errors Properly

Return proper error results instead of panicking:

**Bad**:
```go
func (d *MyDriver) HandleCall(call *driver.ResourceCall) (*driver.ResourceResult, error) {
    if call.URI == "" {
        panic("invalid URI")
    }
    // ...
}
```

**Good**:
```go
func (d *MyDriver) HandleCall(call *driver.ResourceCall) (*driver.ResourceResult, error) {
    if call.URI == "" {
        return driver.NewErrorResult(400, "URI required"), nil
    }
    // ...
}
```

### 4. Use Abstain for Auth Drivers

Return abstain when your driver doesn't handle the auth type:

```go
func (d *MyAuthDriver) Authenticate(payload *driver.AuthPayload) (*driver.AuthResult, error) {
    authHeader, ok := payload.Headers["Authorization"]
    if !ok || !strings.HasPrefix(authHeader, "Bearer ") {
        return driver.NewAuthAbstain(), nil
    }
    // ... validate token
}
```

## Examples

### Complete Resource Driver

```go
package main

import (
    "fmt"
    "os"
    "github.com/wazeos/wazeos/sdk/driver"
)

type FileDriver struct{}

func (d *FileDriver) HandleCall(call *driver.ResourceCall) (*driver.ResourceResult, error) {
    switch call.Method {
    case "READ":
        data, err := os.ReadFile(call.URI)
        if err != nil {
            return driver.NewErrorResult(404, "file not found"), nil
        }
        return driver.NewResourceResult(200, data), nil

    case "WRITE":
        if err := os.WriteFile(call.URI, call.Body, 0644); err != nil {
            return driver.NewErrorResult(500, err.Error()), nil
        }
        return driver.NewResourceResult(200, []byte("success")), nil

    default:
        return driver.NewErrorResult(405, "method not allowed"), nil
    }
}

//export handle_call
func handleCall(callPtr, callLen uint32) uint32 {
    return driver.ServeResource(&FileDriver{}, callPtr, callLen)
}

func main() {
    fmt.Println("File driver initialized")
    select {}
}
```

### Complete Auth Driver

```go
package main

import (
    "encoding/base64"
    "fmt"
    "strings"
    "github.com/wazeos/wazeos/sdk/driver"
)

type BasicAuthDriver struct {
    credentials map[string]string
}

func NewBasicAuthDriver() *BasicAuthDriver {
    return &BasicAuthDriver{
        credentials: map[string]string{
            "admin": "password",
        },
    }
}

func (d *BasicAuthDriver) Authenticate(payload *driver.AuthPayload) (*driver.AuthResult, error) {
    authHeader, ok := payload.Headers["Authorization"]
    if !ok {
        return driver.NewAuthAbstain(), nil
    }

    if !strings.HasPrefix(authHeader, "Basic ") {
        return driver.NewAuthAbstain(), nil
    }

    encoded := strings.TrimPrefix(authHeader, "Basic ")
    decoded, err := base64.StdEncoding.DecodeString(encoded)
    if err != nil {
        return driver.NewAuthError("invalid encoding"), nil
    }

    parts := strings.SplitN(string(decoded), ":", 2)
    if len(parts) != 2 {
        return driver.NewAuthError("invalid format"), nil
    }

    username, password := parts[0], parts[1]
    expectedPassword, exists := d.credentials[username]
    if !exists || password != expectedPassword {
        return driver.NewAuthError("invalid credentials"), nil
    }

    return driver.NewAuthResult(fmt.Sprintf("user:%s", username)), nil
}

//export authenticate
func authenticate(payloadPtr, payloadLen uint32) uint32 {
    handler := NewBasicAuthDriver()
    return driver.ServeAuth(handler, payloadPtr, payloadLen)
}

func main() {
    fmt.Println("Basic Auth driver initialized")
    select {}
}
```

## Migration from Manual Implementation

### Before (Manual)

```go
package main

import (
    "encoding/json"
    "unsafe"
)

type ResourceCall struct {
    URI    string `json:"uri"`
    Method string `json:"method"`
    // ... more fields
}

var memoryBuffer []byte

//export allocate
func allocate(size uint32) uint32 {
    memoryBuffer = make([]byte, size)
    return uint32(uintptr(unsafe.Pointer(&memoryBuffer[0])))
}

//export handle_call
func handleCall(callPtr, callLen uint32) uint32 {
    callData := memoryBuffer[callPtr:callPtr+callLen]
    var call ResourceCall
    json.Unmarshal(callData, &call)

    // ... handle call ...

    resultJSON, _ := json.Marshal(result)
    resultPtr := allocate(uint32(len(resultJSON)))
    copy(memoryBuffer, resultJSON)
    return resultPtr
}
```

### After (With SDK)

```go
package main

import "github.com/wazeos/wazeos/sdk/driver"

type MyDriver struct{}

func (d *MyDriver) HandleCall(call *driver.ResourceCall) (*driver.ResourceResult, error) {
    // ... handle call ...
    return driver.NewResourceResult(200, data), nil
}

//export handle_call
func handleCall(callPtr, callLen uint32) uint32 {
    return driver.ServeResource(&MyDriver{}, callPtr, callLen)
}
```

**Benefits**:
- ✅ 80% less boilerplate code
- ✅ No manual JSON handling
- ✅ Type-safe interfaces
- ✅ Consistent error handling
- ✅ Built-in memory management
- ✅ Access to host functions

## Troubleshooting

### Import Error

**Error**: `package github.com/wazeos/wazeos/sdk/driver is not in GOROOT`

**Solution**: Add replace directive in go.mod:
```go
replace github.com/wazeos/wazeos/sdk => ../../sdk
```

### TinyGo Compilation Error

**Error**: `undefined: driver.ServeResource`

**Solution**: Ensure SDK is imported correctly:
```go
import "github.com/wazeos/wazeos/sdk/driver"
```

### Runtime Error

**Error**: `undefined: Allocate`

**Solution**: The SDK automatically exports `allocate` - don't define it yourself.

## Contributing

To improve the SDK:

1. Add new types or helpers to appropriate files
2. Update this README with examples
3. Test with all example drivers
4. Submit PR

## License

Same as WazeOS main project.
