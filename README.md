# WazeOS

**WebAssembly-native Function-as-a-Service (FaaS) Platform**

WazeOS is a lightweight, secure execution platform for WebAssembly applications with automatic credential injection, sophisticated I/O routing, and comprehensive audit logging. It provides a microkernel architecture where functionality is extended through pluggable drivers and user applications run in isolated WASM sandboxes.

## Features

- **WebAssembly Runtime**: Execute WASM applications with TinyGo support
- **Sophisticated I/O Routing**: URI pattern matching with specificity-based driver selection
- **Credential Injection**: Automatic, secure credential management for applications
- **Driver Architecture**: Extensible system with pluggable drivers for I/O, security, and runtime
- **MCP Tool Support**: Model Context Protocol integration for AI-powered workflows
- **REST Management API**: Full-featured HTTP API for administration
- **Comprehensive Audit Logging**: Complete visibility into all I/O operations
- **Authorization Engine**: Fine-grained permission control for resource access

## Quick Start

### Prerequisites

- Go 1.21 or higher
- TinyGo (for building WASM applications)
- Make (optional, for convenience commands)

### Installation

```bash
# Clone the repository
git clone https://github.com/wazeos/wazeos.git
cd wazeos

# Build the binary
make build

# Or install directly to /usr/local/bin
make install
```

### Running the Server

```bash
# Start the server on default port (8081)
./bin/wazeos server start

# Or run directly with go
go run ./cmd/wazeos server start

# Custom port
./bin/wazeos server start --addr :9090

# Quiet mode (suppress non-error output)
./bin/wazeos server start --quiet
```

### Basic Commands

```bash
# List installed applications
wazeos apps list

# Install an application
wazeos apps install ./myapp.wasm --name myapp

# Manage secrets
wazeos secrets set api_key=sk_test_123

# View available drivers
wazeos drivers list
```

## Architecture

WazeOS follows a microkernel architecture with clear separation between kernel services, drivers, and user applications.

```
┌─────────────────────────────────────────────────┐
│              User Applications                  │
│            (WebAssembly Modules)                │
└─────────────────┬───────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────┐
│                 SDK Layer                       │
│  (ctx.IO, ctx.Secret, Error Handling)           │
└─────────────────┬───────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────┐
│             Kernel Services                     │
├─────────────────────────────────────────────────┤
│  • Authorization (Authz Injection Layer)        │
│  • Authentication (AllowAll/Custom)             │
│  • Credential Management                        │
│  • Driver Policy Registry                       │
└─────────────────┬───────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────┐
│              I/O Bus (io.bus)                   │
│  Sophisticated URI Pattern Matching Router      │
│  • Pattern scoring & best match selection       │
│  • Wildcard support (host/path)                 │
│  • Audit logging for all operations             │
└─────────────────┬───────────────────────────────┘
                  │
         ┌────────┼────────┐
         ▼        ▼        ▼
    ┌────────┬────────┬────────┐
    │  I/O   │Runtime │Security│
    │Drivers │Drivers │Drivers │
    └────────┴────────┴────────┘
```

### Core Components

#### 1. **Kernel** (`internal/kernel/`)
The kernel manages driver lifecycle, policy enforcement, and provides core services:
- **Driver Policy Registry**: Enforces cardinality and requirement rules for driver classes
- **Package Manager**: Manages WASM application installation and storage
- **Runtime Coordinator**: Handles application execution and lifecycle

#### 2. **I/O Bus** (`internal/drivers/io/bus/`)
The heart of WazeOS routing, using sophisticated pattern matching:
- Routes resource calls (`file://`, `https://`, `fn://`, etc.) to appropriate drivers
- Scores patterns by specificity (exact matches > wildcards)
- Supports host prefix wildcards (`*.example.com`) and path suffix wildcards (`/data/*`)
- Provides complete audit logging of all I/O operations

**Pattern Matching Example:**
```go
// Register drivers with different specificity
file://*/*                      // Generic fallback (score: 0)
file:///data/*                  // Specific directory (score: 16)
file:///data/important.txt      // Exact file (score: 235)

// URI: file:///data/important.txt
// → Routes to exact match driver (highest score)
```

See [docs/pattern-matching.md](docs/pattern-matching.md) for complete details.

#### 3. **Drivers**
Pluggable components that implement system functionality:

**I/O Drivers** (`internal/drivers/io/`):
- HTTP driver (name: `wazeos/http`, class: `io.request`) - HTTP request handling
- I/O bus (class: `io.bus`) - Core routing and pattern matching
- Function driver (name: `wazeos/fn`, class: `io.resource`) - App-to-app calls

**Runtime Drivers** (`internal/drivers/kernel/runtime/`):
- Exec driver (name: `wazeos/exec`, class: `runtime.exec`) - WebAssembly execution engine with TinyGo support

**Security Drivers** (`internal/security/`):
- `security.authz`: Authorization and credential injection
- `security.secrets`: Secure secrets storage and retrieval

