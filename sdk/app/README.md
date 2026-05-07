# WazeOS App SDK

Build WebAssembly applications for WazeOS with Go.

## Overview

The WazeOS App SDK provides high-level, ergonomic APIs for building business logic applications. It complements the driver SDK (for infrastructure components) with app-focused abstractions.

## Features

- **Simple handler interfaces** - CLI, Request, Stream, and Message patterns
- **High-level I/O operations** - File, HTTP, app-to-app calls, message queues
- **Structured logging** - JSON logging with request context
- **Type-safe errors** - Consistent error handling with HTTP status codes
- **Testing support** - Mock implementations for local testing
- **50% less boilerplate** - Compared to using the driver SDK directly

## Quick Start

### Basic CLI App

```go
package main

import "github.com/wazeos/wazeos/sdk/app"

type HelloApp struct{}

func (a *HelloApp) Run(ctx *app.Context, args []string) (*app.Response, error) {
    name := "World"
    if len(args) > 0 {
        name = args[0]
    }

    ctx.Log.Info("greeting user", app.String("name", name))

    message := "Hello, " + name + "!"
    return app.SuccessString(message), nil
}

func main() {
    app.RunCLI(&HelloApp{})
}
```

### Building

```bash
tinygo build -o app.wasm -target=wasi main.go
```

## Handler Patterns

### 1. CLI Handler

For command-line style apps with arguments:

```go
type CLIHandler interface {
    Run(ctx *Context, args []string) (*Response, error)
}

func main() {
    app.RunCLI(&MyApp{})
}
```

### 2. Request Handler

For HTTP-style request/response apps:

```go
type RequestHandler interface {
    Handle(ctx *Context, req *Request) (*Response, error)
}

func main() {
    app.RunHandler(&MyApp{})
}
```

### 3. Stream Handler

For line-by-line stream processing:

```go
type StreamHandler interface {
    ProcessLine(ctx *Context, line []byte) error
    Finalize(ctx *Context) (*Response, error)
}

func main() {
    app.RunStream(&MyApp{})
}
```

### 4. Message Handler

For processing messages from queues:

```go
type MessageHandler interface {
    HandleMessage(ctx *Context, msg *Message) error
}

func main() {
    opts := &app.ConsumeOptions{MaxCount: 10}
    app.RunConsumer("events.users", &MyApp{}, opts)
}
```

## I/O Operations

### File Operations

```go
// Read file
data, err := ctx.IO.ReadFile("/tmp/data.txt")

// Write file
err := ctx.IO.WriteFile("/tmp/output.txt", []byte("content"))

// Delete file
err := ctx.IO.DeleteFile("/tmp/old.txt")

// List files
files, err := ctx.IO.ListFiles("/tmp")
```

### HTTP Operations

```go
// GET request
resp, err := ctx.IO.Get("https://api.example.com/data")

// POST request
headers := map[string]string{"Content-Type": "application/json"}
resp, err := ctx.IO.Post("https://api.example.com/data", body, headers)

// Custom request
resp, err := ctx.IO.Request("PUT", "https://...", body, headers)
```

### App-to-App Calls

```go
// Call another app
result, err := ctx.IO.CallApp("user-service", "get", "123")

// Call with input data
result, err := ctx.IO.CallAppWithInput("processor", inputData, "transform")
```

### Message Queue Operations

```go
// Publish message
err := ctx.IO.Publish("events.orders", message)

// Publish with partition key
err := ctx.IO.PublishWithKey("events.orders", "user-123", message)

// Consume messages
opts := &app.ConsumeOptions{MaxCount: 10, Timeout: 5}
messages, err := ctx.IO.Consume("events.orders", opts)
```

## Structured Logging

```go
// Log levels
ctx.Log.Debug("debug message", app.String("key", "value"))
ctx.Log.Info("info message", app.Int("count", 42))
ctx.Log.Warn("warning message", app.Bool("flag", true))
ctx.Log.Error("error occurred", app.ErrorField(err))

// Field helpers
app.String(key, value)     // String field
app.Int(key, value)        // Integer field
app.Int64(key, value)      // Int64 field
app.Bool(key, value)       // Boolean field
app.Any(key, value)        // Any value type
app.ErrorField(err)        // Error field
```

## Error Handling

The App SDK provides clear, actionable error messages that help developers understand what went wrong.

### Missing Driver Errors

When a driver isn't installed, you'll see a clear message:

```
FILE_READ_ERROR: no driver found for URI: file:///tmp/data.txt
```

This tells you that you need to install a resource driver (class: `io.resource`) that handles the `file://` scheme.

### Return Errors

Let the SDK handle formatting:

```go
func (a *App) Run(ctx *app.Context, args []string) (*app.Response, error) {
    data, err := ctx.IO.ReadFile("/tmp/data.txt")
    if err != nil {
        return nil, err  // SDK formats and logs automatically
    }
    return app.SuccessString(string(data)), nil
}
```

### Return Custom Responses

```go
func (a *App) Run(ctx *app.Context, args []string) (*app.Response, error) {
    if len(args) < 1 {
        return app.BadRequest("missing required argument"), nil
    }

    data, err := ctx.IO.ReadFile(args[0])
    if err != nil {
        return app.NotFound("file not found"), nil
    }

    return app.Success(data), nil
}
```

