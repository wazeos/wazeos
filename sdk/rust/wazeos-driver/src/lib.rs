//! WazeOS Driver SDK for Rust
//!
//! This SDK provides types and utilities for building WazeOS WASM drivers in Rust.
//! Drivers communicate with the WazeOS kernel using a JSON-based stdin/stdout protocol.
//!
//! # Example: Resource Driver
//!
//! ```rust,no_run
//! use wazeos_driver::{ResourceCall, ResourceResult, ResourceHandler, serve_resource_once};
//!
//! struct MyDriver;
//!
//! impl ResourceHandler for MyDriver {
//!     fn handle_call(&self, call: &ResourceCall) -> Result<ResourceResult, Box<dyn std::error::Error>> {
//!         // Process the resource call
//!         Ok(ResourceResult::success(200, b"Hello, World!".to_vec()))
//!     }
//! }
//!
//! fn main() {
//!     let driver = MyDriver;
//!     serve_resource_once(&driver);
//! }
//! ```
//!
//! # Example: Authentication Driver
//!
//! ```rust,no_run
//! use wazeos_driver::{AuthPayload, AuthResult, AuthHandler, serve_auth_once};
//!
//! struct MyAuthDriver;
//!
//! impl AuthHandler for MyAuthDriver {
//!     fn authenticate(&self, payload: &AuthPayload) -> Result<AuthResult, Box<dyn std::error::Error>> {
//!         // Validate credentials
//!         if payload.headers.get("Authorization").is_some() {
//!             Ok(AuthResult::success("user:alice".to_string()))
//!         } else {
//!             Ok(AuthResult::error("missing authorization"))
//!         }
//!     }
//! }
//!
//! fn main() {
//!     let auth = MyAuthDriver;
//!     serve_auth_once(&auth);
//! }
//! ```

use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::error::Error;
use std::io::{self, BufRead, Write};

// ============================================================================
// Core Types
// ============================================================================

/// Represents an IO call from a WASM app to a resource driver.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResourceCall {
    /// The URI being accessed (e.g., "file:///data.txt")
    pub uri: String,

    /// HTTP-style headers
    #[serde(default)]
    pub headers: HashMap<String, String>,

    /// Request body as raw bytes
    #[serde(with = "serde_bytes_vec")]
    pub body: Vec<u8>,

    /// Permissions available for this call
    #[serde(default)]
    pub permissions: Vec<String>,
}

/// Represents the result of a resource call.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ResourceResult {
    /// HTTP-style status code (200, 404, 500, etc.)
    pub status_code: i32,

    /// Response headers
    #[serde(default)]
    pub headers: HashMap<String, String>,

    /// Response body as raw bytes
    #[serde(with = "serde_bytes_vec")]
    pub body: Vec<u8>,

    /// Error message if the operation failed
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
}

impl ResourceResult {
    /// Creates a successful resource result.
    pub fn success(status_code: i32, body: Vec<u8>) -> Self {
        Self {
            status_code,
            headers: HashMap::new(),
            body,
            error: None,
        }
    }

    /// Creates an error resource result.
    pub fn error(status_code: i32, message: impl Into<String>) -> Self {
        let msg = message.into();
        Self {
            status_code,
            headers: HashMap::new(),
            body: msg.as_bytes().to_vec(),
            error: Some(msg),
        }
    }

    /// Adds a header to the result.
    pub fn with_header(mut self, key: impl Into<String>, value: impl Into<String>) -> Self {
        self.headers.insert(key.into(), value.into());
        self
    }
}

/// Represents authentication input from a request.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AuthPayload {
    /// HTTP-style headers
    #[serde(default)]
    pub headers: HashMap<String, String>,

    /// Request body as raw bytes
    #[serde(with = "serde_bytes_vec")]
    pub body: Vec<u8>,
}

/// Represents the authentication result.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AuthResult {
    /// The authenticated principal (e.g., "user:alice")
    #[serde(skip_serializing_if = "Option::is_none")]
    pub principal: Option<String>,

    /// Error message or "abstain"
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
}

impl AuthResult {
    /// Creates a successful authentication result.
    pub fn success(principal: impl Into<String>) -> Self {
        Self {
            principal: Some(principal.into()),
            error: None,
        }
    }

