# WazeOS v2 Directory Structure

Clean separation between Go module code and Rust WASM projects.

## Overview

```
repository root/
├── v1/               # Archived v1 (historical reference)
│
├── v2/               # Go module (platform code only)
│   ├── go.mod        # Go module definition
│   ├── core/         # Core platform components
│   ├── drivers/runtime/  # Go runtime driver
│   ├── docs/         # Documentation
│   ├── tests/        # Test suites
│   ├── examples/     # Example references
│   └── bin/          # Built binaries
│
├── drivers/          # Rust WASM I/O drivers (outside Go module)
│   ├── file/         # file:// driver + SDK addon
│   ├── shell/        # shell:// driver + SDK addon
│   └── http/         # http:// driver + SDK addon
│
└── apps/             # Rust WASM applications (outside Go module)
    └── test-tool/    # Example application
```

**Key Principle**: Go code stays in `v2/`, Rust WASM projects live outside.

## Detailed Structure

### `v2/core/` - Go Platform Code

All Go code for the WazeOS platform.

```
v2/core/
├── cmd/wazeos/       # CLI tool
│   ├── main.go
│   ├── app.go        # App commands
│   └── mcp.go        # MCP server
│
├── kernel/           # Core IO Bus
│   ├── iobus/        # Bus implementation
│   │   ├── types.go
│   │   ├── router.go
│   │   └── iobus.go
│   └── drivers.go    # Driver registration
│
├── internal/         # Internal packages
│   └── native/       # Native implementations (Go)
│       ├── file/     # OS file operations
│       ├── shell/    # Process execution
│       └── http/     # HTTP client
│
└── sdk/              # SDKs
    └── rust/wazeos-app/  # Core app SDK (Rust)
        ├── Cargo.toml
        └── src/lib.rs    # AppContext, call(), etc.
```

**Key Concepts:**
- URI pattern matching (trie-based)
- Permission enforcement
- Native implementations (internal only)
- Generic SDK foundation

### `v2/drivers/runtime/` - WASM Runtime (Go)

The WASM execution engine that runs WASM drivers and apps.

```
v2/drivers/runtime/wasm/
└── driver.go         # WASM runtime using wazero
```

**Purpose:** Executes WASM modules, provides host functions to WASM code.
**Language:** Go (part of v2/ module)
**Why in v2/:** Imports from `core/kernel/iobus`, must be same Go module.

### `drivers/` - WASM I/O Drivers (Rust)

Independent Rust projects that compile to WASM drivers.

```
drivers/
├── file/             # file:// driver
│   ├── Cargo.toml
│   ├── src/lib.rs    # WASM driver implementation
│   └── sdk/rust/     # SDK addon
│       ├── Cargo.toml
│       └── src/lib.rs  # FileOps trait
│
├── shell/            # shell:// driver
│   ├── Cargo.toml
│   ├── src/lib.rs
│   └── sdk/rust/     # ShellOps trait
│
└── http/             # http:// driver
    ├── Cargo.toml
    ├── src/lib.rs
    └── sdk/rust/     # HttpOps trait
```

**Architecture:**
- **Driver** (`src/lib.rs`): WASM module that routes to native implementations
- **SDK Addon** (`sdk/rust/`): Trait extending AppContext with ergonomic methods

**Flow:**
```
App → SDK Addon (FileOps) → Driver (file.wasm) → Native (Go) → OS
```

**Why outside v2/:** These are Rust projects with no Go dependencies. They compile to .wasm binaries that v2/ loads at runtime.

### `apps/` - WASM Applications (Rust)

Independent Rust applications that compile to WASM.

```
apps/
└── test-tool/
    ├── wazeos.toml   # Manifest + permissions
    ├── Cargo.toml    # Dependencies
    ├── src/lib.rs    # Implementation
    └── target/       # Build artifacts
        └── wasm32-wasip1/release/
            └── test_tool.wasm
```

**Dependencies:**
- Core SDK: `v2/core/sdk/rust/app`
- Driver addons: `drivers/file/sdk/rust`, etc.

**Why outside v2/:** Rust projects with no Go dependencies. They're user applications, not platform code.

### `/docs/` - Documentation

Comprehensive documentation for users and developers.

