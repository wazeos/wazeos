# WazeOS v2

**Build portable, secure WASM-based MCP tools with full I/O capabilities**

WazeOS is a platform for building Model Context Protocol (MCP) tools that run as WebAssembly modules. It provides a secure, permission-controlled environment where tools can access files, execute shell commands, make HTTP requests, and more—all while maintaining portability and sandboxing.

## ✨ Key Features

- **🔒 Secure Sandbox**: WASM-based isolation with fine-grained permissions
- **🚀 Portable**: Build once, run anywhere (macOS, Linux, Windows)
- **🔌 Generic I/O**: Extensible driver system for any resource type
- **📦 Easy Distribution**: Package apps as `.wazpkg` files for sharing
- **🤝 MCP Integration**: Works seamlessly with Claude Desktop and other MCP clients
- **🦀 Multi-Language**: Build in any WASM language - Rust, C, Go, AssemblyScript, Zig, etc.
- **⚡ Production-Ready**: Full end-to-end I/O, packaging, and tooling

## 🎯 What Can You Build?

WazeOS apps are MCP tools that can:
- **Read and write files** (with permission control)
- **Execute shell commands** (sandboxed)
- **Make HTTP requests** (to approved domains)
- **Process data** (pure computation)
- **Access custom resources** (via extensible drivers)

Example use cases:
- File search and manipulation tools
- Git helpers for code management
- Log analyzers and data processors
- API integration tools
- Custom development utilities

## 🚀 Quick Start

### Prerequisites

- **Go 1.21+** (for building WazeOS CLI)
- **Rust + wasm32-wasip1 target** (for building apps)
- **jq** (optional, for testing)

```bash
# Install Rust target
rustup target add wasm32-wasip1
```

### Installation

```bash
# Clone repository
git clone https://github.com/wazeos/wazeos
cd wazeos

# Build WazeOS CLI
go build -o wazeos ./core/cmd/wazeos

# Verify installation
./wazeos --version
```

### Create Your First Tool

```bash
# 1. Create a new app
./wazeos app new my-tool

# 2. Edit the tool logic
cd apps/my-tool
# Edit src/lib.rs with your tool implementation

# 3. Build it
cd ../..
./wazeos app build my-tool

# 4. Install locally
./wazeos app install my-tool

# 5. Use it with Claude Desktop
# Add to ~/.config/claude/mcp_servers.json
```

## 📚 Example: Hello World Tool

```rust
// apps/my-tool/src/lib.rs
use serde_json::{json, Value};
use wazeos_app::{AppContext, AppResult, register_tool};
use wazeos_file::FileOps;  // Optional: ergonomic file operations

#[no_mangle]
pub extern "C" fn tool_main(ctx: &AppContext, args: Value) -> AppResult {
    let name = args["name"].as_str().unwrap_or("World");

    // Example: Read a file (if permitted)
    let data = ctx.read_file("/tmp/config.txt")?;

    Ok(json!({
        "greeting": format!("Hello, {}!", name),
        "config": data
    }))
}

register_tool!(tool_main);
```

### Declare Permissions

```toml
# apps/my-tool/wazeos.toml
[permissions]
file = ['/tmp/**']      # Can read/write /tmp files
shell = ['echo', 'date'] # Can run specific commands
http = ['api.example.com/**'] # Can access specific domains
```

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────┐
│         MCP Client (Claude Desktop)             │
└─────────────────────────────────────────────────┘
                    ↓ JSON-RPC 2.0
┌─────────────────────────────────────────────────┐
│           WazeOS MCP Server                     │
│  ┌──────────────────────────────────────────┐  │
│  │   App: my-tool.wasm                      │  │
│  │   - Isolated WASM sandbox                 │  │
│  │   - Permission-checked I/O                │  │
│  └──────────────────────────────────────────┘  │
└─────────────────────────────────────────────────┘
                    ↓ IO Bus
┌──────────────┬──────────────┬──────────────────┐
│ file-driver  │ shell-driver │ http-driver      │
│ (WASM)       │ (WASM)       │ (WASM)           │
└──────────────┴──────────────┴──────────────────┘
        ↓              ↓              ↓