    /// Creates an error authentication result.
    pub fn error(message: impl Into<String>) -> Self {
        Self {
            principal: None,
            error: Some(message.into()),
        }
    }

    /// Creates an abstain authentication result.
    /// Use this when the driver doesn't handle this authentication type.
    pub fn abstain() -> Self {
        Self {
            principal: None,
            error: Some("abstain".to_string()),
        }
    }

    /// Checks if this result is an abstain.
    pub fn is_abstain(&self) -> bool {
        self.error.as_deref() == Some("abstain")
    }
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

/// Represents the set of permissions for a principal.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PermissionContext {
    /// Permission entries
    pub entries: Vec<PermissionEntry>,
}

// ============================================================================
// Traits
// ============================================================================

/// Interface that resource drivers must implement.
pub trait ResourceHandler {
    /// Processes a resource call and returns a result.
    fn handle_call(&self, call: &ResourceCall) -> Result<ResourceResult, Box<dyn Error>>;
}

/// Interface that authentication drivers must implement.
pub trait AuthHandler {
    /// Validates credentials and returns a principal or error.
    /// Return `AuthResult::abstain()` if the driver doesn't handle this auth type.
    fn authenticate(&self, payload: &AuthPayload) -> Result<AuthResult, Box<dyn Error>>;
}

// ============================================================================
// Serve Functions
// ============================================================================

/// Handles a single resource call from stdin and writes result to stdout, then exits.
///
/// This is the recommended entry point for most resource drivers.
///
/// # Example
///
/// ```rust,no_run
/// use wazeos_driver::{ResourceCall, ResourceResult, ResourceHandler, serve_resource_once};
///
/// struct MyDriver;
///
/// impl ResourceHandler for MyDriver {
///     fn handle_call(&self, call: &ResourceCall) -> Result<ResourceResult, Box<dyn std::error::Error>> {
///         Ok(ResourceResult::success(200, b"OK".to_vec()))
///     }
/// }
///
/// fn main() {
///     serve_resource_once(&MyDriver);
/// }
/// ```
pub fn serve_resource_once<H: ResourceHandler>(handler: &H) {
    // Read request from stdin
    let call: ResourceCall = match serde_json::from_reader(io::stdin()) {
        Ok(call) => call,
        Err(e) => {
            write_error(&format!("failed to parse call: {}", e));
            std::process::exit(1);
        }
    };

    // Handle the call
    let result = match handler.handle_call(&call) {
        Ok(result) => result,
        Err(e) => ResourceResult::error(500, e.to_string()),
    };

    // Write result to stdout
    if let Err(e) = serde_json::to_writer(io::stdout(), &result) {
        write_error(&format!("failed to write result: {}", e));
        std::process::exit(1);
    }

    // Flush to ensure it's written
    let _ = io::stdout().flush();
}

/// Handles a single auth request from stdin and writes result to stdout, then exits.
///
/// # Example
///
/// ```rust,no_run
/// use wazeos_driver::{AuthPayload, AuthResult, AuthHandler, serve_auth_once};
///
/// struct MyAuthDriver;
///
/// impl AuthHandler for MyAuthDriver {
///     fn authenticate(&self, payload: &AuthPayload) -> Result<AuthResult, Box<dyn std::error::Error>> {
///         Ok(AuthResult::success("user:alice"))
///     }
/// }
///
/// fn main() {
///     serve_auth_once(&MyAuthDriver);
/// }
/// ```
pub fn serve_auth_once<H: AuthHandler>(handler: &H) {
    // Read request from stdin
    let payload: AuthPayload = match serde_json::from_reader(io::stdin()) {
        Ok(payload) => payload,
        Err(e) => {
            write_auth_error(&format!("failed to parse payload: {}", e));
            std::process::exit(1);
        }
    };

    // Authenticate
    let result = match handler.authenticate(&payload) {
        Ok(result) => result,
        Err(e) => AuthResult::error(e.to_string()),
    };

    // Write result to stdout
    if let Err(e) = serde_json::to_writer(io::stdout(), &result) {
        write_auth_error(&format!("failed to write result: {}", e));
        std::process::exit(1);
    }

    // Flush to ensure it's written
    let _ = io::stdout().flush();
}