#### 4. **SDK** (`sdk/`)
Go SDK for building WazeOS applications:
- **Context API**: `ctx.IO()`, `ctx.Secret()`, `ctx.Log()`
- **Error Handling**: Structured errors with codes and HTTP status
- **Type-safe I/O**: Request/response handling with automatic marshaling

#### 5. **Management API** (`internal/api/`)
REST API for system administration:
- `GET /api/health` - Health check
- `GET /api/apps` - List applications
- `POST /api/apps` - Install application
- `DELETE /api/apps/{name}` - Remove application
- `GET /api/secrets` - List secrets
- `POST /api/secrets` - Store secret
- `POST /mcp` - MCP tool invocation endpoint

## Driver Classes and Policies

WazeOS enforces driver policies to ensure system integrity. Each driver class has rules about how many instances can exist and whether they're required.

| Driver Class | Cardinality | Requirement | Description |
|--------------|-------------|-------------|-------------|
| `io.bus` | One | Required | Core I/O routing bus - exactly one required for system operation |
| `io.resource` | Many | Optional | Resource drivers for external I/O - multiple allowed, not required |
| `io.request` | Many | Required | Request drivers for inbound requests - at least one required |
| `runtime.exec` | One | Required | Runtime execution engine - exactly one required |
| `pkg.install` | One | Required | Package manager - exactly one required |
| `security.authz` | One | Required | Authorization engine - exactly one required |

### Policy Enforcement

**Cardinality:**
- **One**: Exactly one driver instance allowed (singletons)
- **Many**: Multiple driver instances allowed (extensible)

**Requirement:**
- **Required**: System cannot start without this driver class
- **Optional**: Driver class is optional but recommended

**Example Violations:**
```bash
# ERROR: Multiple io.bus drivers (cardinality violation)
RegisterDriver(busDriver1)  // class: "io.bus"
RegisterDriver(busDriver2)  // class: "io.bus" ❌ Violates "One" cardinality

# ERROR: No runtime.exec driver (requirement violation)
# System won't start if required drivers are missing ❌

# OK: Multiple io.resource drivers (cardinality: Many)
RegisterDriver(s3Driver)   // class: "io.resource", name: "wazeos/s3"
RegisterDriver(gcsDriver)  // class: "io.resource", name: "wazeos/gcs" ✓ Allowed
```

## URI Pattern Matching

The I/O bus uses sophisticated pattern matching to route URIs to drivers. Patterns support wildcards and are scored by specificity.

### Pattern Syntax

- **Scheme**: Must match exactly (`file://`, `https://`, `fn://`)
- **Host wildcards**: `*.domain.com` matches any subdomain
- **Path wildcards**: `/path/to/*` matches any path with that prefix
- **Full wildcards**: `*` or `/*` matches everything

### Scoring System

Higher scores indicate more specific matches:

**Host scoring:**
- Exact match: `(segments × 15) + length + 50` points
  - Example: `api.example.com` = `(3 × 15) + 15 + 50 = 110`
- Prefix wildcard: `(suffix segments × 15)` points
  - Example: `*.example.com` = `(2 × 15) = 30`
- Full wildcard: `0` points

**Path scoring:**
- Exact match: `(segments × 10) + length + 100` points
  - Example: `/data/file.txt` = `(2 × 10) + 15 + 100 = 135`
- Suffix wildcard: `(prefix segments × 10) + prefix length` points
  - Example: `/data/*` = `(1 × 10) + 6 = 16`
- Full wildcard: `0` points

### Example: HTTP API Routing

```
Drivers registered:
1. https://*/*                    (generic HTTPS driver)
2. https://*.example.com/*        (domain wildcard driver)
3. https://api.example.com/*      (specific host driver)
4. https://api.example.com/v1/*   (specific API version driver)

URI: https://api.example.com/v1/users
├─ Driver 1 matches: score 0 (full wildcards)
├─ Driver 2 matches: score 30 (2 host segments × 15)
├─ Driver 3 matches: score 110 (exact host: 3×15 + 15 chars + 50)
└─ Driver 4 matches: score 133 (exact host + path prefix) ✓ SELECTED
```

See [docs/pattern-matching.md](docs/pattern-matching.md) for complete documentation.

## Building WASM Applications

WazeOS applications are built using TinyGo and the WazeOS SDK.

### Example Application

```go
package main

import (
    "github.com/wazeos/wazeos/sdk/app"
)

func main() {
    app.HandleRequest(handler)
}

func handler(ctx *app.Context) error {
    // Read from secret store
    apiKey, err := ctx.Secret("api_key")
    if err != nil {
        return err
    }

    // Make HTTP request with injected credentials
    result, err := ctx.IO("https://api.example.com/data").Call(map[string]interface{}{
        "method": "GET",
        "headers": map[string]string{
            "Authorization": "Bearer " + apiKey,
        },
    })
    if err != nil {
        return err
    }

    // Return response
    return ctx.JSON(200, result)
}
```

### Build and Install

