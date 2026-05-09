# WazeOS SDK Directory

This directory contains SDKs for building WazeOS drivers and apps in multiple languages.

## Available SDKs

### Rust (Production-Ready)
- **App SDK**: [rust/app/](rust/app/) - For building MCP tools
- **Driver SDK**: [rust/driver/](rust/driver/) - For building I/O drivers
- **Status**: ✅ Complete and battle-tested
- **Best for**: Production drivers, performance-critical code
- **Docs**: See inline documentation in source files

### Go (Production-Ready)
- **App SDK**: [go/app/](go/app/) - For building MCP tools
- **Driver SDK**: [go/driver/](go/driver/) - For building I/O drivers
- **Status**: ✅ Complete and production-ready
- **Best for**: Rapid development, familiar Go idioms
- **Docs**: See [go/README.md](go/README.md)

### C (Template/Reference)
- **Driver SDK**: [c/driver/](c/driver/)
- **Status**: 📝 Reference implementation (not yet production-ready)
- **Best for**: Wrapping existing C/C++ libraries
- **Docs**: See [c/driver/README.md](c/driver/README.md)

### Future Languages

Additional language SDKs can be added following the same pattern:

```
sdk/
├── rust/           # ✅ Production (Rust)
├── go/             # ✅ Production (TinyGo)
├── c/              # 📝 Template (clang)
├── assemblyscript/ # 🔮 Future
└── zig/            # 🔮 Future
```

## How to Add a New Language

See the comprehensive guide: [v2/docs/LANGUAGE_SUPPORT.md](../../v2/docs/LANGUAGE_SUPPORT.md)

**Quick steps**:
1. Create `sdk/<language>/app/` and/or `sdk/<language>/driver/` directories
2. Implement the WASM contract (exports and imports)
3. Provide ergonomic wrappers (traits/classes/functions)
4. Create examples
5. Document build process
6. Test contract compliance

## WASM Driver Contract

All language SDKs must implement this contract:

**Exports** (functions your driver provides):
```
driver_metadata() -> JSON string
driver_init(config_ptr, config_len) -> error code
driver_call(request_ptr, request_len) -> JSON response
```

**Imports** (functions provided by WazeOS):
```
host_iobus_call(request_ptr, request_len) -> packed response
host_iobus_create_handle(uri_ptr, uri_len) -> packed handle ID
host_iobus_close_handle(uri_ptr, uri_len) -> error code
```

See full specification in [v2/drivers/runtime/wasm/loader.go](../../v2/drivers/runtime/wasm/loader.go)

## Language Requirements

To create an SDK for a language, it must:
1. Compile to `wasm32-wasip1` target
2. Export functions with C ABI
3. Import and call host functions
4. Handle JSON serialization/deserialization
5. Manage linear memory (strings/buffers)

**Supported**: Rust ✅, C ✅, Go (TinyGo) ✅, AssemblyScript ✅, Zig ✅, C# ✅

**Not Supported**: Standard Go (use TinyGo), Python (no WASI), Node.js (no WASI)

## Testing

All SDKs should pass the same test suite:

```bash
# Contract compliance
wasm-objdump -x driver.wasm | grep "export.*driver_"

# Integration test
go test ./v2/drivers/runtime/wasm/... -v

# Performance benchmark
go test -bench=. -benchmem
```

## Examples

Each SDK directory should include:
- `README.md` - Installation and usage
- `examples/` - Working examples
- `Makefile` or build script
- Tests validating contract compliance

## References

- **Multi-Language Guide**: [../../docs/LANGUAGE_SUPPORT.md](../../docs/LANGUAGE_SUPPORT.md)
- **Rust SDK**: [rust/app/](rust/app/) | [rust/driver/](rust/driver/)
- **Go SDK**: [go/app/](go/app/) | [go/driver/](go/driver/)
- **C SDK** (template): [c/driver/](c/driver/)
