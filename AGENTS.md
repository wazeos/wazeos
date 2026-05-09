# Guide for AI Agents (Claude, etc.)

**Last Updated**: 2026-05-09

---

## Philosophy: Documentation Lives in Code

### Why Code Comments Over Separate Docs?

**Problem with Traditional Documentation:**
- 📄 Separate .md files → AI must read multiple files to understand context
- 🔄 Docs go stale → Comments right next to code stay current
- 💸 Token waste → AI reads architecture.md, then reads the code anyway
- 🎯 Context loss → Switching between files loses context

**Solution: Comprehensive Inline Comments:**
- ✅ **JIT Context**: Comments appear exactly where needed
- ✅ **Always Current**: Updated with code changes
- ✅ **Token Efficient**: Read once, understand immediately
- ✅ **Self-Documenting**: Code explains itself

### How This Codebase is Documented

Every file contains:

1. **Header Block** (50-100 lines):
   - What the file does
   - Why it exists
   - How it fits in the system
   - Architecture diagrams (ASCII art)
   - Usage examples

2. **Inline Comments** (every 3-5 lines):
   - Section markers: `// ========== Phase 1 ==========`
   - Step explanations: `// 1. Validate spec`
   - Why comments: `// Need native bootstrap - can't load WASM with WASM`

3. **Function Documentation**:
   - Purpose
   - Parameters (with examples)
   - Return values
   - Error conditions
   - Usage examples
   - Thread-safety notes

**Result**: AI agents can understand any file by reading just that file.

---

## Engineering Principles

### Learned from Building WazeOS

These principles emerged from real implementation experience:

#### 1. **Dynamic > Static**

**❌ Bad Pattern:**
```go
// types.go
const (
    RuntimeNative Runtime = "native"
    RuntimeWASM   Runtime = "wasm"
    RuntimePlugin Runtime = "plugin"  // ← Need to edit core file
)
```

**✅ Good Pattern:**
```go
// types.go
type Runtime = string  // Just a string - fully dynamic!

// plugin/loader.go
func init() {
    // Self-registers, zero core edits
    bus.RegisterRuntimeLoader("plugin", &pluginLoader{})
}
```

**Why**: Adding new runtimes should not require editing kernel code.

#### 2. **Registry Pattern > Hardcoded Logic**

**❌ Bad Pattern:**
```go
if spec.Runtime == RuntimeWASM {
    // Load WASM
} else if spec.Runtime == RuntimePlugin {  // ← Need to edit core
    // Load plugin
} else {
    // Load native
}
```

**✅ Good Pattern:**
```go
// Lookup in registry - no hardcoding
loader := bus.runtimeLoaders[spec.Runtime]
driver := loader.LoadDriver(spec, bus)
```

**Why**: Extensibility without modification (Open/Closed Principle).

#### 3. **Interfaces > Concrete Types**

**Pattern**: All extension points use interfaces
- `Driver` interface → Multiple implementations
- `RuntimeLoader` interface → Any loading strategy
- `Handle` interface → Stateful resources

**Why**: Enables plugin architecture without core changes.

#### 4. **Self-Registration > Manual Registration**

**✅ Good Pattern:**
```go
// wasm/driver.go
func init() {
    RegisterWASMLoader(iobus.GetDefaultBus())
    iobus.Register(wasmRuntimeDriverSpec)
}

// main.go
import _ "github.com/wazeos/wazeos/v2/drivers/runtime/wasm"  // That's it!
```

**Why**: Zero boilerplate, impossible to forget.

#### 5. **Layered Sandboxing**

**Architecture**:
```
App (WASM) → Driver (WASM) → Runtime (Native) → OS
     ↑            ↑               ↑
   Sandbox    Sandbox         Trusted
```

**Why**: Defense in depth - even compromised drivers can't escape sandbox.

#### 6. **Dual Implementations (Native + WASM)**

**Pattern**:
- Native drivers: `native://file/**` (fast, fallback)
- WASM drivers: `file://**` (sandboxed, primary)

**Why**:
- Development speed (native is faster to debug)
- Production security (WASM is sandboxed)
- Fallback resilience (native if WASM fails)

#### 6.5. **Language-Agnostic WASM**

