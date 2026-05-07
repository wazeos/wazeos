# WazeOS App SDK for Rust

Rust SDK for building WazeOS WASM applications and MCP tools. This SDK provides high-level APIs for creating applications that run in the WazeOS kernel.

## Installation

Add this to your `Cargo.toml`:

```toml
[dependencies]
wazeos-app = "0.1"
serde_json = "1.0"

[lib]
crate-type = ["cdylib"]
```

## Quick Start

### MCP Tool Example

The simplest way to build a WazeOS app is as an MCP tool:

```rust
use wazeos_app::{MCPToolHandler, Context, run_mcp_tool};
use serde_json::{json, Value};

struct GreetingTool;

impl MCPToolHandler for GreetingTool {
    fn handle(&self, ctx: &Context, input: Value) -> Result<Value, Box<dyn std::error::Error>> {
        // Parse input
        let name = input["name"].as_str().unwrap_or("World");

        // Log the request
        ctx.info(&format!("Greeting {}", name));

        // Return JSON response
        Ok(json!({
            "greeting": format!("Hello, {}!", name),
            "timestamp": "2024-01-01T00:00:00Z"
        }))
    }
}

fn main() {
    run_mcp_tool(&GreetingTool);
}
```

### CLI App Example

```rust
use wazeos_app::{CLIHandler, Context, Response, run_cli};

struct MyCLI;

impl CLIHandler for MyCLI {
    fn run(&self, ctx: &Context, args: Vec<String>) -> Result<Response, Box<dyn std::error::Error>> {
        if args.is_empty() {
            return Ok(Response::bad_request("No arguments provided"));
        }

        let message = format!("Received {} arguments", args.len());
        Ok(Response::success_string(message))
    }
}

fn main() {
    run_cli(&MyCLI);
}
```

### Request/Response App Example

```rust
use wazeos_app::{RequestHandler, Context, Request, Response, run_handler};

struct APIHandler;

impl RequestHandler for APIHandler {
    fn handle(&self, ctx: &Context, req: Request) -> Result<Response, Box<dyn std::error::Error>> {
        match req.method.as_str() {
            "GET" => {
                Ok(Response::success_json(&serde_json::json!({
                    "path": req.path,
                    "method": req.method
                }))?)
            }
            _ => Ok(Response::error(405, "Method not allowed")),
        }
    }
}

fn main() {
    run_handler(&APIHandler);
}
```

## Building

Compile your app to WASM with the `wasm32-wasi` target:

```bash
rustup target add wasm32-wasi
cargo build --target wasm32-wasi --release
```

The compiled WASM binary will be at `target/wasm32-wasi/release/your_app.wasm`.

## Core Types

### `Context`

Provides access to execution metadata and permissions:

```rust
pub struct Context {
    pub request_id: String,            // Unique request identifier
    pub trace_id: String,              // Distributed tracing ID
    pub principal: String,             // Authenticated user (e.g., "user:alice")
    pub permissions: Option<PermissionContext>, // URI-based access control
    pub metadata: HashMap<String, String>,      // Additional metadata
}
```

Methods:
- `Context::from_env()` - Creates context from environment variables
- `.has_permission(uri_pattern, required_perms)` - Check if a URI is accessible
- `.info(message)` / `.warn(message)` / `.error(message)` - Structured logging

### `Response`

Represents app output:

```rust
pub struct Response {
    pub status_code: i32,                 // HTTP status code
    pub headers: HashMap<String, String>, // Response headers
    pub body: Vec<u8>,                    // Response body
    pub exit_code: i32,                   // Process exit code
}
```

Helper methods:
- `Response::success(body)` - Create 200 response
- `Response::success_string(message)` - Create text response
- `Response::success_json(value)` - Create JSON response
- `Response::error(status_code, message)` - Create error response
- `Response::bad_request(message)` - Create 400 response
- `Response::forbidden(message)` - Create 403 response
- `Response::not_found(message)` - Create 404 response
- `Response::internal_error(message)` - Create 500 response
- `.with_header(key, value)` - Add a header

### `Request`

HTTP-style request (for RequestHandler):

```rust
pub struct Request {
    pub method: String,                   // HTTP method
    pub path: String,                     // Request path
    pub headers: HashMap<String, String>, // Request headers
    pub body: Vec<u8>,                    // Request body
}
```

## Handler Traits

### `MCPToolHandler`

The simplest handler for building MCP tools:

```rust
pub trait MCPToolHandler {
    fn handle(&self, ctx: &Context, input: Value) -> Result<Value, Box<dyn Error>>;
}
```

Use with `run_mcp_tool(handler)`.

### `CLIHandler`

For command-line style apps:

```rust
pub trait CLIHandler {
    fn run(&self, ctx: &Context, args: Vec<String>) -> Result<Response, Box<dyn Error>>;
}
```

Use with `run_cli(handler)`.

### `RequestHandler`

For HTTP-style request/response apps:

```rust
pub trait RequestHandler {
    fn handle(&self, ctx: &Context, req: Request) -> Result<Response, Box<dyn Error>>;
}
```

Use with `run_handler(handler)`.

## Logging

Use the context for structured logging:

```rust
fn handle(&self, ctx: &Context, input: Value) -> Result<Value, Box<dyn Error>> {
    ctx.info("Processing request");
    ctx.warn("This is a warning");
    ctx.error("Something went wrong");

    // Custom logging
    ctx.log("DEBUG", "Detailed debug info");

    Ok(json!({"status": "ok"}))
}
```

Logs are written to stderr with request context automatically included.

## Permissions

Check permissions before performing operations:

```rust
fn handle(&self, ctx: &Context, input: Value) -> Result<Value, Box<dyn Error>> {
    // Check if we can read from /tmp
    if !ctx.has_permission("file:///tmp/*", &["read"]) {
        return Err("Permission denied".into());
    }

    // Proceed with file operation...
    Ok(json!({"status": "success"}))
}
```

## Error Handling

Always return results, not panics:

```rust
fn handle(&self, ctx: &Context, input: Value) -> Result<Value, Box<dyn Error>> {
    // Parse input with proper error handling
    let name = input["name"]
        .as_str()
        .ok_or("name field is required")?;

    // Validate input
    if name.is_empty() {
        return Err("name cannot be empty".into());
    }

    Ok(json!({
        "greeting": format!("Hello, {}!", name)
    }))
}
```

## Communication Protocol

Apps communicate with the WazeOS kernel using JSON over stdin/stdout:

1. Kernel writes JSON request to app's stdin
2. App processes the request
3. App writes JSON response to stdout
4. Kernel reads the response

This protocol is language-agnostic and works across Go and Rust implementations.

## Examples

See the `examples/` directory for more complete examples:
- `examples/hello_world.rs` - Simple MCP tool
- `examples/cli_tool.rs` - Command-line app
- `examples/api_handler.rs` - HTTP-style handler

## License

MIT OR Apache-2.0