### Error Response Builders

```go
app.BadRequest(message)      // 400
app.Forbidden(message)       // 403
app.NotFound(message)        // 404
app.InternalError(message)   // 500
app.Error(statusCode, msg)   // Custom status
```

### Common Error Constants

```go
app.ErrPermissionDenied      // 403
app.ErrNotFound              // 404
app.ErrInvalidInput          // 400
app.ErrInternal              // 500
app.ErrTimeout               // 504
app.ErrUnavailable           // 503
```

## Testing

### Create Test Context

```go
import "testing"

func TestMyApp(t *testing.T) {
    ctx := app.TestContext()

    // Mock file
    mockIO := app.GetMockIO(ctx)
    mockIO.MockFile("/test.txt", []byte("hello"))

    // Run handler
    handler := &MyApp{}
    resp, err := handler.Run(ctx, []string{"/test.txt"})

    // Assert
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if string(resp.Body) != "hello" {
        t.Errorf("expected 'hello', got '%s'", resp.Body)
    }
}
```

### Mock I/O Operations

```go
mockIO := app.GetMockIO(ctx)

// Mock files
mockIO.MockFile("/tmp/data.txt", []byte("content"))

// Mock app responses
mockIO.MockApp("user-service", app.SuccessString("user data"))

// Mock HTTP responses
mockIO.MockHTTP("https://api.example.com/data", &app.HTTPResponse{
    StatusCode: 200,
    Body:       []byte("response"),
})

// Mock messages
mockIO.MockMessages("events.orders", []*app.Message{
    {ID: "1", Topic: "events.orders", Body: []byte("msg1")},
})

// Check call log
calls := mockIO.GetCallLog()
if err := app.AssertCallCount(mockIO, "ReadFile", 1); err != nil {
    t.Error(err)
}
```

### Test with Permissions

```go
ctx := app.TestContextWithPermissions([]driver.PermissionEntry{
    app.AllowFile("/tmp/*", "rw"),
    app.AllowHTTP("https://api.example.com/*"),
    app.AllowApp("user-service"),
})

// Check permissions
if !ctx.HasPermission("file:///tmp/data.txt", "rw") {
    t.Error("should have read/write permission")
}
```

## Context

The context provides access to execution metadata and I/O operations:

```go
type Context struct {
    // Execution metadata (read-only)
    RequestID   string                    // Unique request identifier
    TraceID     string                    // Distributed tracing ID
    Principal   string                    // Authenticated user (e.g., "user:alice")
    Permissions *driver.PermissionContext // URI-based permissions
    Metadata    map[string]string         // Additional metadata

    // I/O operations
    IO  IOClient // High-level I/O client
    Log *Logger  // Structured logger
}

// Check permissions
hasAccess := ctx.HasPermission("file:///tmp/*", "rw")
```

## Response Types

```go
type Response struct {
    StatusCode int               // HTTP status code (200, 404, etc.)
    Headers    map[string]string // Response metadata
    Body       []byte            // Response payload
    ExitCode   int               // Process exit code (0 = success)
}

// Success responses
app.Success([]byte("data"))
app.SuccessString("message")
app.SuccessJSON(struct{ Name string }{"Alice"})

// Error responses
app.BadRequest("invalid input")      // 400
app.Forbidden("access denied")       // 403
app.NotFound("not found")            // 404
app.InternalError("internal error")  // 500
```

## Migration from Driver SDK

### Before (Driver SDK)

```go
package main

import (
    "fmt"
    "os"
    "github.com/wazeos/wazeos/sdk/driver"
)

func main() {
    if len(os.Args) < 2 {
        fmt.Fprintln(os.Stderr, "Usage: app <path>")
        os.Exit(1)
    }

    call := &driver.ResourceCall{
        URI:     fmt.Sprintf("file://%s", os.Args[1]),
        Method:  "READ",
        Headers: make(map[string]string),
        Body:    []byte{},
    }

    result, err := driver.CallResourceCall(call)
    if err != nil {
        fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
        os.Exit(1)
    }

    if result.StatusCode != 200 {
        fmt.Fprintf(os.Stderr, "ERROR: %s\n", result.Error)
        os.Exit(1)
    }

    fmt.Println(string(result.Body))
}
```

### After (App SDK)

```go
package main

import "github.com/wazeos/wazeos/sdk/app"

type FileReaderApp struct{}

func (a *FileReaderApp) Run(ctx *app.Context, args []string) (*app.Response, error) {
    if len(args) < 1 {
        return app.BadRequest("usage: app <path>"), nil
    }

    ctx.Log.Info("reading file", app.String("path", args[0]))

    data, err := ctx.IO.ReadFile(args[0])
    if err != nil {
        return nil, err
    }

    return app.SuccessString(string(data)), nil
}

func main() {
    app.RunCLI(&FileReaderApp{})
}
```

**Benefits**: 50% less code, structured logging, automatic error handling, testable interface.

## Examples

See [examples/apps/](../../examples/apps/) for complete examples:
- `file-reader` - CLI app that reads files

## TinyGo Compatibility

The SDK is designed for TinyGo with the following constraints:
- No reflection beyond basic JSON
- Limited stdlib (no `net/http` client, etc.)
- No goroutines or channels
- Minimal dependencies

All I/O operations use resource calls instead of stdlib implementations.
