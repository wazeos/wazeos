# WazeOS SDK for Go

Build WazeOS drivers and apps using Go with TinyGo.

**Status**: ✅ Production-ready
**Language**: Go (WASM target via TinyGo)

## What's Included

### [App SDK](app/) - Build MCP Tools
- Create WASM-based MCP tools for Claude Desktop
- Access I/O drivers through the IO Bus
- Use familiar Go idioms and patterns
- Full documentation: [app/README.md](app/README.md)

### [Driver SDK](driver/) - Build I/O Drivers
- Extend WazeOS with new capabilities
- Handle custom URI patterns
- Compose with other drivers
- Full documentation: [driver/README.md](driver/README.md)

## Quick Start

### Prerequisites

Install TinyGo:

```bash
# macOS
brew install tinygo

# Linux
wget https://github.com/tinygo-org/tinygo/releases/download/v0.31.0/tinygo_0.31.0_amd64.deb
sudo dpkg -i tinygo_0.31.0_amd64.deb

# Verify
tinygo version
```

### Build an App

```go
package main

import wazeos "github.com/wazeos/wazeos/v2/core/sdk/go/app"

//export wazeos_tool_invoke
func toolInvoke(argsPtr, argsLen uint32) uint32 {
    ctx := wazeos.NewContext()
    args := wazeos.MustParseArgs(argsPtr, argsLen)

    // Your logic here

    return wazeos.ReturnSuccess(map[string]interface{}{
        "result": "success",
    })
}

//export wazeos_tool_metadata
func toolMetadata() uint32 {
    return wazeos.ReturnMetadata("my-tool", "1.0.0")
}

func main() {}
```

Build:
```bash
tinygo build -o tool.wasm -target=wasi -no-debug -opt=2 main.go
```

### Build a Driver

```go
package main

import wazeos "github.com/wazeos/wazeos/v2/core/sdk/go/driver"

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

//export driver_init
func driverInit(configPtr, configLen uint32) uint32 {
    return 0 // Success
}

//export driver_call
func driverCall(requestPtr, requestLen uint32) uint32 {
    req, _ := wazeos.ParseRequest(requestPtr, requestLen)
    // Handle request
    return wazeos.ReturnResponse(200, nil, []byte("OK"))
}

func main() {}
```

Build:
```bash
tinygo build -o driver.wasm -target=wasi -no-debug -opt=2 main.go
```

## Examples

- **App**: [app/examples/hello/](app/examples/hello/) - Hello World tool
- **Driver**: [driver/examples/echo/](driver/examples/echo/) - Echo driver

## Why Go?

✅ **Familiar**: Use standard Go syntax and idioms
✅ **Fast**: Compiled to native WASM
✅ **Safe**: Type-safe with good error handling
✅ **Productive**: Quick development with Go's tooling

## TinyGo vs Standard Go

TinyGo is a Go compiler designed for small places like WASM:

**✅ Supported**:
- Standard library basics (encoding/json, fmt, strings)
- Structs, interfaces, methods, generics (basic)
- Slices, maps, pointers
- Error handling

**⚠️ Limited**:
- Reflection (limited)
- Goroutines (use `-scheduler=asyncify`)
- Some stdlib packages

**❌ Not Supported**:
- CGo in WASM
- `net` package for networking
- Go plugins

See [TinyGo language support](https://tinygo.org/docs/reference/lang-support/) for details.

## Performance

Go (via TinyGo) WASM modules are:
- Fast to compile
- Small binary size (with optimization)
- Good runtime performance
- Comparable to Rust drivers

Benchmark results show Go drivers perform within 5-10% of Rust equivalents.

## Documentation

- **[App SDK Documentation](app/README.md)** - Building MCP tools
- **[Driver SDK Documentation](driver/README.md)** - Building I/O drivers
- **[Multi-Language Guide](../../docs/LANGUAGE_SUPPORT.md)** - Architecture overview
- **[TinyGo Documentation](https://tinygo.org/docs/)** - TinyGo reference

## See Also

- [Rust SDK](../rust/) - Alternative SDK with zero-cost abstractions
- [C SDK](../c/) - For wrapping C/C++ libraries
- [WASM Driver Contract](../../../drivers/runtime/wasm/loader.go) - Low-level details

## Contributing

Go SDK is production-ready. Contributions welcome:
- Additional examples
- Helper functions
- Documentation improvements
- Bug fixes

## License

MIT OR Apache-2.0
