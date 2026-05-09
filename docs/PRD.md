# WazeOS v2: Product Requirements Document

**Version**: 2.0.0
**Status**: Draft
**Last Updated**: 2026-05-08
**Authors**: WazeOS Core Team

---

## Executive Summary

WazeOS v2 is a capability-based resource abstraction layer designed for AI agents, automation tools, and secure multi-tenant applications. Building on v1's learnings, v2 introduces **handle-based sessions**, **binary streaming**, and a **hierarchical driver model** to solve fundamental limitations around large data handling, stateful operations, and runtime complexity.

### Key Improvements over v1

| Problem (v1) | Solution (v2) |
|--------------|---------------|
| 20MB models sent with every inference request | Handle-based sessions: load once, reference many times |
| Base64 encoding overhead (4/3x size increase) | Binary protocol with streaming support |
| Hardcoded buffer limits (50MB Go, 30MB Rust) | Streaming eliminates buffer size limits |
| WASM memory exhaustion with large data | Handles keep data in kernel memory |
| Confusing driver vs app distinction | Unified component model with driver classes |
| JSON serialization mismatches | Protocol negotiation (MessagePack, Cap'n Proto) |
| No state between requests | Session management with automatic cleanup |

---

## Vision & Goals

### Vision
Enable AI agents to orchestrate complex workflows across heterogeneous resources (files, APIs, databases, ML models) with zero-trust security and language-agnostic extensibility.

### Goals
1. **Security**: Capability-based access control with URI pattern permissions
2. **Performance**: Handle 10GB+ datasets and 1B+ parameter models efficiently
3. **Extensibility**: Third-party drivers in any language (Rust, Go, Python, JS)
4. **Simplicity**: 10-line SDK examples for common patterns
5. **MCP-Native**: Model Context Protocol as first-class citizen
6. **Agent-Native**: CLI and APIs designed for both humans and AI coding agents

---

## RFC Process for Change Management

**Requirement**: All significant architectural, design, or implementation changes **MUST** be documented in an RFC (Request for Comments) before implementation.

### What Requires an RFC?

An RFC is required for:

1. **Architectural Changes**
   - New driver classes or capabilities
   - Changes to the IO Bus routing
   - Modifications to the handle system
   - Protocol changes (JSON → MessagePack, etc.)

2. **API Changes**
   - New CLI commands or flags
   - Changes to SDK interfaces
   - Breaking changes to existing APIs
   - New driver or app interfaces

3. **Security Changes**
   - Permission model modifications
   - Authentication/authorization changes
   - New security boundaries
   - Audit logging changes

4. **Performance Changes**
   - Changes that affect memory usage
   - Modifications to streaming behavior
   - Changes to caching strategies
   - Concurrency model changes

### What Doesn't Require an RFC?

- Bug fixes that don't change behavior
- Documentation updates
- Test additions
- Internal refactoring (same external behavior)
- Performance optimizations (same interface)

### RFC Structure

Each RFC must include:

```markdown
# RFC-XXX: [Title]

**Status**: Draft | Under Review | Accepted | Rejected | Implemented
**Author**: [Name]
**Created**: [Date]
**Updated**: [Date]

## Abstract
[1-2 paragraph summary of the change]

## Motivation
**Problem**: What problem are we solving?
**Why now**: Why is this change necessary?
**Impact**: Who is affected by this change?

## Proposed Solution
**Design**: How will we solve this problem?
**Alternatives considered**: What other approaches did we evaluate?
**Why this approach**: Why is this the best solution?

## Detailed Design
[Technical implementation details]
- Architecture diagrams
- Code examples
- API specifications
- Performance considerations

## Migration Path
**Breaking changes**: What breaks?
**Migration steps**: How do users migrate?
**Deprecation timeline**: When will old behavior be removed?

## Trade-offs
**Pros**: Benefits of this approach
**Cons**: Drawbacks and limitations
**Risks**: What could go wrong?

## Open Questions
[Unresolved issues for discussion]

## Decision
**Status**: [Why was this accepted/rejected?]
**Date**: [When was decision made?]
**Reviewers**: [Who reviewed and approved?]
```

### RFC Numbering

RFCs are numbered sequentially:
- **RFC-001**: Driver Architecture (✅ Complete)
- **RFC-002**: Handle System (✅ Complete)
- **RFC-003**: Binary Protocol (📝 TODO)
- **RFC-004**: Security Model (📝 TODO)
- **RFC-005**: CLI Tooling (📝 TODO)
- **RFC-XXX**: [Future RFCs]

### Review Process

1. **Draft**: Author creates RFC in `docs/` directory
2. **Under Review**: RFC is shared for feedback
   - Minimum 3 days for review
   - Address comments and update RFC
3. **Accepted**: RFC is approved for implementation
   - Document decision rationale
   - Assign implementation owner
4. **Implemented**: Change is completed
   - Update RFC with "Implemented" status
   - Link to PR/commit
5. **Rejected**: If not moving forward
   - Document why rejected
   - Preserve for future reference

### RFC Templates

**Quick RFC** (for smaller changes):
```bash
$ wazeos rfc new "Add --quiet flag to CLI" --quick
Created: docs/RFC-042-quiet-flag.md

# Quick template includes:
- Problem statement
- Proposed solution
- Breaking changes (if any)
```

**Full RFC** (for major changes):
```bash
$ wazeos rfc new "Distributed Handle System" --full
Created: docs/RFC-043-distributed-handles.md

# Full template includes all sections
```

### Example: RFC Decision Reasoning

**RFC-002: Handle System** was accepted because:

✅ **Problem**: v1 sent 20MB models with every inference (2GB for 100 requests)
✅ **Solution**: Handles keep data in kernel, apps reference by ID
✅ **Evidence**: 100x reduction in data transfer, 95% memory savings
✅ **Trade-offs**: Added complexity (session management, GC)
✅ **Alternatives**: Considered caching (rejected: no explicit control), global model store (rejected: security concerns)
✅ **Decision**: Accepted - benefits outweigh complexity

### Living Documentation

RFCs are **living documents**:
- Update when implementation reveals new insights
- Add "Lessons Learned" section after implementation
- Reference RFCs in code comments: `// See RFC-002 for handle design rationale`

### RFC Index

Maintain `docs/RFCs.md` with:
```markdown
# WazeOS v2 RFCs

## Accepted & Implemented
- [RFC-001: Driver Architecture](./RFC-001-driver-architecture.md) - ✅ Implemented
- [RFC-002: Handle System](./RFC-002-handle-system.md) - ✅ Implemented

## Under Review
- [RFC-005: CLI Tooling](./RFC-005-cli-tooling.md) - 📝 Under Review

## Rejected
- [RFC-042: Global Model Cache](./RFC-042-global-model-cache.md) - ❌ Rejected (security concerns)
```

### Why This Matters

**For the project**:
- 📚 **Historical record**: Understand why decisions were made
- 🎯 **Clear rationale**: Everyone knows the "why" behind changes
- 🔍 **Review process**: Catch issues before implementation
- 🚀 **Faster onboarding**: New contributors read RFCs to understand design

**For AI agents**:
- 🤖 **Context**: Agents can read RFCs to understand system design
- 🧠 **Reasoning**: Agents learn from documented trade-offs
- 📊 **Consistency**: Agents follow established patterns from RFCs
- 🔗 **References**: Agents cite RFCs when suggesting changes

**Example Agent Workflow**:
```
User: "Should we add a distributed handle system?"

Agent: Let me check existing RFCs...
[Reads RFC-002: Handle System]
[Sees "Alternatives Considered" section]
[Finds "Distributed handles" was considered but rejected]

Agent: "Actually, distributed handles were already considered in
RFC-002 and rejected due to security concerns around handle sharing
across trust boundaries. The current design keeps handles scoped to
a single session owner. If you need distributed access, RFC-002
suggests using explicit handle transfer with permission delegation."
```

---

## Agent-Native Developer Experience

WazeOS v2 is designed from the ground up to be equally usable by human developers and AI coding agents (like Claude, GPT-4, etc.). This principle informs every aspect of the developer tooling.

### Design Principles

1. **Discoverable**: `wazeos help` reveals all commands
2. **Consistent**: All commands follow `wazeos <noun> <verb> [args]` pattern
3. **Structured Output**: JSON output mode for agent parsing (`--json`)
4. **Self-Documenting**: Commands include examples and descriptions
5. **Idempotent**: Safe to run commands multiple times
6. **Error-Friendly**: Clear error messages with actionable suggestions

### CLI Command Structure

```bash
# Discovery
wazeos help                          # List all commands
wazeos driver help                   # List driver commands
wazeos driver new --help             # Get detailed help

# Structured output for agents
wazeos driver list --json            # Machine-readable output
wazeos app invoke tool --json        # Parse results easily

# Consistency
wazeos driver new <name>             # Create new driver
wazeos driver build <name>           # Build driver
wazeos driver install <path>         # Install driver

wazeos app new <name>                # Create new app
wazeos app build <name>              # Build app
wazeos app install <path>            # Install app
```

### For Human Developers

```bash
# Interactive prompts
$ wazeos driver new my-s3-driver
? Choose driver class: (Use arrow keys)
  ❯ io.connect - Client connector
    io.listen - Server listener
    runtime.* - Execution runtime
    kernel.* - Kernel plugin

? Select language:
  ❯ Go (native)
    Rust (WASM)

✓ Created driver scaffold at drivers/my-s3-driver/
→ Next steps:
  1. cd drivers/my-s3-driver
  2. Edit driver.go to implement your logic
  3. wazeos driver build my-s3-driver
  4. wazeos driver install
```

### For AI Coding Agents

```bash
# Non-interactive, structured output
$ wazeos driver new my-s3-driver \
    --class io.connect \
    --language go \
    --json

{
  "status": "success",
  "driver": {
    "name": "my-s3-driver",
    "path": "drivers/my-s3-driver",
    "class": "io.connect",
    "language": "go"
  },
  "files_created": [
    "drivers/my-s3-driver/driver.go",
    "drivers/my-s3-driver/driver_test.go",
    "drivers/my-s3-driver/README.md"
  ],
  "next_steps": [
    "cd drivers/my-s3-driver",
    "Edit driver.go to implement Call() method",
    "wazeos driver build my-s3-driver",
    "wazeos driver install drivers/my-s3-driver/build/my-s3-driver.so"
  ]
}
```

### Agent-Friendly Features

1. **Exit Codes**: 0 = success, non-zero = failure
2. **JSON Output**: All commands support `--json` flag
3. **Validation**: Input validation with clear error messages
4. **Idempotency**: Running same command twice is safe
5. **Progress**: Optional `--quiet` flag for scripting
6. **Documentation**: Embedded in CLI (`--help` on any command)

---

## Core Architecture

### 1. IO Bus

The **IO Bus** is the central routing layer that dispatches resource calls to registered drivers based on URI patterns.

```
┌─────────────────────────────────────────────────────────┐
│                        IO Bus                           │
│  ┌───────────────────────────────────────────────────┐ │
│  │  URI Pattern Matcher                              │ │
│  │  - Trie-based routing                             │ │
│  │  - Longest prefix match                           │ │
│  │  - Wildcard support (*, **)                       │ │
│  └───────────────────────────────────────────────────┘ │
│  ┌───────────────────────────────────────────────────┐ │
│  │  Security Layer                                   │ │
│  │  - Permission checks                              │ │
│  │  - Audit logging                                  │ │
│  │  - Rate limiting                                  │ │
│  └───────────────────────────────────────────────────┘ │
│  ┌───────────────────────────────────────────────────┐ │
│  │  Session Manager                                  │ │
│  │  - Handle lifecycle                               │ │
│  │  - Reference counting                             │ │
│  │  - Automatic cleanup                              │ │
│  └───────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
         ↓              ↓              ↓
    ┌─────────┐   ┌─────────┐   ┌──────────┐
    │ Connect │   │ Listen  │   │ Runtime  │
    │ Drivers │   │ Drivers │   │ Drivers  │
    └─────────┘   └─────────┘   └──────────┘
```

### 2. Driver Class Hierarchy

Drivers are organized into four main classes, each with specific capabilities and lifecycle:

#### **io.connect** - Client Connectors
Outbound connections to resources (files, APIs, databases).

| URI Pattern | Driver | Capabilities |
|-------------|--------|--------------|
| `file://**` | Local filesystem | read, write, stream |
| `http://**`, `https://**` | HTTP client | call, stream |
| `s3://**` | AWS S3 | read, write, stream |
| `hf://**` | HuggingFace Hub | read, stream |
| `pg://**` | PostgreSQL | call, stream, txn |
| `redis://**` | Redis | call, pubsub |

**Lifecycle**: Stateless by default, optionally stateful (connections, sessions)

#### **io.listen** - Server Listeners
Inbound connections for serving requests (HTTP servers, stdio).

| URI Pattern | Driver | Capabilities |
|-------------|--------|--------------|
| `http://127.0.0.1:*/mcp` | HTTP MCP server | listen |
| `stdio://*/mcp` | Stdio MCP server | listen |
| `ws://127.0.0.1:*/events` | WebSocket server | listen, pubsub |

**Lifecycle**: Long-running, starts on kernel boot, stops on shutdown

#### **runtime.\*** - Execution Runtimes
Execute code and models in isolated environments.

| URI Pattern | Driver | Capabilities |
|-------------|--------|--------------|
| `kernel://runtime/wasm/{uuid}` | WASM executor | exec, stream |
| `kernel://runtime/onnx/{uuid}` | ONNX inference | handle, stream |
| `kernel://runtime/python/{uuid}` | Python sandbox | exec, stream |
| `kernel://runtime/llm/{uuid}` | LLM inference | handle, stream |

**Lifecycle**: Handle-based, creates session on load, automatic cleanup

#### **kernel.\*** - Kernel Plugins
Core kernel functionality and security.

| URI Pattern | Driver | Capabilities |
|-------------|--------|--------------|
| `kernel://security/audit` | Audit logging | call |
| `kernel://security/credentials` | Secret management | call |
| `kernel://security/authz` | Authorization | call |
| `kernel://metrics` | Observability | call |

**Lifecycle**: Singleton, initialized on kernel boot

---

## Key Features

### 1. Handle-Based Sessions

**Problem (v1)**: ML models re-uploaded with every inference request (20MB × 100 requests = 2GB transferred)

**Solution**: Load once, reference by handle

```rust
// Create session (loads model into kernel memory)
let model = ctx.load("onnx://load")
    .with_source("hf://microsoft/whisper-tiny/encoder.onnx")
    .create()?;
// Returns: Handle("kernel://runtime/onnx/abc-123")

// Use handle many times (only sends input data)
for audio_file in audio_files {
    let result = model.call(json!({
        "inputs": load_audio(audio_file)
    }))?;
}

// Cleanup (or automatic on context drop)
model.close()?;
```

**Benefits**:
- 100x less data transferred (20MB once vs 20MB × 100)
- Model stays loaded in GPU/CPU cache
- Works for any stateful resource (DB connections, file handles, etc.)

### 2. Binary Streaming Protocol

**Problem (v1)**: JSON + base64 encoding = 4/3x size overhead, hardcoded 50MB buffer limit

**Solution**: Binary protocol (MessagePack) with streaming

```rust
// Stream large file without loading into memory
ctx.io("file:///models/llama-70b.bin")
    .stream_to("s3://my-bucket/backups/llama-70b.bin")?;

// Stream processing (read chunks, never buffer entire file)
let mut stream = ctx.io("hf://large-dataset.parquet").stream_read()?;
let mut processor = ctx.io("kernel://runtime/python/data-processor").stream_write()?;

std::io::copy(&mut stream, &mut processor)?;
```

**Benefits**:
- No buffer size limits
- Constant memory usage (chunk size, e.g., 64KB)
- Backpressure support
- Works with any size data (GB+)

### 3. Protocol Negotiation

Drivers and clients negotiate the best protocol:

```
Request Headers:
  Accept: application/msgpack, application/json;q=0.9
  Transfer-Encoding: chunked

Response Headers:
  Content-Type: application/msgpack
  Transfer-Encoding: chunked
```

**Supported Protocols**:
1. **MessagePack** (default for binary efficiency)
2. **JSON** (fallback for debugging, legacy)
3. **Cap'n Proto** (future: zero-copy)

### 4. Driver Capabilities

Each driver declares its capabilities during registration:

```go
type Driver interface {
    // Metadata
    URIPattern() string
    Class() DriverClass // io.connect, io.listen, runtime.*, kernel.*
    Capabilities() []Capability

    // Core methods
    Call(ctx Context, req Request) (Response, error)

    // Optional (based on capabilities)
    CreateHandle(ctx Context, args map[string]any) (Handle, error)
    Stream(ctx Context, req Request) (io.ReadWriteCloser, error)
    Listen(ctx Context, addr string) error
}

type Capability string

const (
    CapCall     Capability = "call"     // One-shot request/response
    CapStream   Capability = "stream"   // Streaming I/O
    CapHandle   Capability = "handle"   // Stateful sessions
    CapListen   Capability = "listen"   // Server/listener
    CapPubSub   Capability = "pubsub"   // Publish/subscribe
    CapTxn      Capability = "txn"      // Transactions
)
```

**Benefits**:
- Clients know what operations are supported
- IO Bus can optimize routing
- Clear contracts for driver implementers

---

## Driver Registration & Discovery

### Registration API

```go
// In driver implementation
func init() {
    iobus.Register(DriverSpec{
        Name:         "s3-client",
        Class:        io.ConnectDriver,
        URIPattern:   "s3://**",
        Capabilities: []Capability{CapCall, CapStream},

        // Runtime
        Runtime:      RuntimeNative, // or RuntimeWASM
        Binary:       "/usr/local/lib/wazeos/drivers/s3.so",

        // Permissions (what this driver needs)
        Permissions: []string{
            "http://**",              // Needs HTTP to call S3 API
            "kernel://credentials/**", // Needs AWS credentials
        },

        Factory: func() Driver {
            return &S3Driver{}
        },
    })
}
```

### Discovery & Routing

```
1. Request arrives: io("s3://my-bucket/file.txt")

2. IO Bus matches URI pattern:
   - Trie lookup: s3:// → finds S3Driver
   - Permission check: Does caller have "s3://my-bucket/**"?
   - Capability check: Does S3Driver support requested operation?

3. Route to driver:
   - If native: direct function call
   - If WASM: instantiate module, call exported function

4. Return result or handle
```

### URI Pattern Matching

Uses a Trie (prefix tree) for O(log n) lookups:

```
Patterns:
  file://**
  http://**
  https://**
  s3://**
  hf://models/**
  hf://datasets/**
  kernel://runtime/wasm/*
  kernel://runtime/onnx/*

Trie Structure:
  file:// → FileDriver
  http:// → HTTPDriver
  https:// → HTTPDriver
  s3:// → S3Driver
  hf://
    ├─ models/** → HFModelsDriver
    ├─ datasets/** → HFDatasetsDriver
  kernel://
    ├─ runtime/
    │   ├─ wasm/* → WASMRuntime
    │   ├─ onnx/* → ONNXRuntime
    ├─ security/
        ├─ audit → AuditDriver
        ├─ credentials → CredentialsDriver
```

**Matching Rules**:
- `*` matches single path segment
- `**` matches any number of segments
- Longest prefix wins
- Exact match beats wildcard

---

## Security Model

### Capability-Based Access Control

Every operation requires explicit permission grant:

```yaml
# Permission manifest for an MCP tool
tool: transcribe-audio
permissions:
  - hf://microsoft/whisper-*/**       # Can access Whisper models
  - kernel://runtime/onnx/*           # Can use ONNX runtime
  - file:///tmp/audio-*               # Can read audio in /tmp
  - s3://my-bucket/transcripts/**     # Can write results to S3
```

### Permission Inheritance

Drivers can request permissions, which are checked recursively:

```
MCP Tool "transcribe-audio"
  → Needs: hf://microsoft/whisper-tiny/**
    → HF Driver needs: http://huggingface.co/**
      → HTTP Driver needs: (none, native)

Permission check:
  1. Does tool have "hf://microsoft/whisper-tiny/**"? ✓
  2. Does HF driver have "http://huggingface.co/**"? ✓
  3. Route approved
```

### Audit Trail

All operations logged to `kernel://security/audit`:

```json
{
  "timestamp": "2026-05-08T10:45:30Z",
  "principal": "mcp-tool:transcribe-audio:v1.0.0",
  "action": "io.call",
  "uri": "hf://microsoft/whisper-tiny/encoder.onnx",
  "result": "success",
  "bytes_transferred": 20971520,
  "duration_ms": 450
}
```

---

## SDK Architecture

### Rust SDK (for WASM components)

```rust
// High-level API
use wazeos_sdk::{Context, Handle};

#[wazeos::mcp_tool]
fn transcribe(ctx: &Context, audio_path: String) -> Result<String> {
    // Load model (creates handle, model stays in kernel)
    let model = ctx.load("onnx://load")
        .with_source("hf://microsoft/whisper-tiny/encoder.onnx")
        .create()?;

    // Stream audio directly to preprocessing
    let audio = ctx.io(&audio_path).stream_read()?;
    let preprocessed = preprocess_audio(audio)?;

    // Inference (only sends audio data, not model)
    let result = model.call(json!({
        "inputs": { "audio": preprocessed }
    }))?;

    // Handle auto-closes on drop
    Ok(result["text"].as_str().unwrap().to_string())
}
```

### Go SDK (for native drivers)

```go
package main

import "github.com/wazeos/sdk/driver"

type S3Driver struct {
    client *s3.Client
}

func (d *S3Driver) Call(ctx driver.Context, req driver.Request) (driver.Response, error) {
    bucket, key := parseS3URI(req.URI)

    switch req.Headers["operation"] {
    case "read":
        data, err := d.client.GetObject(ctx, bucket, key)
        return driver.NewResponse(200, data), err
    case "write":
        err := d.client.PutObject(ctx, bucket, key, req.Body)
        return driver.NewResponse(200, nil), err
    default:
        return driver.NewError(400, "unsupported operation")
    }
}

func (d *S3Driver) Stream(ctx driver.Context, req driver.Request) (io.ReadWriteCloser, error) {
    bucket, key := parseS3URI(req.URI)
    return d.client.GetObjectStream(ctx, bucket, key)
}
```

### Python SDK (future)

```python
from wazeos import Context, mcp_tool

@mcp_tool
def analyze_data(ctx: Context, dataset_uri: str) -> dict:
    # Stream processing with pandas
    with ctx.io(dataset_uri).stream_read() as stream:
        df = pd.read_csv(stream, chunksize=10000)
        result = process_chunks(df)

    return result
```

---

## Developer Experience & End-to-End Testing

To ensure WazeOS v2 delivers on its promise of simplicity and extensibility, we define three end-to-end test scenarios that validate the complete developer experience from driver creation to app deployment.

### Test Scenario 1: Time App with Shell Driver

**Goal**: Verify users can create a custom driver and app that work together

**Components**:
1. Shell driver (`shell://`) - Executes local bash/zsh commands
2. Time app - MCP tool that returns current system time

#### 1.1 Shell Driver Implementation

```go
// drivers/shell/driver.go
package shell

import (
    "context"
    "os/exec"
    "github.com/wazeos/wazeos/v2/kernel/iobus"
)

type ShellDriver struct{}

func init() {
    iobus.Register(iobus.DriverSpec{
        Name:    "shell-driver",
        Version: "1.0.0",
        Class:   iobus.ConnectDriver,
        URIPattern: "shell://**",
        Capabilities: []iobus.Capability{iobus.CapCall},
        Runtime: iobus.RuntimeNative,
        Permissions: []string{}, // Needs elevated permissions

        Factory: func() iobus.Driver {
            return &ShellDriver{}
        },
    })
}

func (d *ShellDriver) URIPattern() string { return "shell://**" }
func (d *ShellDriver) Class() iobus.DriverClass { return iobus.ConnectDriver }
func (d *ShellDriver) Capabilities() []iobus.Capability { return []iobus.Capability{iobus.CapCall} }
func (d *ShellDriver) Init(ctx context.Context, config iobus.Config) error { return nil }
func (d *ShellDriver) Close() error { return nil }

func (d *ShellDriver) Call(ctx iobus.Context, req iobus.Request) (iobus.Response, error) {
    // Get command from headers
    command := req.Headers["command"]
    if command == "" {
        return iobus.NewErrorResponse(400, "command header required"), nil
    }

    // Execute command
    cmd := exec.CommandContext(ctx, "sh", "-c", command)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return iobus.NewErrorResponse(500, string(output)), nil
    }

    return iobus.NewResponse(200, output), nil
}
```

#### 1.2 Time App Implementation

```rust
// apps/time/src/main.rs
use wazeos_sdk::{Context, mcp_tool, json, Result};

#[mcp_tool]
fn get_time(ctx: &Context) -> Result<String> {
    // Call shell driver to get current time
    let result = ctx.io("shell://exec")
        .call(json!({
            "command": "date '+%Y-%m-%d %H:%M:%S %Z'"
        }))?;

    let time_str = String::from_utf8_lossy(&result.body).to_string();

    Ok(json!({
        "local_time": time_str.trim(),
        "source": "shell:date"
    }).to_string())
}

fn main() {
    wazeos_sdk::run_mcp_tool(&get_time);
}
```

#### 1.3 Developer Workflow (Using CLI Tools)

**For Humans (Interactive)**:

```bash
# Step 1: Create driver scaffold
$ wazeos driver new shell --class io.connect --language go
✓ Created driver scaffold at drivers/shell/
→ Next: Edit drivers/shell/driver.go to implement Call() method

# Step 2: Implement driver logic
$ cd drivers/shell
# ... edit driver.go to add shell execution logic ...

# Step 3: Build driver
$ wazeos driver build shell
Building Go driver: shell
✓ Build complete: drivers/shell/build/shell.so

# Step 4: Test driver
$ wazeos driver test shell
Running tests for shell driver...
✓ All tests passed (5/5)

# Step 5: Install driver
$ wazeos driver install shell
✓ Driver installed: shell:// → shell driver v1.0.0

# Step 6: Create app scaffold
$ wazeos app new time --language rust
✓ Created app scaffold at apps/time/
→ Next: Edit apps/time/src/main.rs to implement tool logic

# Step 7: Implement app logic
$ cd apps/time
# ... edit src/main.rs to call shell driver ...

# Step 8: Build app
$ wazeos app build time
Building Rust app: time
✓ Build complete: apps/time/target/wasm32-wasi/release/time.wasm

# Step 9: Test app locally
$ wazeos app test time
Running tests for time app...
✓ All tests passed (3/3)

# Step 10: Install app
$ wazeos app install time
✓ App installed: time (1 tool: get_time)

# Step 11: Run app
$ wazeos invoke time/get_time
{
  "local_time": "2026-05-08 14:30:00 PDT",
  "source": "shell:date"
}
```

**For AI Agents (Non-Interactive)**:

```bash
# Single-command workflow with JSON output
$ wazeos driver new shell \
    --class io.connect \
    --language go \
    --json | jq -r '.files_created[]'
drivers/shell/driver.go
drivers/shell/driver_test.go
drivers/shell/README.md

# Agent edits driver.go programmatically

$ wazeos driver build shell --json
{"status":"success","binary":"drivers/shell/build/shell.so","size_bytes":2048576}

$ wazeos driver test shell --json
{"status":"success","tests_passed":5,"tests_failed":0}

$ wazeos driver install shell --json
{"status":"success","driver":"shell://","version":"1.0.0"}

# Same for app
$ wazeos app new time --language rust --json
# ... agent edits files ...
$ wazeos app build time --json
$ wazeos app install time --json

# Invoke with JSON output
$ wazeos invoke time/get_time --json
{
  "status": "success",
  "result": {
    "local_time": "2026-05-08 14:30:00 PDT",
    "source": "shell:date"
  },
  "duration_ms": 45
}
```

**Success Criteria**:
- ✅ `wazeos driver new` creates valid scaffold
- ✅ `wazeos driver build` compiles successfully
- ✅ `wazeos driver test` passes all tests
- ✅ `wazeos app new` creates valid scaffold
- ✅ `wazeos app build` produces working WASM
- ✅ `wazeos invoke` returns correct result
- ✅ JSON output mode works for agents

---

### Test Scenario 2: Random Wikipedia App with HTTP Driver

**Goal**: Verify HTTP client driver and network-based operations

**Components**:
1. HTTP driver (`http://`, `https://`) - Already implemented as core driver
2. Random app - MCP tool that fetches random Wikipedia article

#### 2.1 Random App Implementation

```rust
// apps/random/src/main.rs
use wazeos_sdk::{Context, mcp_tool, json, Result};
use serde_json::Value;

#[mcp_tool]
fn random_article(ctx: &Context) -> Result<Value> {
    // Get random article URL
    let redirect_result = ctx.io("https://en.wikipedia.org/wiki/Special:Random")
        .call(json!({
            "method": "GET",
            "follow_redirects": false
        }))?;

    // Extract redirect location
    let location = redirect_result.headers
        .get("location")
        .ok_or("No redirect location found")?;

    // Fetch the actual article
    let article_result = ctx.io(location)
        .call(json!({
            "method": "GET"
        }))?;

    let html = String::from_utf8_lossy(&article_result.body);

    // Extract title (simple parsing)
    let title = extract_title(&html)?;

    Ok(json!({
        "title": title,
        "url": location,
        "size_bytes": html.len()
    }))
}

fn extract_title(html: &str) -> Result<String> {
    // Simple title extraction (production would use proper HTML parser)
    let start = html.find("<title>").ok_or("No title tag")? + 7;
    let end = html[start..].find("</title>").ok_or("No closing title")? + start;
    Ok(html[start..end].replace(" - Wikipedia", ""))
}

fn main() {
    wazeos_sdk::run_mcp_tool(&random_article);
}
```

#### 2.2 Developer Workflow (Using CLI Tools)

**For Humans (Interactive)**:

```bash
# HTTP driver is built-in, no need to create

# Step 1: Create app
$ wazeos app new random --language rust
✓ Created app scaffold at apps/random/

# Step 2: Implement app logic
$ cd apps/random
# ... edit src/main.rs to call HTTP driver and parse Wikipedia ...

# Step 3: Build app
$ wazeos app build random
Building Rust app: random
Compiling with dependencies: serde, serde_json
✓ Build complete: apps/random/target/wasm32-wasi/release/random.wasm (1.2MB)

# Step 4: Test app locally
$ wazeos app test random
Running tests for random app...
Testing HTTP redirect handling...
Testing HTML parsing...
✓ All tests passed (4/4)

# Step 5: Install app
$ wazeos app install random
✓ App installed: random (1 tool: random_article)

# Step 6: Run app
$ wazeos invoke random/random_article
{
  "title": "Quantum Entanglement",
  "url": "https://en.wikipedia.org/wiki/Quantum_Entanglement",
  "size_bytes": 125847
}
```

**For AI Agents (Non-Interactive)**:

```bash
# Create, build, install in pipeline
$ wazeos app new random --language rust --json
{"status":"success","path":"apps/random","files_created":["apps/random/src/main.rs","apps/random/Cargo.toml"]}

# Agent edits files

$ wazeos app build random --json
{"status":"success","wasm":"apps/random/target/wasm32-wasi/release/random.wasm","size_bytes":1245184}

$ wazeos app install random --json
{"status":"success","app":"random","tools":["random_article"]}

$ wazeos invoke random/random_article --json
{
  "status": "success",
  "result": {
    "title": "Quantum Entanglement",
    "url": "https://en.wikipedia.org/wiki/Quantum_Entanglement",
    "size_bytes": 125847
  },
  "duration_ms": 312
}
```

**Success Criteria**:
- ✅ `wazeos app new` creates scaffold with correct dependencies
- ✅ `wazeos app build` compiles and links successfully
- ✅ HTTP driver (built-in) handles redirects correctly
- ✅ App makes multiple HTTP requests
- ✅ App parses HTML and extracts data
- ✅ `wazeos invoke` returns structured response

---

### Test Scenario 3: Temp File Manager App with File Driver

**Goal**: Verify file operations (list, write, read)

**Components**:
1. File driver (`file://`) - Already implemented as core driver
2. Temp app - MCP tool that lists /tmp, writes to file, reads it back

#### 3.1 Temp App Implementation

```rust
// apps/temp/src/main.rs
use wazeos_sdk::{Context, mcp_tool, json, Result};
use serde_json::Value;

#[mcp_tool]
fn list_and_save(ctx: &Context) -> Result<Value> {
    // Step 1: List files in /tmp
    let list_result = ctx.io("file:///tmp")
        .call(json!({
            "operation": "list"
        }))?;

    let entries: Vec<Value> = serde_json::from_slice(&list_result.body)?;

    // Step 2: Format the list
    let mut content = String::from("Files in /tmp:\n\n");
    for entry in &entries {
        let name = entry["name"].as_str().unwrap();
        let is_dir = entry["is_dir"].as_bool().unwrap();
        let size = entry["size"].as_i64().unwrap();

        let type_str = if is_dir { "DIR " } else { "FILE" };
        content.push_str(&format!("{} {:>10} bytes  {}\n", type_str, size, name));
    }

    // Step 3: Write to /tmp/contents.txt
    let write_result = ctx.io("file:///tmp/contents.txt")
        .call(json!({
            "operation": "write",
            "body": content.as_bytes()
        }))?;

    // Step 4: Read it back
    let read_result = ctx.io("file:///tmp/contents.txt")
        .call(json!({
            "operation": "read"
        }))?;

    let read_content = String::from_utf8_lossy(&read_result.body);

    // Verify content matches
    let matches = content == read_content;

    Ok(json!({
        "files_listed": entries.len(),
        "file_created": "/tmp/contents.txt",
        "content_length": content.len(),
        "verification": if matches { "PASS" } else { "FAIL" },
        "preview": read_content.lines().take(5).collect::<Vec<_>>().join("\n")
    }))
}

fn main() {
    wazeos_sdk::run_mcp_tool(&list_and_save);
}
```

#### 3.2 Developer Workflow (Using CLI Tools)

**For Humans (Interactive)**:

```bash
# File driver is built-in, no need to create

# Step 1: Create app
$ wazeos app new temp --language rust
✓ Created app scaffold at apps/temp/

# Step 2: Implement app logic
$ cd apps/temp
# ... edit src/main.rs to list, write, read files ...

# Step 3: Build app
$ wazeos app build temp
Building Rust app: temp
✓ Build complete: apps/temp/target/wasm32-wasi/release/temp.wasm (892KB)

# Step 4: Test app locally
$ wazeos app test temp
Running tests for temp app...
Testing file list operation...
Testing file write operation...
Testing file read operation...
✓ All tests passed (6/6)

# Step 5: Install app
$ wazeos app install temp
✓ App installed: temp (1 tool: list_and_save)

# Step 6: Run app
$ wazeos invoke temp/list_and_save
{
  "files_listed": 42,
  "file_created": "/tmp/contents.txt",
  "content_length": 2847,
  "verification": "PASS",
  "preview": "Files in /tmp:\n\nDIR           0 bytes  logs\nFILE       1024 bytes  test.txt\n..."
}

# Step 7: Verify file was actually created
$ cat /tmp/contents.txt
Files in /tmp:

DIR           0 bytes  logs
FILE       1024 bytes  test.txt
...
```

**For AI Agents (Non-Interactive)**:

```bash
# Streamlined pipeline
$ wazeos app new temp --language rust --json
{"status":"success","path":"apps/temp"}

# Agent edits files

$ wazeos app build temp --json
{"status":"success","wasm":"apps/temp/target/wasm32-wasi/release/temp.wasm","size_bytes":913408}

$ wazeos app test temp --json
{"status":"success","tests_passed":6,"tests_failed":0}

$ wazeos app install temp --json
{"status":"success","app":"temp","tools":["list_and_save"]}

$ wazeos invoke temp/list_and_save --json
{
  "status": "success",
  "result": {
    "files_listed": 42,
    "file_created": "/tmp/contents.txt",
    "content_length": 2847,
    "verification": "PASS",
    "preview": "Files in /tmp:\\n\\nDIR           0 bytes  logs\\nFILE       1024 bytes  test.txt"
  },
  "duration_ms": 28
}

# Agent verifies file existence
$ wazeos file exists /tmp/contents.txt --json
{"status":"success","exists":true,"size_bytes":2847}
```

**Success Criteria**:
- ✅ `wazeos app new` creates valid scaffold
- ✅ `wazeos app build` compiles successfully
- ✅ `wazeos app test` validates all operations
- ✅ File driver (built-in) lists directory contents
- ✅ File driver writes file successfully
- ✅ File driver reads file correctly
- ✅ Content verification passes in app
- ✅ File persists on disk (verifiable by shell)

---

### Test Suite Structure

```
tests/
├── e2e/
│   ├── test_time_app.sh           # Test scenario 1
│   ├── test_random_app.sh         # Test scenario 2
│   ├── test_temp_app.sh           # Test scenario 3
│   └── common.sh                  # Shared test utilities
│
├── drivers/
│   ├── shell_driver_test.go       # Unit tests for shell driver
│   ├── http_driver_test.go        # Unit tests for http driver
│   └── file_driver_test.go        # Unit tests for file driver
│
└── apps/
    ├── time_app_test.rs           # Unit tests for time app
    ├── random_app_test.rs         # Unit tests for random app
    └── temp_app_test.rs           # Unit tests for temp app
```

### CI/CD Integration

```yaml
# .github/workflows/e2e-tests.yml
name: E2E Tests

on: [push, pull_request]

jobs:
  e2e-tests:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v3

      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.23'

      - name: Setup Rust
        uses: actions-rs/toolchain@v1
        with:
          toolchain: stable
          target: wasm32-wasi

      - name: Build kernel
        run: |
          cd v2/kernel
          go build -o wazeos ./...

      - name: Test Scenario 1 - Time App
        run: ./tests/e2e/test_time_app.sh

      - name: Test Scenario 2 - Random App
        run: ./tests/e2e/test_random_app.sh

      - name: Test Scenario 3 - Temp App
        run: ./tests/e2e/test_temp_app.sh

      - name: Upload test artifacts
        if: failure()
        uses: actions/upload-artifact@v3
        with:
          name: test-failures
          path: tests/e2e/*.log
```

### Developer Experience Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Time to create new driver | < 30 min | From scaffold to working driver |
| Time to create new app | < 15 min | From scaffold to working app |
| Lines of code (driver) | < 100 LOC | Excluding boilerplate |
| Lines of code (app) | < 50 LOC | Simple MCP tool |
| Build time (driver) | < 5 sec | Go native driver |
| Build time (app) | < 30 sec | Rust WASM app |
| Test execution time | < 10 sec | All 3 E2E scenarios |

### Documentation Requirements

Each test scenario must include:
1. **Tutorial** - Step-by-step guide in docs/
2. **API reference** - Generated from code comments
3. **Video walkthrough** - 5-minute screencast
4. **Troubleshooting guide** - Common issues and solutions
5. **Best practices** - Performance tips and patterns

---

## Performance Targets

| Metric | v1 | v2 Target |
|--------|----|---------:|
| Max file size (in-memory) | 50MB | ∞ (streaming) |
| WASM memory usage (20MB model) | 27MB | 0 bytes |
| Model load time (20MB) | 450ms | 450ms (same) |
| Inference latency overhead | +5ms | +0.5ms |
| Concurrent handles per session | N/A | 1000+ |
| Max streaming throughput | N/A | 1GB/s |

---

## CLI Tooling Requirements

To support the agent-native developer experience, WazeOS v2 requires a comprehensive CLI tool (`wazeos`) that works equally well for humans and AI agents.

### Core Commands

#### `wazeos driver` - Driver Management

```bash
# Create new driver
wazeos driver new <name> [--class CLASS] [--language LANG] [--json]
  Creates driver scaffold with boilerplate
  --class: io.connect|io.listen|runtime.*|kernel.*
  --language: go|rust
  --json: Output machine-readable JSON

# Build driver
wazeos driver build <name> [--release] [--json]
  Compiles driver to binary (.so for Go, .wasm for Rust)
  --release: Optimized build
  --json: Output build metadata

# Test driver
wazeos driver test <name> [--verbose] [--json]
  Runs driver unit tests
  --verbose: Show detailed output
  --json: Output test results

# Install driver
wazeos driver install <path> [--force] [--json]
  Registers driver with kernel
  --force: Overwrite existing
  --json: Output installation status

# List drivers
wazeos driver list [--json]
  Shows all installed drivers
  --json: Machine-readable list

# Uninstall driver
wazeos driver uninstall <name> [--json]
  Removes driver from kernel
```

#### `wazeos app` - Application Management

```bash
# Create new app
wazeos app new <name> [--language LANG] [--template TEMPLATE] [--json]
  Creates app scaffold with MCP tool boilerplate
  --language: rust|go|python|javascript
  --template: mcp-tool|cli|http-handler
  --json: Output metadata

# Build app
wazeos app build <name> [--release] [--json]
  Compiles app to WASM
  --release: Optimized build with strip
  --json: Output build metadata

# Test app
wazeos app test <name> [--verbose] [--json]
  Runs app unit tests
  --verbose: Show detailed output
  --json: Output test results

# Install app
wazeos app install <path> [--json]
  Registers app with MCP server
  --json: Output installation status

# List apps
wazeos app list [--json]
  Shows all installed apps
  --json: Machine-readable list

# Uninstall app
wazeos app uninstall <name> [--json]
  Removes app from system
```

#### `wazeos invoke` - Tool Invocation

```bash
# Invoke MCP tool
wazeos invoke <app>/<tool> [ARGS] [--json] [--timeout DURATION]
  Executes an MCP tool
  ARGS: Tool-specific arguments (JSON or key=value)
  --json: Machine-readable output
  --timeout: Max execution time (default: 30s)

# Examples
wazeos invoke time/get_time
wazeos invoke random/random_article --json
wazeos invoke transcribe/audio file=audio.wav model=whisper-tiny
```

#### `wazeos dev` - Development Utilities

```bash
# Start development server
wazeos dev serve [--port PORT]
  Starts local MCP server with hot-reload
  --port: Server port (default: 8080)

# Inspect driver
wazeos dev inspect driver <name> [--json]
  Shows driver metadata and capabilities
  --json: Machine-readable

# Inspect app
wazeos dev inspect app <name> [--json]
  Shows app tools and permissions
  --json: Machine-readable

# Debug invocation
wazeos dev debug <app>/<tool> [ARGS]
  Runs tool with verbose logging
```

#### `wazeos file` - File Utilities (for agents)

```bash
# Check file existence
wazeos file exists <path> [--json]
  Returns file existence status
  --json: {"exists": true, "size_bytes": 1234}

# Read file
wazeos file read <path> [--json]
  Outputs file contents
  --json: {"content": "...", "size_bytes": 1234}

# Write file
wazeos file write <path> <content> [--json]
  Writes content to file
  --json: {"success": true, "size_bytes": 1234}
```

### JSON Output Format

All commands with `--json` flag output structured data:

```json
{
  "status": "success" | "error",
  "command": "driver build",
  "result": {
    // Command-specific result data
  },
  "errors": [
    {
      "code": "BUILD_FAILED",
      "message": "Compilation error in driver.go:42",
      "suggestion": "Check syntax near line 42"
    }
  ],
  "duration_ms": 1234
}
```

### Error Handling

Exit codes:
- `0` - Success
- `1` - User error (bad arguments, file not found, etc.)
- `2` - System error (build failed, runtime error, etc.)
- `3` - Permission denied

Error messages include:
- **What went wrong**: Clear description
- **Why it happened**: Root cause if known
- **How to fix it**: Actionable suggestion
- **Related docs**: Link to relevant documentation

Example:
```bash
$ wazeos driver build missing-driver
ERROR: Driver not found: missing-driver

Searched in:
  - ./drivers/missing-driver
  - ~/.wazeos/drivers/missing-driver

Did you mean one of these?
  - shell-driver
  - http-driver

To create a new driver:
  wazeos driver new missing-driver --class io.connect

Docs: https://wazeos.dev/docs/drivers/create
```

### Scaffolding Templates

#### Driver Template (Go)

```
drivers/<name>/
├── driver.go           # Main driver implementation
├── driver_test.go      # Unit tests
├── go.mod              # Dependencies
├── README.md           # Documentation
└── wazeos.yaml         # Driver manifest
```

#### App Template (Rust)

```
apps/<name>/
├── src/
│   └── main.rs         # Main app implementation
├── tests/
│   └── integration_test.rs
├── Cargo.toml          # Dependencies
├── README.md           # Documentation
└── wazeos.yaml         # App manifest
```

### Manifest Format (wazeos.yaml)

**Driver Manifest**:
```yaml
name: shell-driver
version: 1.0.0
class: io.connect
uri_pattern: shell://**
capabilities:
  - call
runtime: native
permissions:
  - kernel://security/audit
description: Executes local shell commands
author: Your Name <you@example.com>
```

**App Manifest**:
```yaml
name: time
version: 1.0.0
language: rust
tools:
  - name: get_time
    description: Returns current system time
    input_schema:
      type: object
      properties: {}
    permissions:
      - shell://**
description: Time utility app
author: Your Name <you@example.com>
```

### Implementation Priority

**Phase 1 (Weeks 1-2)**: Core commands
- `wazeos driver new/build/install`
- `wazeos app new/build/install`
- `wazeos invoke`
- Basic JSON output

**Phase 2 (Weeks 3-4)**: Testing and development
- `wazeos driver test`
- `wazeos app test`
- `wazeos dev serve/inspect`
- Enhanced error messages

**Phase 3 (Weeks 5-6)**: Agent utilities
- `wazeos file` commands
- Full JSON output mode
- Interactive prompts for humans
- Shell completion

**Phase 4 (Weeks 7-8)**: Polish
- Documentation generation
- Templates for common patterns
- Performance benchmarks
- CI/CD integration helpers

---

## Migration Path (v1 → v2)

### Phase 1: Backward Compatible Handle API (Week 1-2)
- Add handle support to kernel
- Implement `CreateHandle()` in ONNX driver
- v1 `Call()` method still works

### Phase 2: Binary Protocol Support (Week 3-4)
- Implement MessagePack serialization
- Add content negotiation
- JSON still supported (fallback)

### Phase 3: Streaming API (Week 5-6)
- Add streaming host functions
- Update SDKs with streaming traits
- File and HTTP drivers gain streaming

### Phase 4: Driver Classes (Week 7-8)
- Implement io.connect, io.listen, runtime.*, kernel.* classes
- Migrate existing drivers to new registration API
- Update documentation

### Phase 5: Migration Tooling (Week 9-10)
- Automated refactoring tools
- v1 → v2 code migration scripts
- Deprecation warnings

---

## Success Criteria

1. **Performance**: ML workflows use <10% memory of v1
2. **Adoption**: 3+ community drivers in each class
3. **Reliability**: Zero "buffer too small" or "memory allocation" errors
4. **Developer Experience**:
   - 80% reduction in SDK boilerplate
   - All 3 E2E test scenarios pass (time, random, temp apps)
   - New driver creation < 30 minutes
   - New app creation < 15 minutes
5. **Testing**:
   - 100% of core drivers have E2E tests
   - CI/CD runs all E2E tests on every commit
   - Test execution time < 10 seconds total

---

## Open Questions

1. **Handle Sharing**: Should handles be shareable between WASM instances? (Likely no for security)
2. **Handle Lifetime**: Tied to request context? Explicit TTL? (Both, with sensible defaults)
3. **Handle Transfer**: Can component A create handle, pass to B? (Yes, with permission delegation)
4. **Protocol Performance**: MessagePack vs Cap'n Proto vs custom binary? (Benchmark all three)

---

## References

- [RFC-001: Driver Architecture](./RFC-001-driver-architecture.md)
- [RFC-002: Handle System](./RFC-002-handle-system.md)
- [RFC-003: Binary Protocol](./RFC-003-binary-protocol.md)
- [RFC-004: Security Model](./RFC-004-security-model.md)

---

## Appendix: v1 Post-Mortem

**What Worked**:
- URI-based resource abstraction
- Capability-based permissions
- MCP integration
- Multi-language SDK approach

**What Didn't Work**:
- JSON + base64 for large binary data
- Stateless operations for expensive resources (models, connections)
- Hardcoded buffer limits
- Unclear driver vs app distinction
- No streaming support

**Key Lesson**: Don't move large data through WASM. Use handles/references instead.