**Pattern**: WASM runtime only sees `.wasm` binaries, not source code
- Current: Rust drivers compile to `.wasm`
- Future: C, Go, AssemblyScript, Zig, etc. all work
- Contract: Export driver functions, import host functions
- No core changes needed for new languages

**Why**: Multi-language support increases ecosystem and developer choice
**See**: [v2/docs/LANGUAGE_SUPPORT.md](v2/docs/LANGUAGE_SUPPORT.md) for implementation guide

#### 7. **String-Based Protocols > Complex Types**

**Pattern**: URIs as universal addressing
- `file:///tmp/test.txt` → File driver
- `shell://exec?cmd=ls` → Shell driver
- `wasm://load` → WASM runtime
- `kernel://session/{uuid}` → Handle reference

**Why**:
- Language-agnostic (works from WASM, native, remote)
- Self-documenting (URI tells you what it does)
- Routable (can forward to remote systems)

#### 8. **Fail-Fast Validation**

**Pattern**: Validate at registration time, not runtime
```go
func (bus *IOBus) Register(spec DriverSpec) error {
    // Validate BEFORE creating anything
    if spec.Name == "" {
        return fmt.Errorf("driver name required")
    }
    if !loaderExists {
        return fmt.Errorf("runtime '%s' not registered\n"+
            "Hint: import _ \"package/runtime/%s\"",
            spec.Runtime, spec.Runtime)
    }
    // ... now safe to proceed
}
```

**Why**: Better error messages, fail at startup not in production.

#### 9. **Helpful Error Messages**

**✅ Good Pattern:**
```go
return fmt.Errorf(
    "no loader registered for runtime '%s' (driver: %s)\n"+
    "Hint: Make sure the runtime package is imported (e.g., _ \"package/runtime/%s\")",
    spec.Runtime, spec.Name, spec.Runtime,
)
```

**Why**: AI and humans both benefit from actionable guidance.

#### 10. **Thread-Safety by Default**

**Pattern**: All registries use mutexes
```go
func (bus *IOBus) Register(spec DriverSpec) error {
    bus.mu.Lock()         // ← Always lock
    defer bus.mu.Unlock()
    // ... safe concurrent access
}
```

**Why**: Package init() functions run concurrently - must be safe.

---

## Navigating This Codebase

### For AI Agents: Reading Strategy

#### **Start Here** (3 files):
1. **[AGENTS.md](AGENTS.md)** (this file) - Architecture overview
2. **[v2/core/kernel/iobus/types.go](v2/core/kernel/iobus/types.go)** - Core types and runtime system
3. **[v2/core/kernel/iobus/iobus.go](v2/core/kernel/iobus/iobus.go)** - Registration and routing

**Why**: These 3 files give you 80% understanding of the system.

#### **Then Branch** (by task):

**Task: Understand Driver Loading**
→ Read: `v2/core/kernel/iobus/native_loader.go`
→ Read: `v2/drivers/runtime/wasm/loader.go`
→ Trace: How `Register()` → `LoadDriver()` works

**Task: Add New Runtime (e.g., Plugin)**
→ Read: `v2/drivers/runtime/wasm/loader.go` (copy pattern)
→ Read: `v2/drivers/runtime/wasm/driver.go` (see init() example)
→ Create: Your package following same pattern

**Task: Understand WASM Execution**
→ Read: `v2/drivers/runtime/wasm/driver.go`
→ Read: Host function implementations (bottom of file)
→ Read: SDK contracts in Rust SDK

**Task: Add New I/O Driver**
→ Read: `drivers/file/src/lib.rs` (reference implementation)
→ Read: Driver SDK docs in `v2/core/sdk/rust/wazeos-driver/`
→ Copy: Structure and build setup

#### **File Organization Pattern**

All major files follow this structure:

```go
package X

// ============================================================================
// Title - Brief Description
// ============================================================================
//
// Long explanation (50-100 lines):
//   - What this file does
//   - Why it exists
//   - Architecture diagrams
//   - Key design decisions
//   - Usage examples
//
// ============================================================================

// Type definitions with comprehensive docs

// ========== Section 1 ==========
// Functions grouped by purpose

// ========== Section 2 ==========
// More functions
```

