//! WazeOS App SDK for Rust
//!
//! This SDK provides types and utilities for building WazeOS WASM applications and MCP tools in Rust.
//! Apps communicate with the WazeOS kernel using a JSON-based stdin/stdout protocol.
//!
//! # Example: MCP Tool
//!
//! ```rust,no_run
//! use wazeos_app::{MCPToolHandler, Context, run_mcp_tool};
//! use serde_json::Value;
//!
//! struct GreetingTool;
//!
//! impl MCPToolHandler for GreetingTool {
//!     fn handle(&self, _ctx: &Context, input: Value) -> Result<Value, Box<dyn std::error::Error>> {
//!         let name = input["name"].as_str().unwrap_or("World");
//!         Ok(serde_json::json!({
//!             "greeting": format!("Hello, {}!", name)
//!         }))
//!     }
//! }
//!
//! fn main() {
//!     run_mcp_tool(&GreetingTool);
//! }
//! ```

use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::HashMap;
use std::env;
use std::error::Error;
use std::io::{self, Write};

// ============================================================================
// Core Types
// ============================================================================

/// Execution context providing access to request metadata and permissions.
#[derive(Debug, Clone)]
pub struct Context {
    /// Unique request identifier
    pub request_id: String,

    /// Distributed tracing ID
    pub trace_id: String,

    /// Authenticated user/service (e.g., "user:alice")
    pub principal: String,

    /// URI-based access control permissions
    pub permissions: Option<PermissionContext>,

    /// Additional metadata from environment
    pub metadata: HashMap<String, String>,
}

impl Context {
    /// Creates a context from environment variables set by the WazeOS runtime.
    pub fn from_env() -> Self {
        let request_id = env::var("WAZEOS_REQUEST_ID").unwrap_or_default();
        let trace_id = env::var("WAZEOS_TRACE_ID").unwrap_or_default();
        let principal = env::var("WAZEOS_PRINCIPAL").unwrap_or_default();

        // Parse permissions if present
        let permissions = env::var("WAZEOS_PERMISSIONS")
            .ok()
            .and_then(|json| serde_json::from_str(&json).ok());

        // Parse metadata from WAZEOS_METADATA_* environment variables
        let metadata = env::vars()
            .filter(|(k, _)| k.starts_with("WAZEOS_METADATA_"))
            .map(|(k, v)| (k.trim_start_matches("WAZEOS_METADATA_").to_string(), v))
            .collect();

        Self {
            request_id,
            trace_id,
            principal,
            permissions,
            metadata,
        }
    }

    /// Checks if a URI pattern is accessible with the given permissions.
    ///
    /// # Example
    ///
    /// ```rust
    /// # use wazeos_app::Context;
    /// # let ctx = Context::from_env();
    /// if ctx.has_permission("file:///tmp/*", &["read", "write"]) {
    ///     // Can read and write files in /tmp
    /// }
    /// ```
    pub fn has_permission(&self, uri_pattern: &str, required_perms: &[&str]) -> bool {
        let Some(ref perms) = self.permissions else {
            return false;
        };

        for entry in &perms.entries {
            if matches_pattern(&entry.uri_pattern, uri_pattern) {
                let has_all = required_perms.iter().all(|req| {
                    entry.permissions.iter().any(|p| p == req)
                });
                if has_all {
                    return true;
                }
            }
        }

        false
    }

    /// Logs a message to stderr with structured metadata.
    pub fn log(&self, level: &str, message: &str) {
        eprintln!(
            "[{}] [{}] [req={}] {}",
            level, self.principal, self.request_id, message
        );
    }

    /// Logs an info message.
    pub fn info(&self, message: &str) {
        self.log("INFO", message);
    }

    /// Logs a warning message.
    pub fn warn(&self, message: &str) {
        self.log("WARN", message);
    }

    /// Logs an error message.
    pub fn error(&self, message: &str) {
        self.log("ERROR", message);
    }
}

/// Represents the set of permissions for a principal.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PermissionContext {
    /// Permission entries
    pub entries: Vec<PermissionEntry>,
}

/// Represents a single URI-based access control entry.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PermissionEntry {
    /// URI pattern with wildcard support
    pub uri_pattern: String,

    /// Allowed permission names
    pub permissions: Vec<String>,
}

/// Response from an app execution.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Response {
    /// HTTP-style status code (200, 404, 500, etc.)
    pub status_code: i32,

    /// Response metadata
    #[serde(default)]
    pub headers: HashMap<String, String>,

    /// Response payload
    #[serde(with = "serde_bytes_vec")]
    pub body: Vec<u8>,

    /// Process exit code (0 = success)
    #[serde(default)]
    pub exit_code: i32,
}