```bash
# Build WASM module
tinygo build -o myapp.wasm -target=wasi ./main.go

# Install to WazeOS
wazeos apps install myapp.wasm --name myapp

# Invoke via HTTP
curl http://localhost:8081/invoke/myapp
```

## Security Model

WazeOS provides defense-in-depth security:

### 1. **WASM Sandbox**
- Applications run in isolated WebAssembly environments
- No direct file system or network access
- All I/O mediated through SDK

### 2. **Authorization Layer**
- Credential injection based on URI schemes
- Fine-grained permission control
- Automatic credential attachment for `secret://` and `fn://` URIs

### 3. **Secrets Management**
- Encrypted secret storage
- Never exposed to application code directly
- Retrieved via secure `ctx.Secret()` API

### 4. **Audit Logging**
- Complete visibility into all I/O operations
- Request/response logging with sanitization
- Pattern match scoring and routing decisions

## Development

### Running Tests

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run specific package
make test-pkg PKG=./internal/drivers/io/bus

# Watch mode (requires entr)
make watch
```

### Project Structure

```
wazeos/
├── cmd/wazeos/              # CLI application
│   ├── commands/            # Command implementations
│   └── server.go            # Server startup logic
├── internal/                # Internal packages
│   ├── api/                 # Management API
│   ├── drivers/             # Driver implementations
│   │   ├── io/              # I/O drivers (bus, request)
│   │   └── kernel/          # Kernel drivers (runtime, pkg)
│   ├── kernel/              # Kernel services
│   ├── security/            # Security components
│   └── types/               # Shared types and interfaces
├── sdk/                     # SDK for building applications
│   ├── app/                 # Application context and helpers
│   └── driver/              # Driver interface for Go drivers
├── docs/                    # Documentation
└── Makefile                 # Build automation
```

### Adding a New Driver

1. **Define the driver interface** in `internal/types/`
2. **Implement the driver** in appropriate directory
3. **Register URI patterns** via `Patterns()` method
4. **Register with io.bus** in `cmd/wazeos/server.go`
5. **Add tests** for pattern matching and functionality

Example:
```go
type MyDriver struct{}

// Name returns the driver name in author/name format
func (d *MyDriver) Name() string {
    return "mycompany/mydriver"
}

// Patterns returns URI patterns this driver handles
func (d *MyDriver) Patterns() []string {
    return []string{"myscheme://*/*"}
}

// HandleCall processes resource calls
func (d *MyDriver) HandleCall(ctx context.Context, call *types.ResourceCall) (*types.ResourceResult, error) {
    // Implementation
}
```

Note: The driver class (e.g., "io.resource") is specified in the metadata.json file when packaging the driver, not in the Name() method.

## Configuration

WazeOS can be configured via:

1. **Command-line flags**: `--addr`, `--data-path`, `--quiet`, `--verbose`
2. **Environment variables**: Prefix with `WAZEOS_` (e.g., `WAZEOS_DATA_PATH`)
3. **Config file**: `~/.wazeos/config.yaml` or via `--config`

Example config:
```yaml
server:
  addr: ":8081"

data_path: "/var/lib/wazeos/data"

output:
  format: "table"  # or "json"
  color: true
```

## API Documentation

### Health Check
```bash
GET /api/health
```

### Application Management
```bash
# List applications
GET /api/apps

# Install application
POST /api/apps
Content-Type: multipart/form-data
{
  "name": "myapp",
  "file": <wasm-binary>
}

# Remove application
DELETE /api/apps/{name}
```

### Secrets Management
```bash
# List secrets
GET /api/secrets

# Store secret
POST /api/secrets
Content-Type: application/json
{
  "key": "api_key",
  "value": "sk_test_123"
}

# Delete secret
DELETE /api/secrets/{key}
```

### MCP Tool Invocation
```bash
POST /mcp
Content-Type: application/json
{
  "method": "tools/call",
  "params": {
    "name": "tool_name",
    "arguments": {}
  }
}
```

## Performance

- **O(n) URI routing**: Linear scan of registered patterns with scoring
- **Zero-copy I/O**: Efficient data handling through byte slices
- **Concurrent execution**: Thread-safe driver registration and lookup
- **Lightweight WASM**: TinyGo produces compact binaries (~100KB typical)

## Roadmap

- [ ] Redis-backed io.bus for distributed routing
- [ ] More I/O drivers (S3, PostgreSQL, Redis)
- [ ] Observability integration (OpenTelemetry)
- [ ] Multi-tenancy support
- [ ] Hot reload for applications
- [ ] CLI improvements (interactive mode)

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass (`make test`)
5. Submit a pull request

## License

[License information to be added]

## Support

- **Documentation**: [docs/](docs/)
- **Issues**: https://github.com/wazeos/wazeos/issues
- **SDK Documentation**: [sdk/README.md](sdk/README.md)

## Acknowledgments

Built with:
- [TinyGo](https://tinygo.org/) - Go compiler for WebAssembly
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Viper](https://github.com/spf13/viper) - Configuration management
