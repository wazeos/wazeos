# WazeOS Driver SDK for Rust

Rust SDK for building WazeOS WASM drivers. This SDK provides types and utilities for creating resource drivers and authentication drivers that run in the WazeOS kernel.

## Installation

Add this to your `Cargo.toml`:

```toml
[dependencies]
wazeos-driver = "0.1"

[lib]
crate-type = ["cdylib"]
```

## Quick Start

### Resource Driver Example

```rust
use wazeos_driver::{ResourceCall, ResourceResult, ResourceHandler, serve_resource_once};

struct FileDriver;

impl ResourceHandler for FileDriver {
    fn handle_call(&self, call: &ResourceCall) -> Result<ResourceResult, Box<dyn std::error::Error>> {
        // Check permissions
        if !call.permissions.contains(&"read".to_string()) {
            return Ok(ResourceResult::error(403, "Permission denied"));
        }

        // Process the file:// URI
        let content = format!("Hello from {}", call.uri);
        Ok(ResourceResult::success(200, content.into_bytes()))
    }
}

fn main() {
    serve_resource_once(&FileDriver);
}
```

### Authentication Driver Example

```rust
use wazeos_driver::{AuthPayload, AuthResult, AuthHandler, serve_auth_once};

struct TokenAuthDriver;

impl AuthHandler for TokenAuthDriver {
    fn authenticate(&self, payload: &AuthPayload) -> Result<AuthResult, Box<dyn std::error::Error>> {
        // Check for Authorization header
        if let Some(token) = payload.headers.get("Authorization") {
            if token.starts_with("Bearer ") {
                let username = validate_token(&token[7..]);
                return Ok(AuthResult::success(format!("user:{}", username)));
            }
        }

        // Abstain if we don't handle this auth type
        Ok(AuthResult::abstain())
    }
}

fn main() {
    serve_auth_once(&TokenAuthDriver);
}
```

## Building

Compile your driver to WASM with the `wasm32-wasi` target:

```bash
rustup target add wasm32-wasi
cargo build --target wasm32-wasi --release
```

The compiled WASM binary will be at `target/wasm32-wasi/release/your_driver.wasm`.

## Core Types

### `ResourceCall`

Represents an incoming resource request:

```rust
pub struct ResourceCall {
    pub uri: String,                      // e.g., "file:///data.txt"
    pub headers: HashMap<String, String>, // HTTP-style headers
    pub body: Vec<u8>,                    // Request body
    pub permissions: Vec<String>,         // Available permissions
}
```

### `ResourceResult`

Represents the response from a resource call:

```rust
pub struct ResourceResult {
    pub status_code: i32,                 // HTTP status code (200, 404, etc.)
    pub headers: HashMap<String, String>, // Response headers
    pub body: Vec<u8>,                    // Response body
    pub error: Option<String>,            // Error message if failed
}
```

Helper methods:
- `ResourceResult::success(status_code, body)` - Create success response
- `ResourceResult::error(status_code, message)` - Create error response
- `.with_header(key, value)` - Add a header to the result

### `AuthPayload`

Authentication request input:

```rust
pub struct AuthPayload {
    pub headers: HashMap<String, String>, // Request headers
    pub body: Vec<u8>,                    // Request body
}
```

### `AuthResult`

Authentication response:

```rust
pub struct AuthResult {
    pub principal: Option<String>,  // e.g., "user:alice"
    pub error: Option<String>,      // Error or "abstain"
}
```

Helper methods:
- `AuthResult::success(principal)` - Create success result
- `AuthResult::error(message)` - Create error result
- `AuthResult::abstain()` - Indicate this driver doesn't handle this auth type
- `.is_abstain()` - Check if result is an abstain

## Serve Functions

### `serve_resource_once`

Handles a single resource call from stdin and exits. Recommended for most drivers.

```rust
fn main() {
    serve_resource_once(&MyDriver);
}
```

### `serve_auth_once`

Handles a single auth request from stdin and exits.

```rust
fn main() {
    serve_auth_once(&MyAuthDriver);
}
```

### `serve_resource_stdio`

Runs in a loop, handling multiple resource calls. Each call is one JSON line on stdin.

```rust
fn main() {
    serve_resource_stdio(&MyDriver);
}
```

### `serve_auth_stdio`

Runs in a loop, handling multiple auth requests.

```rust
fn main() {
    serve_auth_stdio(&MyAuthDriver);
}
```

## Communication Protocol

Drivers communicate with the WazeOS kernel using JSON over stdin/stdout:

1. Kernel writes JSON request to driver's stdin
2. Driver processes the request
3. Driver writes JSON response to stdout
4. Kernel reads the response

This protocol is language-agnostic and works across Go and Rust implementations.

## Error Handling

Always return a `ResourceResult` or `AuthResult`, even on errors:

```rust
fn handle_call(&self, call: &ResourceCall) -> Result<ResourceResult, Box<dyn std::error::Error>> {
    // Don't return Err() - return an error result instead
    if call.uri.is_empty() {
        return Ok(ResourceResult::error(400, "URI is required"));
    }

    // Process call...
    Ok(ResourceResult::success(200, data))
}
```

## License

MIT OR Apache-2.0