impl Response {
    /// Creates a successful response with the given body.
    pub fn success(body: Vec<u8>) -> Self {
        Self {
            status_code: 200,
            headers: HashMap::new(),
            body,
            exit_code: 0,
        }
    }

    /// Creates a successful response with a string body.
    pub fn success_string(message: impl Into<String>) -> Self {
        Self::success(message.into().into_bytes())
    }

    /// Creates a successful response with JSON-encoded body.
    pub fn success_json(value: &impl Serialize) -> Result<Self, serde_json::Error> {
        let body = serde_json::to_vec(value)?;
        let mut resp = Self::success(body);
        resp.headers.insert("Content-Type".to_string(), "application/json".to_string());
        Ok(resp)
    }

    /// Creates an error response with the given status code and message.
    pub fn error(status_code: i32, message: impl Into<String>) -> Self {
        let exit_code = if status_code >= 500 { 2 } else { 1 };
        Self {
            status_code,
            headers: HashMap::new(),
            body: message.into().into_bytes(),
            exit_code,
        }
    }

    /// Creates a 400 Bad Request response.
    pub fn bad_request(message: impl Into<String>) -> Self {
        Self::error(400, message)
    }

    /// Creates a 403 Forbidden response.
    pub fn forbidden(message: impl Into<String>) -> Self {
        Self::error(403, message)
    }

    /// Creates a 404 Not Found response.
    pub fn not_found(message: impl Into<String>) -> Self {
        Self::error(404, message)
    }

    /// Creates a 500 Internal Server Error response.
    pub fn internal_error(message: impl Into<String>) -> Self {
        Self::error(500, message)
    }

    /// Adds a header to the response.
    pub fn with_header(mut self, key: impl Into<String>, value: impl Into<String>) -> Self {
        self.headers.insert(key.into(), value.into());
        self
    }
}

/// HTTP-style request for request/response apps.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Request {
    /// HTTP method (GET, POST, etc.)
    pub method: String,

    /// Request path
    pub path: String,

    /// Request headers
    #[serde(default)]
    pub headers: HashMap<String, String>,

    /// Request body
    #[serde(with = "serde_bytes_vec")]
    pub body: Vec<u8>,
}

// ============================================================================
// Handler Traits
// ============================================================================

/// Handler for MCP tool invocations with JSON input/output.
///
/// This is the simplest and most common handler for building MCP tools.
pub trait MCPToolHandler {
    /// Processes arbitrary JSON input and returns a JSON response.
    ///
    /// The input Value contains the parsed JSON arguments from the MCP call.
    /// Return a Value that will be serialized to JSON for the response.
    fn handle(&self, ctx: &Context, input: Value) -> Result<Value, Box<dyn Error>>;
}

/// Handler for CLI-style apps with command-line arguments.
pub trait CLIHandler {
    /// Processes command-line arguments and returns a response.
    fn run(&self, ctx: &Context, args: Vec<String>) -> Result<Response, Box<dyn Error>>;
}

/// Handler for HTTP-style request/response apps.
pub trait RequestHandler {
    /// Processes a request and returns a response.
    fn handle(&self, ctx: &Context, req: Request) -> Result<Response, Box<dyn Error>>;
}

// ============================================================================
// Entry Point Functions
// ============================================================================

/// Executes an MCP tool handler with JSON input from stdin.
///
/// This is the recommended entry point for building MCP tools.
///
/// # Example
///
/// ```rust,no_run
/// use wazeos_app::{MCPToolHandler, Context, run_mcp_tool};
/// use serde_json::Value;
///
/// struct MyTool;
///
/// impl MCPToolHandler for MyTool {
///     fn handle(&self, ctx: &Context, input: Value) -> Result<Value, Box<dyn std::error::Error>> {
///         Ok(serde_json::json!({"status": "success"}))
///     }
/// }
///
/// fn main() {
///     run_mcp_tool(&MyTool);
/// }
/// ```
pub fn run_mcp_tool<H: MCPToolHandler>(handler: &H) {
    let ctx = Context::from_env();

    // Read JSON input from stdin
    let input: Value = match serde_json::from_reader(io::stdin()) {
        Ok(input) => input,
        Err(e) => {
            // Empty input is okay for tools that don't require parameters
            if e.is_eof() {
                Value::Object(serde_json::Map::new())
            } else {
                handle_error(&ctx, &format!("Failed to parse JSON input: {}", e));
                std::process::exit(1);
            }
        }
    };

    // Call handler
    let output = match handler.handle(&ctx, input) {
        Ok(output) => output,
        Err(e) => {
            handle_error(&ctx, &e.to_string());
            std::process::exit(1);
        }
    };

    // Write JSON output to stdout
    if let Err(e) = serde_json::to_writer(io::stdout(), &output) {
        handle_error(&ctx, &format!("Failed to encode response: {}", e));
        std::process::exit(1);
    }

    let _ = io::stdout().flush();
}

