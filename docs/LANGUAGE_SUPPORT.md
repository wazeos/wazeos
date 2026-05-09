# Multi-Language WASM Support

**Status**: Ready for implementation
**Current Languages**: Rust
**Architecture**: Language-agnostic WASM runtime

---

## Overview

WazeOS drivers and apps are compiled to WebAssembly, making them **inherently language-agnostic**. Any language that can compile to WASM and implement the required contract can be used.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│ Language (Rust, C, Go, AssemblyScript, etc.)           │
│ - Source code in any WASM-capable language              │
└──────────────────────┬──────────────────────────────────┘
                       │ Compile to wasm32-wasip1
                       ▼
┌─────────────────────────────────────────────────────────┐
│ WASM Binary (.wasm file)                                │
│ - Exports: driver_metadata, driver_init, driver_call    │
│ - Imports: host_iobus_call, host_iobus_create_handle   │
└──────────────────────┬──────────────────────────────────┘
                       │ Loaded by WASM runtime
                       ▼
┌─────────────────────────────────────────────────────────┐
│ WazeOS WASM Runtime (Go)                                │
│ - Language-agnostic loader using wazero                 │
│ - Provides host functions for I/O Bus access            │
└─────────────────────────────────────────────────────────┘
```

**Key Insight**: The WASM runtime only sees `.wasm` binaries, not source code. It doesn't know or care what language produced them.

---

## WASM Driver Contract

All WASM drivers must implement this C ABI contract, regardless of source language.

### Required Exports

```c
// Returns JSON string with driver metadata
// Format: {"name":"...", "version":"...", "class":"...", "uri_pattern":"...", "capabilities":[...]}
const char* driver_metadata();

// Initializes the driver with JSON config
// config_ptr: pointer to JSON string in WASM memory
// config_len: length of JSON string
// Returns: 0 on success, non-zero on error
uint32_t driver_init(uint32_t config_ptr, uint32_t config_len);

// Handles a request
// request_ptr: pointer to JSON request in WASM memory
// request_len: length of request JSON
// Returns: pointer to JSON response string in WASM memory
const char* driver_call(uint32_t request_ptr, uint32_t request_len);
```

### Available Imports

```c
// Call another driver through IO Bus
// request_ptr: pointer to JSON request
// request_len: length of request
// Returns: packed u64 (high 32 bits = response pointer, low 32 bits = length)
uint64_t host_iobus_call(uint32_t request_ptr, uint32_t request_len);

// Create a handle to another driver
// uri_ptr: pointer to URI string
// uri_len: length of URI
// Returns: packed u64 (high 32 bits = handle ID pointer, low 32 bits = length)
uint64_t host_iobus_create_handle(uint32_t uri_ptr, uint32_t uri_len);

// Close a handle
// uri_ptr: pointer to handle URI
// uri_len: length of URI
// Returns: 0 on success, non-zero on error
uint32_t host_iobus_close_handle(uint32_t uri_ptr, uint32_t uri_len);
```

### Data Format

- **All data exchange happens via JSON strings**
- Request format: `{"uri":"...", "operation":"...", "args":{...}, "headers":{...}, "body":"base64..."}`
- Response format: `{"status_code":200, "headers":{...}, "body":"base64...", "error":"..."}`

See [v2/drivers/runtime/wasm/loader.go:62-85](../../drivers/runtime/wasm/loader.go) for full specification.

---

## Language Requirements

To add support for a new language, you need:

### 1. WASM Compilation Target

The language must compile to `wasm32-wasip1` (WASI preview 1):
- ✅ **Rust** - via `cargo build --target wasm32-wasip1`
- ✅ **C/C++** - via `clang --target=wasm32-wasi`
- ✅ **Go** - via TinyGo (not standard Go)
- ✅ **AssemblyScript** - native WASM target
- ✅ **Zig** - via `zig build -Dtarget=wasm32-wasi`
- ✅ **C#** - via Blazor/Uno Platform
- ❌ **Python** - Pyodide doesn't support WASI well
- ❌ **JavaScript** - No direct WASI support

### 2. Foreign Function Interface (FFI)

The language must be able to:
- **Export functions** with C ABI (extern "C")
- **Import functions** from host environment
- **Work with raw pointers** and memory addresses
- **Control memory layout** (for passing strings/buffers)

### 3. JSON Support

The language needs:
- JSON serialization (struct → JSON string)
- JSON deserialization (JSON string → struct)
- Most languages have standard libraries or crates for this

### 4. Memory Management

Understanding of WASM linear memory:
- Strings/buffers must live in WASM memory
- Pointers are u32 offsets into linear memory
- Host can read WASM memory but not vice versa
- Need to manage lifetimes (especially for returned strings)

---

## Adding a New Language

### Step 1: Create SDK Package

Create `v2/core/sdk/<language>/wazeos-driver/`:

```
v2/core/sdk/
├── rust/              # ✅ Current
│   ├── wazeos-app/
│   └── wazeos-driver/
├── c/                 # Future
│   ├── wazeos-app/
│   └── wazeos-driver/
├── go/                # Future (TinyGo)
└── assemblyscript/    # Future
```

### Step 2: Implement Low-Level Bindings

Create a library that:
- Exports the required functions (`driver_metadata`, `driver_init`, `driver_call`)
- Imports host functions (`host_iobus_call`, etc.)
- Handles JSON serialization/deserialization
- Manages string/buffer memory

### Step 3: Create High-Level API

Provide an ergonomic API on top of low-level bindings:

**Rust SDK** (current):
```rust
pub trait Driver {
    fn metadata(&self) -> DriverMetadata;
    fn init(&mut self, config: HashMap<String, Value>) -> Result<(), String>;
    fn call(&mut self, req: Request) -> Result<Response, String>;
}