/// Runs a resource driver in a loop, processing multiple calls from stdin.
///
/// Each call is expected to be a JSON object on a single line.
///
/// # Example
///
/// ```rust,no_run
/// use wazeos_driver::{ResourceHandler, serve_resource_stdio};
///
/// # struct MyDriver;
/// # impl ResourceHandler for MyDriver {
/// #     fn handle_call(&self, call: &wazeos_driver::ResourceCall) -> Result<wazeos_driver::ResourceResult, Box<dyn std::error::Error>> {
/// #         Ok(wazeos_driver::ResourceResult::success(200, vec![]))
/// #     }
/// # }
/// fn main() {
///     serve_resource_stdio(&MyDriver);
/// }
/// ```
pub fn serve_resource_stdio<H: ResourceHandler>(handler: &H) {
    let stdin = io::stdin();
    let reader = stdin.lock();

    for line in reader.lines() {
        let line = match line {
            Ok(line) => line,
            Err(e) => {
                write_error(&format!("failed to read stdin: {}", e));
                continue;
            }
        };

        // Parse ResourceCall
        let call: ResourceCall = match serde_json::from_str(&line) {
            Ok(call) => call,
            Err(e) => {
                write_error(&format!("failed to parse call: {}", e));
                continue;
            }
        };

        // Handle the call
        let result = match handler.handle_call(&call) {
            Ok(result) => result,
            Err(e) => ResourceResult::error(500, e.to_string()),
        };

        // Write result to stdout
        if let Ok(json) = serde_json::to_string(&result) {
            println!("{}", json);
            let _ = io::stdout().flush();
        } else {
            write_error("failed to serialize result");
        }
    }
}

/// Runs an auth driver in a loop, processing multiple auth requests from stdin.
///
/// Each request is expected to be a JSON object on a single line.
///
/// # Example
///
/// ```rust,no_run
/// use wazeos_driver::{AuthHandler, serve_auth_stdio};
///
/// # struct MyAuthDriver;
/// # impl AuthHandler for MyAuthDriver {
/// #     fn authenticate(&self, payload: &wazeos_driver::AuthPayload) -> Result<wazeos_driver::AuthResult, Box<dyn std::error::Error>> {
/// #         Ok(wazeos_driver::AuthResult::success("user"))
/// #     }
/// # }
/// fn main() {
///     serve_auth_stdio(&MyAuthDriver);
/// }
/// ```
pub fn serve_auth_stdio<H: AuthHandler>(handler: &H) {
    let stdin = io::stdin();
    let reader = stdin.lock();

    for line in reader.lines() {
        let line = match line {
            Ok(line) => line,
            Err(e) => {
                write_auth_error(&format!("failed to read stdin: {}", e));
                continue;
            }
        };

        // Parse AuthPayload
        let payload: AuthPayload = match serde_json::from_str(&line) {
            Ok(payload) => payload,
            Err(e) => {
                write_auth_error(&format!("failed to parse payload: {}", e));
                continue;
            }
        };

        // Authenticate
        let result = match handler.authenticate(&payload) {
            Ok(result) => result,
            Err(e) => AuthResult::error(e.to_string()),
        };

        // Write result to stdout
        if let Ok(json) = serde_json::to_string(&result) {
            println!("{}", json);
            let _ = io::stdout().flush();
        } else {
            write_auth_error("failed to serialize result");
        }
    }
}

// ============================================================================
// Helper Functions
// ============================================================================

/// Writes an error ResourceResult to stdout
fn write_error(message: &str) {
    let result = ResourceResult::error(500, message);
    let _ = serde_json::to_writer(io::stdout(), &result);
    let _ = io::stdout().flush();
}

/// Writes an error AuthResult to stdout
fn write_auth_error(message: &str) {
    let result = AuthResult::error(message);
    let _ = serde_json::to_writer(io::stdout(), &result);
    let _ = io::stdout().flush();
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
            serde_json::Value::String(s) => {
                // Try base64 decode, fallback to UTF-8 bytes
                base64::decode(&s)
                    .or_else(|_| Ok(s.into_bytes()))
                    .map_err(Error::custom)
            }
            serde_json::Value::Array(arr) => {
                // Array of numbers
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

// Simple base64 decoder (to avoid adding a dependency)
mod base64 {
    pub fn decode(s: &str) -> Result<Vec<u8>, ()> {
        // This is a placeholder - in production, use a proper base64 crate
        // For now, just fail so we fallback to UTF-8
        Err(())
    }
}