/// Executes a CLI handler with command-line arguments.
///
/// # Example
///
/// ```rust,no_run
/// use wazeos_app::{CLIHandler, Context, Response, run_cli};
///
/// struct MyCLI;
///
/// impl CLIHandler for MyCLI {
///     fn run(&self, ctx: &Context, args: Vec<String>) -> Result<Response, Box<dyn std::error::Error>> {
///         Ok(Response::success_string(format!("Args: {:?}", args)))
///     }
/// }
///
/// fn main() {
///     run_cli(&MyCLI);
/// }
/// ```
pub fn run_cli<H: CLIHandler>(handler: &H) {
    let ctx = Context::from_env();
    let args: Vec<String> = env::args().skip(1).collect();

    let response = match handler.run(&ctx, args) {
        Ok(response) => response,
        Err(e) => {
            handle_error(&ctx, &e.to_string());
            std::process::exit(1);
        }
    };

    write_response(&response);
    std::process::exit(response.exit_code);
}

/// Executes a request handler with input from stdin.
///
/// # Example
///
/// ```rust,no_run
/// use wazeos_app::{RequestHandler, Context, Request, Response, run_handler};
///
/// struct MyHandler;
///
/// impl RequestHandler for MyHandler {
///     fn handle(&self, ctx: &Context, req: Request) -> Result<Response, Box<dyn std::error::Error>> {
///         Ok(Response::success_string(format!("Received: {}", req.path)))
///     }
/// }
///
/// fn main() {
///     run_handler(&MyHandler);
/// }
/// ```
pub fn run_handler<H: RequestHandler>(handler: &H) {
    let ctx = Context::from_env();

    // Read request from stdin
    let request: Request = match serde_json::from_reader(io::stdin()) {
        Ok(req) => req,
        Err(e) => {
            handle_error(&ctx, &format!("Failed to parse request: {}", e));
            std::process::exit(1);
        }
    };

    // Handle request
    let response = match handler.handle(&ctx, request) {
        Ok(response) => response,
        Err(e) => {
            handle_error(&ctx, &e.to_string());
            std::process::exit(1);
        }
    };

    write_response(&response);
    std::process::exit(response.exit_code);
}

// ============================================================================
// Helper Functions
// ============================================================================

/// Writes an error to stderr and stdout.
fn handle_error(ctx: &Context, message: &str) {
    ctx.error(message);
    eprintln!("Error: {}", message);
}

/// Writes a response to stdout.
fn write_response(response: &Response) {
    // For simple responses, just write the body
    let _ = io::stdout().write_all(&response.body);
    let _ = io::stdout().flush();
}

/// Checks if a URI matches a pattern (simple glob matching).
fn matches_pattern(pattern: &str, uri: &str) -> bool {
    if pattern.ends_with('*') {
        let prefix = pattern.trim_end_matches('*');
        uri.starts_with(prefix)
    } else {
        pattern == uri
    }
}

// ============================================================================
// Serde helpers for byte arrays
// ============================================================================

mod serde_bytes_vec {
    use serde::{Deserialize, Deserializer, Serializer};
    use serde::de::Error;

    pub fn serialize<S>(bytes: &Vec<u8>, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: Serializer,
    {
        serializer.serialize_bytes(bytes)
    }

    pub fn deserialize<'de, D>(deserializer: D) -> Result<Vec<u8>, D::Error>
    where
        D: Deserializer<'de>,
    {
        let bytes = serde_json::Value::deserialize(deserializer)?;
        match bytes {
            serde_json::Value::String(s) => Ok(s.into_bytes()),
            serde_json::Value::Array(arr) => {
                arr.iter()
                    .map(|v| {
                        v.as_u64()
                            .and_then(|n| u8::try_from(n).ok())
                            .ok_or_else(|| Error::custom("invalid byte value"))
                    })
                    .collect()
            }
            _ => Err(Error::custom("expected string or array for bytes")),
        }
    }
}
