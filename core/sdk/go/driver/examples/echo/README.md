# Echo Driver - Go Example

A minimal WazeOS I/O driver written in Go that echoes back requests.

## Build

```bash
# Initialize Go module
go mod init echo-driver
go mod edit -replace github.com/wazeos/wazeos/v2=../../../../..
go mod tidy

# Build with TinyGo
tinygo build -o echo.wasm \
    -target=wasi \
    -no-debug \
    -opt=2 \
    main.go
```

## Install

```bash
wazeos driver install echo.wasm
```

## Usage

Once installed, any URI matching `echo://**` will be handled by this driver:

```go
// From an app
ctx := wazeos.NewContext()
resp, err := ctx.Call("echo://test",
    map[string]string{"custom": "header"},
    []byte("Hello, Echo!"))

// Response will contain:
// {
//   "uri": "echo://test",
//   "operation": "call",
//   "headers": {"custom": "header"},
//   "body_size": 12,
//   "body": "Hello, Echo!"
// }
```

## Features Demonstrated

- Driver metadata export
- Configuration parsing (with graceful handling)
- Request parsing
- JSON response building
- Error handling with proper status codes
- Headers in response

## Testing

```bash
# Verify exports
wasm-objdump -x echo.wasm | grep "export.*driver_"

# Should see:
# - driver_metadata
# - driver_init
# - driver_call
```