┌──────────────┬──────────────┬──────────────────┐
│ native-file  │ native-shell │ native-http      │
│ (Go)         │ (Go)         │ (Go)             │
└──────────────┴──────────────┴──────────────────┘
        ↓              ↓              ↓
    Filesystem     OS Shell       Network
```

### Key Components

1. **Apps**: Your WASM tools that implement MCP functionality
2. **Drivers**: Extensible I/O layer (file, shell, HTTP, etc.)
3. **IO Bus**: Routes requests to appropriate drivers with permission checks
4. **MCP Server**: Exposes apps as MCP tools to Claude Desktop

## 🎨 Driver SDK Addons

WazeOS uses a **generic I/O architecture** where drivers provide SDK addons for ergonomic APIs:

### Core SDK (Generic)

```rust
use wazeos_app::AppContext;

// Low-level: Works with ANY driver
let resp = ctx.call("file:///tmp/data.txt", headers, body)?;
let resp = ctx.call("redis://localhost/GET/key", headers, body)?;
```

### Driver Addons (Ergonomic)

```rust
use wazeos_file::FileOps;   // File driver addon
use wazeos_shell::ShellOps; // Shell driver addon
use wazeos_http::HttpOps;   // HTTP driver addon

// High-level: Convenient methods
let content = ctx.read_file("/tmp/data.txt")?;
ctx.write_file("/tmp/output.txt", "Hello")?;
let output = ctx.shell_exec("date")?;
let data = ctx.http_get("https://api.example.com")?;
```

**Benefits**:
- Core SDK never needs updates for new drivers
- Each driver provides its own ergonomic API
- Apps can use raw calls for custom drivers
- Type-safe, self-documenting APIs

## 📦 Package & Distribution

### Creating Packages

```bash
# Build your app
./wazeos app build my-tool

# Create distributable package
./wazeos app package my-tool

# Output: my-tool-1.0.0.wazpkg (62 KB)
```

### Sharing Packages

```bash
# Via file
cp my-tool-1.0.0.wazpkg /shared/folder/

# Via web
scp my-tool-1.0.0.wazpkg user@example.com:/var/www/tools/

# Via GitHub Releases
# Attach as release asset
```

### Installing Packages

```bash
# From local file
./wazeos app install ./my-tool-1.0.0.wazpkg

# From URL (future)
./wazeos app install https://example.com/my-tool-1.0.0.wazpkg
```

**Package Format**: Standard tar.gz containing manifest, WASM binary, and metadata.

See [docs/PACKAGING_GUIDE.md](docs/PACKAGING_GUIDE.md) for details.

## 🛠️ Development Workflow

```bash
# 1. Create app
./wazeos app new awesome-tool

# 2. Develop
cd apps/awesome-tool
# Edit src/lib.rs
# Edit wazeos.toml (permissions)

# 3. Build
cd ../..
./wazeos app build awesome-tool

# 4. Test locally
./wazeos app install awesome-tool

# 5. Test with Claude
# Tool appears in Claude Desktop

# 6. Package for distribution
./wazeos app package awesome-tool

# 7. Share .wazpkg file
```

## 📖 Documentation

### User Guides
- **[Packaging Guide](docs/PACKAGING_GUIDE.md)** - Package and distribute apps
- **[Package Format](docs/PACKAGE_FORMAT.md)** - Technical specification
- **[Language Support](docs/LANGUAGE_SUPPORT.md)** - Multi-language WASM guide

### For AI Agents
- **[AGENTS.md](../AGENTS.md)** - Comprehensive guide for Claude and other AI agents
  - Philosophy: Documentation lives in code
  - Engineering principles learned from building WazeOS
  - Navigation strategies for token efficiency
  - Common tasks reference

### Architecture
- **[PRD](docs/PRD.md)** - Product requirements and vision
- **Architecture docs** - See inline code comments in:
  - [core/kernel/iobus/types.go](core/kernel/iobus/types.go) - Core types and runtime system
  - [core/kernel/iobus/iobus.go](core/kernel/iobus/iobus.go) - Registration and routing
  - [drivers/runtime/wasm/loader.go](../drivers/runtime/wasm/loader.go) - WASM driver contract

### SDK Reference
- **[App SDK (Rust)](core/sdk/rust/app/)** - Core SDK for apps
- **[Driver SDK (Rust)](core/sdk/rust/driver/)** - Core SDK for drivers
- **File Driver Example** - [drivers/file/src/lib.rs](../../drivers/file/src/lib.rs)
- **Shell Driver Example** - [drivers/shell/src/lib.rs](../../drivers/shell/src/lib.rs)
- **HTTP Driver Example** - [drivers/http/src/lib.rs](../../drivers/http/src/lib.rs)

## 🗂️ Project Structure

```
core/
├── cmd/wazeos/          # CLI tool
│   ├── app.go           # App management commands
│   ├── driver.go        # Driver management commands
│   ├── mcp.go           # MCP server
│   ├── dev.go           # Development utilities
│   └── ...
│
├── kernel/              # Core IO Bus implementation
│   └── iobus/           # Routing, sessions, permissions
│
├── sdk/                 # SDKs for building apps and drivers
│   ├── rust/            # Rust SDK
│   └── go/              # Go SDK
│
└── internal/            # Internal packages
    └── pkg/             # Package management