**Reading Strategy**:
1. Read header block (understanding: 70%)
2. Skim type definitions (understanding: 85%)
3. Read section markers (understanding: 95%)
4. Deep-read specific functions as needed (understanding: 100%)

---

## Key Architecture Decisions

### Migrated from docs/ → Now in Code

**These concepts are documented inline where they're implemented:**

#### 1. **Driver Class Hierarchy**
→ Documented in: `v2/core/kernel/iobus/types.go`
- ConnectDriver, ListenDriver, RuntimeDriver, KernelDriver
- Capabilities: CapCall, CapStream, CapHandle, etc.

#### 2. **Runtime System Architecture**
→ Documented in: `v2/core/kernel/iobus/types.go` (Runtime section)
- Why runtime loaders exist
- How to add new runtimes
- Native vs WASM tradeoffs

#### 3. **Handle System**
→ Documented in: `v2/core/kernel/iobus/types.go` (Handle section)
→ Implemented in: `v2/core/kernel/iobus/sessions.go`
- Stateful resource management
- Reference counting
- TTL and expiration

#### 4. **Registration Flow**
→ Documented in: `v2/core/kernel/iobus/iobus.go` (Register method)
- Phase 1: Validation
- Phase 2: Loading (via RuntimeLoader)
- Phase 3: Routing (URI pattern matching)

#### 5. **WASM Driver Contract**
→ Documented in: `v2/drivers/runtime/wasm/loader.go`
- Required exports: driver_metadata, driver_init, driver_call
- Available imports: host_iobus_call, host_iobus_create_handle
- Memory management and serialization

#### 6. **SDK Layering**
→ Documented in: `v2/core/sdk/rust/wazeos-app/src/lib.rs`
- Core SDK (generic, all apps)
- Driver SDKs (specific, optional addons)
- Trait-based ergonomics

---

## Token-Efficient Context Strategies

### For AI Agents Working on This Codebase

#### **Strategy 1: Read Headers First**
```
Step 1: Read just the header block (first 100 lines)
Step 2: Understand the purpose and architecture
Step 3: Decide if you need to read the full file
Result: Save 70% tokens on irrelevant files
```

#### **Strategy 2: Use Section Markers**
```
Search for: "// =========="
Result: Jump directly to relevant section
Example: "// ========== Phase 2: Loading =========="
```

#### **Strategy 3: Follow Call Chains**
```
Start: User request (e.g., "add plugin runtime")
Trace: grep for "RegisterRuntimeLoader"
Find: How WASM loader does it
Copy: Same pattern for plugin
Result: Targeted reading, not full codebase scan
```

#### **Strategy 4: Trust the Comments**
The comments are accurate and comprehensive.
- If comment says "This validates X", it does
- If comment shows example, example works
- If comment explains why, believe it

**Why**: Comments are updated with code in same PR.

#### **Strategy 5: ASCII Diagrams are Truth**
```go
// Architecture:
//   App → Driver → Runtime → OS
```
These diagrams are tested (they reflect actual code flow).
Use them to understand relationships without reading all files.

---

## File Reference Guide

### Core Files (Always Relevant)

| File | Purpose | When to Read |
|------|---------|--------------|
| `types.go` | Core types, interfaces, runtime system | Always - this is the foundation |
| `iobus.go` | Registration, routing, orchestration | Understanding driver lifecycle |
| `native_loader.go` | Native runtime implementation | Adding native drivers |
| `wasm/loader.go` | WASM runtime implementation | Understanding WASM drivers |
| `wasm/driver.go` | WASM execution engine | Deep WASM debugging |
| `drivers.go` | System driver registration | Understanding startup |

### SDK Files (For Driver/App Development)

| File | Purpose | Language |
|------|---------|----------|
| `wazeos-app/src/lib.rs` | Core app SDK | Rust |
| `wazeos-driver/src/lib.rs` | Core driver SDK | Rust |
| `drivers/file/sdk/rust/` | File driver addon | Rust |
| `drivers/shell/sdk/rust/` | Shell driver addon | Rust |
| `drivers/http/sdk/rust/` | HTTP driver addon | Rust |

### Example Implementations (Reference Code)