// Usage
register_driver!(MyDriver);
```

**Ideal C SDK**:
```c
typedef struct {
    char* name;
    char* version;
    // ...
} DriverMetadata;

typedef int (*InitFunc)(const char* config_json);
typedef char* (*CallFunc)(const char* request_json);

#define REGISTER_DRIVER(name, init_fn, call_fn) \
    /* macro that generates exports */
```

**Ideal Go SDK** (TinyGo):
```go
type Driver interface {
    Metadata() Metadata
    Init(config map[string]any) error
    Call(req Request) (Response, error)
}

func RegisterDriver(d Driver) {
    // Generates exports
}
```

### Step 4: Create Example Driver

Implement a simple driver (e.g., echo driver) to validate the SDK:

```
drivers/<language>-examples/
└── echo/
    ├── src/
    ├── build.sh
    └── README.md
```

### Step 5: Document Build Process

Create `v2/core/sdk/<language>/README.md` with:
- Installation instructions
- Build commands
- Common issues and solutions
- Examples

---

## Language-Specific Guides

### C/C++ (Ready to Implement)

**Strengths**:
- Excellent WASM support (clang)
- Direct memory control
- Existing native libraries can be compiled to WASM

**Challenges**:
- Manual memory management
- No built-in JSON (need library like cJSON)
- String handling requires care

**Example**:
```c
#include <stdint.h>
#include <string.h>
#include "cJSON.h"

// Import host functions
__attribute__((import_module("env"), import_name("host_iobus_call")))
extern uint64_t host_iobus_call(uint32_t ptr, uint32_t len);

// Export driver_metadata
__attribute__((export_name("driver_metadata")))
const char* driver_metadata() {
    static char metadata[512];
    cJSON *json = cJSON_CreateObject();
    cJSON_AddStringToObject(json, "name", "http-driver-c");
    cJSON_AddStringToObject(json, "version", "1.0.0");
    // ... more fields
    char* str = cJSON_PrintUnformatted(json);
    strncpy(metadata, str, sizeof(metadata));
    free(str);
    cJSON_Delete(json);
    return metadata;
}

// Export driver_init
__attribute__((export_name("driver_init")))
uint32_t driver_init(uint32_t config_ptr, uint32_t config_len) {
    // Parse config JSON and initialize
    return 0;
}

// Export driver_call
__attribute__((export_name("driver_call")))
const char* driver_call(uint32_t request_ptr, uint32_t request_len) {
    // Handle request and return response JSON
    static char response[4096];
    // ... implementation
    return response;
}
```

**Build**:
```bash
clang --target=wasm32-wasi \
      -O2 \
      -nostdlib \
      -Wl,--no-entry \
      -Wl,--export=driver_metadata \
      -Wl,--export=driver_init \
      -Wl,--export=driver_call \
      -Wl,--import-memory \
      -o driver.wasm \
      driver.c cJSON.c
```

### Go via TinyGo (Ready to Implement)

**Strengths**:
- Go developers can use familiar syntax
- Good WASM support via TinyGo
- Built-in JSON encoding

**Challenges**:
- TinyGo has some limitations vs regular Go
- Garbage collection in WASM
- Binary size can be larger

**Example**:
```go
package main

import (
    "encoding/json"
    "unsafe"
)

type Metadata struct {
    Name        string   `json:"name"`
    Version     string   `json:"version"`
    Class       string   `json:"class"`
    URIPattern  string   `json:"uri_pattern"`
    Capabilities []string `json:"capabilities"`
}

//export driver_metadata
func driver_metadata() *byte {
    meta := Metadata{
        Name:    "http-driver-go",
        Version: "1.0.0",
        Class:   "io.connect",
        URIPattern: "http://**",
        Capabilities: []string{"call"},
    }

    data, _ := json.Marshal(meta)
    return &data[0]
}

//export driver_init
func driver_init(configPtr, configLen uint32) uint32 {
    // Initialize driver
    return 0
}

