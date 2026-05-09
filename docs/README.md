# WazeOS v2 Documentation

Complete documentation for building, deploying, and distributing WazeOS apps.

## 📖 Quick Links

- **[Main README](../README.md)** - Start here for overview and quick start
- **[Packaging Guide](PACKAGING_GUIDE.md)** - How to package and distribute apps
- **[Claude Desktop Integration](CLAUDE_DESKTOP.md)** - Using WazeOS with Claude

## 🏗️ Architecture & Design

### Core Design Documents

- **[PRD (Product Requirements)](PRD.md)**
  - High-level vision and architecture
  - Key features and capabilities
  - Design goals and trade-offs

- **[Architecture Overview](ARCHITECTURE.md)**
  - WASM-first layered architecture
  - Double sandboxing security model
  - Request flow and performance characteristics
  - Design decisions and rationale

- **[Directory Structure](STRUCTURE.md)**
  - Complete directory organization
  - Component locations and purposes
  - Navigation guide for developers
  - Installation locations

- **[RFC-001: Driver Architecture](RFC-001-driver-architecture.md)**
  - Driver class taxonomy (io.connect, io.listen, runtime.*, kernel.*)
  - URI pattern matching and routing
  - Driver lifecycle and capabilities

- **[RFC-002: Handle System](RFC-002-handle-system.md)**
  - Session-based state management
  - Handle lifecycle and operations
  - Memory efficiency for large resources

- **[RFCs Overview](RFCs.md)**
  - Index of all RFC documents
  - Status and implementation tracking

### Technical Specifications

- **[Package Format](PACKAGE_FORMAT.md)**
  - .wazpkg file structure (tar.gz)
  - Metadata and validation
  - Security considerations

## 📚 User Guides

### Getting Started