drivers/
├── native/              # Native driver implementations (Go)
│   ├── file/            # File system operations
│   ├── http/            # HTTP client
│   └── shell/           # Shell execution
│
└── runtime/             # Runtime drivers
    └── wasm/            # WASM runtime driver
│   ├── file/            # File driver + SDK addon
│   ├── shell/           # Shell driver + SDK addon
│   └── http/            # HTTP driver + SDK addon
│
├── sdk/                 # SDKs for building apps
│   └── rust/
│       └── wazeos-app/  # Core app SDK
│
├── apps/                # Example apps
│   └── test-tool/       # Comprehensive I/O test app
│
└── docs/                # Documentation
```

## ✅ Current Status

| Component | Status |
|-----------|--------|
| **Core** | |
| IO Bus & Routing | ✅ Complete |
| Permission System | ✅ Complete |
| WASM Runtime | ✅ Complete |
| **Drivers** | |
| File Operations | ✅ Complete |
| Shell Execution | ✅ Complete |
| HTTP Client | ✅ Complete |
| **SDKs** | |
| Core App SDK (Rust) | ✅ Complete |
| File Driver SDK | ✅ Complete |
| Shell Driver SDK | ✅ Complete |
| HTTP Driver SDK | ✅ Complete |
| **Tooling** | |
| CLI (app commands) | ✅ Complete |
| MCP Server | ✅ Complete |
| Package System (.wazpkg) | ✅ Complete |
| **Testing** | |
| I/O Integration Tests | ✅ Complete |
| Package Install/Distribute | ✅ Complete |

**Status**: Production-ready for building and distributing MCP tools! 🎉

## 🔄 Version 2 Improvements

v2 is a complete rewrite addressing v1 limitations:

| Aspect | v1 | v2 |
|--------|----|----|
| **I/O Architecture** | Hardcoded in SDK | Generic with driver addons |
| **Scalability** | SDK updates needed | Drivers self-contained |
| **Distribution** | Manual files | `.wazpkg` packages |
| **Ergonomics** | Low-level only | Both raw and helper APIs |
| **Testing** | Stub-based | Real end-to-end I/O |
| **Documentation** | Scattered .md files | Inline code comments + guides |
| **Language Support** | Rust only (implicit) | Any WASM language (explicit) |
| **Runtime System** | Static types | Dynamic registration |

## 🤝 Contributing

Contributions welcome! Areas of interest:
- **Additional drivers** (database, cloud services, Redis, etc.)
- **Language SDKs** - C, Go (TinyGo), AssemblyScript (see [LANGUAGE_SUPPORT.md](docs/LANGUAGE_SUPPORT.md))
- **Example apps** showcasing capabilities
- **Documentation** improvements (inline code comments preferred)
- **Performance** optimizations
- **Testing** for multi-language drivers

## 📝 License

MIT OR Apache-2.0

---

**Version**: 2.0.0
**Status**: Production Ready
**Last Updated**: 2026-05-08

**Getting Started**: See [Quick Start](#-quick-start) above
**Questions?**: Check [docs/](docs/) or open an issue