//export driver_call
func driver_call(reqPtr, reqLen uint32) *byte {
    // Handle request
    response := []byte(`{"status_code":200}`)
    return &response[0]
}

func main() {}
```

**Build**:
```bash
tinygo build -o driver.wasm \
    -target=wasi \
    -no-debug \
    driver.go
```

### AssemblyScript (Ready to Implement)

**Strengths**:
- TypeScript-like syntax
- Designed specifically for WASM
- Excellent WASM tooling

**Challenges**:
- Smaller ecosystem than TypeScript
- Memory management is manual

**Example**:
```typescript
// driver.ts
import { JSON } from "json-as/assembly";

@external("env", "host_iobus_call")
declare function host_iobus_call(ptr: u32, len: u32): u64;

class Metadata {
    name!: string;
    version!: string;
    class!: string;
    uri_pattern!: string;
    capabilities!: string[];
}

export function driver_metadata(): string {
    const meta = new Metadata();
    meta.name = "http-driver-as";
    meta.version = "1.0.0";
    meta.class = "io.connect";
    meta.uri_pattern = "http://**";
    meta.capabilities = ["call"];

    return JSON.stringify(meta);
}

export function driver_init(config_ptr: u32, config_len: u32): u32 {
    // Initialize driver
    return 0;
}

export function driver_call(request_ptr: u32, request_len: u32): string {
    // Handle request
    return '{"status_code":200}';
}
```

**Build**:
```bash
asc driver.ts \
    --target release \
    --runtime stub \
    --exportRuntime \
    -o driver.wasm
```

---

## Testing Multi-Language Drivers

All drivers, regardless of language, must pass the same test suite:

### 1. Contract Compliance

```bash
# Verify WASM module exports required functions
wasm-objdump -x driver.wasm | grep -E "export.*driver_(metadata|init|call)"
```

### 2. Metadata Test

```go
// Load driver and verify metadata
driver, _ := iobus.NewWASMDriver(spec, bus)
metadata := driver.Metadata()
assert(metadata.Name != "")
assert(metadata.URIPattern != "")
```

### 3. Integration Test

```go
// Full request/response cycle
req := iobus.Request{
    URI: "test://echo",
    Operation: "call",
    Body: []byte("hello"),
}
resp, err := driver.Call(ctx, req)
assert(err == nil)
assert(resp.StatusCode == 200)
```

### 4. Performance Benchmark

```bash
# Compare performance across languages
go test -bench=. -benchmem
# Benchmark WASM driver load time
# Benchmark request/response throughput
```

---

## Migration Path

### Phase 1: Foundation (Current)
- ✅ Language-agnostic WASM runtime
- ✅ Rust SDK (reference implementation)
- ✅ Documentation of driver contract

### Phase 2: C/C++ Support
- [ ] Create C SDK (`v2/core/sdk/c/`)
- [ ] Port one driver to C (e.g., file driver)
- [ ] Document C-specific patterns

### Phase 3: Go Support
- [ ] Create TinyGo SDK (`v2/core/sdk/go/`)
- [ ] Port one driver to Go
- [ ] Performance comparison with Rust

### Phase 4: AssemblyScript Support
- [ ] Create AssemblyScript SDK
- [ ] Example driver
- [ ] Web-friendly documentation

### Phase 5: Additional Languages
- [ ] Community-driven: Zig, Swift, C#, etc.
- [ ] SDK template for new languages
- [ ] Automated testing harness

---

## Best Practices

### For SDK Authors

1. **Hide Complexity**: Wrap low-level FFI with ergonomic API
2. **Provide Macros**: Auto-generate boilerplate (like Rust's `register_driver!`)
3. **Handle Memory**: Manage string lifetimes automatically
4. **Include Examples**: Show common patterns (HTTP call, file I/O, etc.)
5. **Document Limitations**: Language-specific constraints

### For Driver Authors

1. **Start with Rust**: Best tooling and ecosystem
2. **Use C for Libraries**: Wrap existing native code
3. **Use Go for Familiarity**: If team knows Go
4. **Use AssemblyScript for Web**: If targeting browser too

### For Core Team

1. **Don't Leak Abstractions**: Keep WASM runtime language-agnostic
2. **Test All Languages**: Same test suite for all
3. **Document Contract**: Keep driver contract stable and clear
4. **Performance Parity**: All languages should have similar overhead

---

## References

- **WASM Contract**: [v2/drivers/runtime/wasm/loader.go](../../drivers/runtime/wasm/loader.go)
- **WASM Runtime**: [v2/drivers/runtime/wasm/driver.go](../../drivers/runtime/wasm/driver.go)
- **Rust SDK**: [v2/core/sdk/rust/driver/](../../core/sdk/rust/driver/)
- **Rust Example**: [drivers/file/src/lib.rs](../../../drivers/file/src/lib.rs)

---

**Next Steps**: Choose a second language to support and create its SDK following the steps above. C/C++ is recommended as it has the most mature WASM tooling.