```
docs/
├── README.md                     # Documentation index
├── PACKAGING_GUIDE.md            # How to package apps
├── PACKAGE_FORMAT.md             # .wazpkg specification
├── CLAUDE_DESKTOP.md             # Claude Desktop integration
├── PRD.md                        # Product requirements
├── RFC-001-driver-architecture.md
├── RFC-002-handle-system.md
├── RFCs.md
└── archive/                      # Historical documents
    ├── IO_IMPLEMENTATION_COMPLETE.md
    ├── IMPLEMENTATION_STATUS.md
    └── ...
```

### `/internal/` - Internal Packages

Internal utilities and packages.

```
internal/
├── pkg/              # Package management
│   └── manifest.go   # wazeos.toml parsing
└── ...
```

### `/tests/` - Test Suites

Integration and end-to-end tests.

```
tests/
└── test_io_capabilities.sh    # I/O test suite
```

## Installation Locations

### `~/.wazeos/` - User Data

```
~/.wazeos/
├── apps/             # Installed apps
│   └── test-tool/
│       ├── wazeos.toml
│       └── test-tool.wasm
│
├── drivers/          # Installed drivers
│   ├── file-driver-wasm.wasm
│   ├── shell-driver-wasm.wasm
│   └── http-driver-wasm.wasm
│
└── packages/         # Package cache (future)
```

### `~/.config/claude/` - Claude Desktop Config

```
~/.config/claude/
└── mcp_servers.json
    └── "wazeos": {
          "command": "wazeos",
          "args": ["mcp", "server"]
        }
```

## Build Artifacts

### Cargo Build Output

```
target/                      # Rust build cache
└── wasm32-wasip1/
    └── release/
        └── *.wasm           # Compiled WASM binaries
```

### Package Output

```
*.wazpkg                     # Distributable packages
```

**Note:** `.wazpkg` files are tar.gz archives containing:
- `wazeos.toml` - Manifest
- `app-name.wasm` - WASM binary
- `package.json` - Metadata

## Key Design Principles

### 1. Go Module Separation

- **v2/** = Go module containing only Go code
- **drivers/** = Rust WASM projects (independent, no Go dependencies)
- **apps/** = Rust WASM applications (independent, no Go dependencies)
- Clear boundary: v2/ is the Go module root, everything else is external

### 2. Separation of Concerns

- **Native implementations** (`v2/core/internal/native/`) - Direct OS interaction (Go)
- **Runtime driver** (`v2/drivers/runtime/wasm/`) - WASM execution (Go)
- **I/O drivers** (`drivers/`) - Sandboxed routing (Rust → WASM)
- **SDK addons** (`drivers/*/sdk/rust/`) - Ergonomic APIs (Rust)
- **Core SDK** (`v2/core/sdk/rust/app/`) - Generic, driver-agnostic (Rust)

### 3. Extensibility

- New drivers don't require SDK updates
- Drivers provide their own SDK addons
- Apps can use raw `call()` for any driver
- Rust projects compile independently from Go module

### 4. Security

- WASM sandboxing for apps and drivers
- Permission-based access control
- Native code isolated in v2/core/internal/
- Minimal trusted computing base

### 5. Distribution

- Apps packaged as `.wazpkg` files
- Self-contained with manifest and binary
- Easy sharing and installation

## Navigation Guide

### I Want To...

**Build an app:**
1. Check `apps/test-tool/` for example
2. Read `v2/core/sdk/rust/app/src/lib.rs` for SDK docs
3. Check `drivers/*/sdk/rust/` for driver APIs

**Create a driver:**
1. Study `drivers/file/` structure
2. Read `v2/docs/RFC-001-driver-architecture.md`
3. Implement driver + SDK addon (Rust projects)

**Understand architecture:**
1. Read `v2/docs/PRD.md` for overview
2. Read `v2/docs/ARCHITECTURE.md` for layered design
3. Check `v2/core/kernel/iobus/types.go` for interfaces
4. Study `v2/core/cmd/wazeos/mcp.go` for MCP integration

**Package and distribute:**
1. Read `v2/docs/PACKAGING_GUIDE.md`
2. Use `wazeos app package`
3. Share `.wazpkg` file

---

**Last Updated:** 2026-05-08
**Version:** 2.0.0
