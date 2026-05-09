# Hello World - Go Example

A minimal WazeOS MCP tool written in Go.

## Build

```bash
# Initialize Go module
go mod init hello-tool
go mod edit -replace github.com/wazeos/wazeos/v2=../../../../..
go mod tidy

# Build with TinyGo
tinygo build -o hello.wasm \
    -target=wasi \
    -no-debug \
    -opt=2 \
    main.go
```

## Install

```bash
wazeos app install hello.wasm
```

## Usage

In Claude Desktop or via MCP:

```json
{
  "name": "hello-go",
  "arguments": {
    "name": "Alice"
  }
}
```

Response:
```json
{
  "success": true,
  "result": {
    "greeting": "Hello, Alice! (Time: 14:32:15)",
    "name": "Alice"
  }
}
```

## Features Demonstrated

- Argument parsing
- Optional IO Bus calls (shell command)
- Error handling with graceful fallback
- Structured JSON responses
