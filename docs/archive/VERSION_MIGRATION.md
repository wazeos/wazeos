# WazeOS Version Migration

This repository contains two major versions of WazeOS:

## v1/ - Original Implementation (ARCHIVED)

**Status**: Archived - Used for reference and learning

The v1 implementation was a proof-of-concept that validated the core ideas:
- URI-based resource abstraction
- Capability-based permissions
- MCP integration
- Multi-language SDKs

However, v1 had fundamental architectural limitations:
- ❌ JSON + base64 for large binary data (4/3x overhead)
- ❌ Stateless operations requiring repeated model uploads
- ❌ Hardcoded buffer limits (50MB)
- ❌ No streaming support
- ❌ WASM memory exhaustion with large data

### What We Learned

1. **Don't move large data through WASM** - Use handles/references instead
2. **Streaming > Buffering** - Never assume a buffer size is "enough"
3. **Stateful resources need sessions** - ML models, DB connections, etc.
4. **Clear taxonomy matters** - Driver classes help developers understand the system

## v2/ - Redesigned Architecture (ACTIVE)

**Status**: 🚧 In Development (Alpha)

v2 is a complete redesign incorporating all v1 learnings:

### Core Improvements

| Problem (v1) | Solution (v2) |
|--------------|---------------|
| Large binary data in JSON | Binary streaming with MessagePack |
| Stateless model loading | Handle-based sessions |
| Hardcoded buffers | Streaming eliminates limits |
| WASM memory limits | Handles keep data in kernel |
| Unclear driver model | 4 driver classes (io.connect, io.listen, runtime.*, kernel.*) |

### Architecture Highlights

```
┌─────────────────────────────────────┐
│       IO Bus (Kernel)               │
│  - Trie router (O(log n))           │
│  - Session manager (handles)        │
│  - Security (authz)                 │
└─────────────────────────────────────┘
         ↓         ↓          ↓
    io.connect  io.listen  runtime.*
```

### Example: The Problem v2 Solves

**v1 Code** (sending 20MB model 100 times):
```rust
// ❌ Inefficient: 2GB total data transfer
for audio_file in audio_files {
    let result = ctx.io("onnx://execute").call(json!({
        "model_bytes": encoder_model,  // 20MB every time!
        "inputs": load_audio(audio_file)
    }))?;
}
```

**v2 Code** (load once, reference many times):
```rust
// ✅ Efficient: 21MB total data transfer
let model = ctx.load("onnx://load")
    .with_source("hf://microsoft/whisper-tiny/encoder.onnx")
    .create()?;

for audio_file in audio_files {
    let result = model.call(json!({
        "inputs": load_audio(audio_file)  // Only input data
    }))?;
}
```

**Improvement**: 100x reduction in data transfer, constant WASM memory

## Directory Structure

```
/Users/dcoady/dev/os/
├── v1/                         # Original implementation (archived)
│   ├── sdk/                    # v1 SDKs (Go, Rust)
│   ├── internal/               # v1 kernel
│   ├── drivers/                # v1 drivers
│   ├── cmd/                    # v1 CLI
│   └── transformers/           # v1 example apps
│
├── v2/                         # New implementation (active)
│   ├── docs/                   # Design docs (PRD, RFCs)
│   ├── kernel/                 # Core IO Bus
│   ├── drivers/                # Driver implementations
│   ├── sdk/                    # SDKs (Rust, Go)
│   └── examples/               # Example apps
│
├── VERSION_MIGRATION.md        # This file
└── README.md                   # Root README
```

## Which Version Should I Use?

- **For new projects**: Use v2 (active development)
- **For learning**: Read v1 code, understand the problems, then see how v2 solves them
- **For production**: Wait for v2 beta (Q3 2026)

## Migration Path (v1 → v2)

If you built something on v1, here's the migration strategy:

### 1. Identify Stateful Operations

Look for repeated operations on the same resource:
```rust
// v1: Loading model repeatedly
for i in 0..100 {
    ctx.io("onnx://").call({model: bytes, input: data})
}
```

Convert to handles:
```rust
// v2: Load once, use many times
let model = ctx.load("onnx://").with_source(...).create()?;
for i in 0..100 {
    model.call({input: data})
}
```

### 2. Update Driver Registration

v1 used manual registration:
```go
// v1
driver := &MyDriver{}
kernel.RegisterDriver("my-pattern", driver)
```

v2 uses init() with metadata:
```go
// v2
func init() {
    iobus.Register(iobus.DriverSpec{
        Name: "my-driver",
        Class: iobus.ConnectDriver,
        URIPattern: "myscheme://**",
        Capabilities: []iobus.Capability{iobus.CapCall},
        Factory: func() iobus.Driver { return &MyDriver{} },
    })
}
```

### 3. Adopt Streaming for Large Files

v1 required loading entire file:
```rust
// v1
let data = ctx.io("file:///large.bin").call()?;
process(data);  // 1GB in memory!
```

v2 supports streaming:
```rust
// v2
let stream = ctx.io("file:///large.bin").stream_read()?;
process_stream(stream);  // Constant memory
```

### 4. Update SDK Imports

```rust
// v1
use wazeos_app::{Context, run_mcp_tool};

// v2
use wazeos_sdk::{Context, run_mcp_tool, Handle};
```

## Timeline

| Phase | Status | Completion |
|-------|--------|------------|
| v1 Development | ✅ Complete | 100% |
| v1 Post-Mortem | ✅ Complete | 100% |
| v2 Design (PRD, RFCs) | ✅ Complete | 100% |
| v2 Core Kernel | ✅ Complete | 100% |
| v2 Drivers | 🚧 In Progress | 25% |
| v2 SDKs | 📝 TODO | 0% |
| v2 Alpha Release | 📝 Q2 2026 | - |
| v2 Beta Release | 📝 Q3 2026 | - |
| v2 Stable Release | 📝 Q4 2026 | - |

## Documentation

- **v1**: See [v1/README.md](v1/README.md) (if exists) or archived docs
- **v2**: See [v2/README.md](v2/README.md) and [v2/docs/](v2/docs/)

## Questions?

- **Design questions**: Read [v2/docs/PRD.md](v2/docs/PRD.md)
- **Technical details**: Read RFCs in [v2/docs/](v2/docs/)
- **Contributing**: See [CONTRIBUTING.md](CONTRIBUTING.md)
- **Issues**: File on GitHub

---

**Key Takeaway**: v2 is not just an incremental improvement—it's a fundamental redesign that solves v1's architectural limitations while keeping what worked (URI abstraction, capability security, MCP integration).