1. **Installation** - See [main README](../README.md#-quick-start)
2. **Create Your First App** - See [main README](../README.md#-example-hello-world-tool)
3. **Permissions** - Declare in `wazeos.toml`
4. **Build & Install** - `wazeos app build` and `wazeos app install`

### Distribution

- **[Packaging Guide](PACKAGING_GUIDE.md)** - Complete guide to packaging apps
  - Creating `.wazpkg` files
  - Distribution methods (file, web, GitHub)
  - Installation process
  - Best practices and checklists

### Integration

- **[Claude Desktop](CLAUDE_DESKTOP.md)** - Integrate with Claude Desktop
  - MCP server configuration
  - Tool discovery and invocation
  - Troubleshooting

## 🔧 Developer Reference

### SDK Documentation

Located in the SDK directories with inline documentation:

- **[Core App SDK](../sdk/rust/app/src/lib.rs)**
  - `AppContext` - Application context
  - `call()` - Generic I/O method
  - `register_tool!()` - Tool registration macro

- **[File Driver SDK](../drivers-wasm/file/sdk/rust/src/lib.rs)**
  - `FileOps` trait
  - `read_file()`, `write_file()`
  - `read_file_bytes()`, `write_file_bytes()`

- **[Shell Driver SDK](../drivers-wasm/shell/sdk/rust/src/lib.rs)**
  - `ShellOps` trait
  - `shell_exec()`, `shell_exec_bytes()`

- **[HTTP Driver SDK](../drivers-wasm/http/sdk/rust/src/lib.rs)**
  - `HttpOps` trait
  - `http_get()`, `http_post()`
  - `http_get_bytes()`, `http_post_bytes()`

### CLI Reference

```bash
# App management
wazeos app new <name>          # Create new app
wazeos app build <name>        # Build WASM binary
wazeos app install <name>      # Install locally
wazeos app package <name>      # Create .wazpkg
wazeos app list                # List installed apps
wazeos app uninstall <name>    # Remove app

# MCP Server
wazeos mcp server              # Run MCP server
wazeos mcp install             # Install to Claude Desktop

# Driver management (future)
wazeos driver install <name>
wazeos driver list
```

## 📂 Documentation Structure

```
docs/
├── README.md (this file)      # Documentation index
│
├── User Guides/
│   ├── PACKAGING_GUIDE.md     # Packaging and distribution
│   └── CLAUDE_DESKTOP.md      # Claude Desktop integration
│
├── Technical Specs/
│   ├── ARCHITECTURE.md        # Layered architecture design
│   ├── STRUCTURE.md           # Directory organization
│   ├── PACKAGE_FORMAT.md      # .wazpkg specification
│   ├── PRD.md                 # Product requirements
│   ├── RFC-001-*.md           # Driver architecture
│   ├── RFC-002-*.md           # Handle system
│   └── RFCs.md                # RFC index
│
└── archive/                   # Historical documents
    ├── VERSION_MIGRATION.md   # v1 to v2 migration guide (outdated)
    ├── IO_IMPLEMENTATION_COMPLETE.md
    ├── IMPLEMENTATION_STATUS.md
    ├── IMPLEMENTATION_PLAN.md
    ├── FINAL_STATUS.md
    └── GETTING_STARTED.md
```

## 🎯 Documentation by Use Case

### I Want To...

**Build My First App**
1. Read [Quick Start](../README.md#-quick-start)
2. Follow [Example: Hello World](../README.md#-example-hello-world-tool)
3. Check [SDK Reference](#sdk-documentation)

**Distribute My App**
1. Read [Packaging Guide](PACKAGING_GUIDE.md)
2. Check [Package Format](PACKAGE_FORMAT.md)
3. Follow [Release Checklist](PACKAGING_GUIDE.md#release-checklist)

**Integrate with Claude Desktop**
1. Read [Claude Desktop Guide](CLAUDE_DESKTOP.md)
2. Run `wazeos mcp install`
3. Restart Claude Desktop

**Understand the Architecture**
1. Read [Architecture Overview](ARCHITECTURE.md) for layered design
2. Read [Directory Structure](STRUCTURE.md) for organization
3. Read [PRD](PRD.md) for high-level vision
4. Read [RFC-001](RFC-001-driver-architecture.md) for driver system
5. Read [RFC-002](RFC-002-handle-system.md) for session management

**Create a Custom Driver**
1. Read [RFC-001](RFC-001-driver-architecture.md) for driver classes
2. Check existing drivers in `../drivers/io/` (WASM) and `../drivers/runtime/` (Go)
3. Follow patterns in [Architecture Overview](ARCHITECTURE.md)
4. Implement driver interface and SDK addon

## 🔄 Document Status

| Document | Status | Last Updated |
|----------|--------|--------------|
| Main README | ✅ Current | 2026-05-08 |
| Architecture Overview | ✅ Current | 2026-05-08 |
| Directory Structure | ✅ Current | 2026-05-08 |
| Packaging Guide | ✅ Current | 2026-05-08 |
| Package Format | ✅ Current | 2026-05-08 |
| Claude Desktop | ✅ Current | (varies) |
| PRD | ✅ Current | 2026-05-08 |
| RFC-001 | ✅ Current | 2026-05-08 |
| RFC-002 | ✅ Current | 2026-05-08 |

## 🤝 Contributing to Docs

Documentation improvements are always welcome! Please:

1. Keep documentation accurate and up-to-date
2. Use clear, concise language
3. Include code examples where helpful
4. Update this index when adding new docs
5. Archive outdated docs to `archive/` rather than deleting

## 📝 Documentation Guidelines

- **User guides**: Focus on "how to" with step-by-step instructions
- **Technical specs**: Detailed specifications with examples
- **Architecture docs**: Explain "why" decisions were made
- **Reference docs**: Comprehensive API documentation with examples

---

**Need Help?**
- Check the [main README](../README.md)
- Read relevant guide above
- Check [GitHub Issues](https://github.com/wazeos/wazeos/issues)