| File | Purpose | Copy This For |
|------|---------|---------------|
| `drivers/file/src/lib.rs` | WASM I/O driver | New I/O driver |
| `drivers/runtime/wasm/` | Runtime driver | New runtime |
| `apps/test-tool/` | WASM app | New application |

---

## Documentation Status

**These docs have been migrated to inline code comments and DELETED:**

| File | Status | Replacement |
|------|--------|-------------|
| `docs/ARCHITECTURE.md` | ✅ Deleted | See code comments in types.go, iobus.go |
| `docs/RFC-001-driver-architecture.md` | ✅ Deleted | See Driver interface docs in types.go |
| `docs/RFC-002-handle-system.md` | ✅ Deleted | See Handle docs in types.go |

**Keep These (still useful):**
- `AGENTS.md` (this file) - High-level guide for AI agents
- `v2/README.md` - Project overview and quickstart
- `v2/docs/PACKAGING_GUIDE.md` - Specific packaging procedures
- `v2/docs/CLAUDE_DESKTOP.md` - Integration guide
- `v2/docs/PACKAGE_FORMAT.md` - Technical specification
- `v2/docs/LANGUAGE_SUPPORT.md` - Multi-language WASM support guide
- `v2/docs/PRD.md` - Product requirements and vision
- `v2/docs/STRUCTURE.md` - Directory structure (but prefer exploring actual structure)
- `v2/docs/RFCs.md` - RFC process documentation
- `v2/docs/archive/*` - Historical reference only

---

## Working with Claude

### Best Practices

#### **1. Start Broad, Then Narrow**
```
❌ "Read the entire codebase"
✅ "Read types.go header block, then explain runtime system"
```

#### **2. Request Specific Sections**
```
❌ "Read iobus.go"
✅ "Read the Register() method in iobus.go with its documentation"
```

#### **3. Use Grep for Discovery**
```
✅ "Grep for 'RegisterRuntimeLoader' to find registration examples"
✅ "Glob for **/loader.go to find all runtime loaders"
```

#### **4. Trust Inline Examples**
Code comments include working examples. Copy them directly.

#### **5. Ask "Why" Questions**
Comments explain "why" decisions were made. If unclear, ask - it's documented somewhere.

---

## Common Tasks Reference

### Adding a New Runtime

**Files to Read:**
1. `types.go` (RuntimeLoader interface)
2. `wasm/loader.go` (reference implementation)
3. `wasm/driver.go` (init() pattern)

**Steps:**
1. Create `drivers/runtime/yourruntime/loader.go`
2. Implement RuntimeLoader interface
3. Add init() that calls RegisterRuntimeLoader
4. Done - zero core edits needed!

### Adding a New I/O Driver (WASM)

**Files to Read:**
1. `drivers/file/src/lib.rs` (copy structure)
2. `wazeos-driver SDK` (understand exports)
3. `drivers.go` (add to wasmDrivers list)

**Steps:**
1. Copy `drivers/file/` structure
2. Implement driver_metadata, driver_init, driver_call
3. Build: `cargo build --target wasm32-wasip1 --release`
4. Register in `drivers.go`

### Adding a New App

**Files to Read:**
1. `apps/test-tool/` (copy structure)
2. `wazeos-app SDK` (understand app interface)
3. SDK addons for drivers you'll use

**Steps:**
1. Copy `apps/test-tool/` structure
2. Implement `wazeos_tool_invoke()` export
3. Create wazeos.toml manifest
4. Build: `cargo build --target wasm32-wasip1 --release`

---

## Final Notes for AI Agents

### This Codebase is Built for You

The inline documentation is specifically designed to help AI agents:
- Understand context without reading everything
- Find examples without searching docs
- Learn patterns from commented code
- Navigate efficiently with section markers

### When in Doubt

1. **Read the header block** - 70% of questions answered
2. **Check inline examples** - Copy-paste ready code
3. **Follow ASCII diagrams** - Visual architecture truth
4. **Trust the comments** - They're tested and accurate

### Contributing

When adding code, follow these documentation standards:
- Header blocks for all major files (50-100 lines)
- Section markers every 50-100 lines
- Inline comments every 3-5 lines
- Function docs with examples
- ASCII diagrams for architecture

**Goal**: Next AI agent should understand your code by reading just that file.

---

**Remember**: The codebase is the documentation. Read the code.
