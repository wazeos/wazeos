//! WazeOS App SDK
//!
//! This SDK provides a generic interface for building MCP tools that run
//! as WASM modules within WazeOS.
//!
//! # Core Interface
//!
//! The SDK provides a generic `call()` method for invoking any IO Bus driver.
//! For convenience, use driver-specific SDK addons (wazeos-file, wazeos-shell, etc.).
//!
//! # Example with Raw Call
//!
//! ```rust
//! use serde_json::{json, Value};
//! use wazeos_app::{AppContext, AppResult, register_tool};
//!
//! #[no_mangle]
//! pub extern "C" fn tool_main(ctx: &AppContext, args: Value) -> AppResult {
//!     // Generic call works with any driver
//!     let resp = ctx.call("file:///tmp/data.txt", Default::default(), Vec::new())?;
//!     let content = String::from_utf8(resp.body).unwrap();
//!
//!     Ok(json!({
//!         "result": content.to_uppercase()
//!     }))
//! }
//!
//! register_tool!(tool_main);
//! ```
//!
//! # Example with Driver Addons
//!
//! ```rust
//! use wazeos_app::{AppContext, AppResult, register_tool};
//! use wazeos_file::FileOps;  // Driver SDK addon
//!
//! #[no_mangle]
//! pub extern "C" fn tool_main(ctx: &AppContext, args: Value) -> AppResult {
//!     // Ergonomic helper from file driver SDK
//!     let content = ctx.read_file("/tmp/data.txt")?;
//!     Ok(json!({"result": content.to_uppercase()}))
//! }
//!
//! register_tool!(tool_main);
//! ```

use base64::prelude::*;
use serde::{Deserialize, Deserializer, Serialize};

// Import and re-export HashMap for use in this module and by driver addons
pub use std::collections::HashMap;

/// Result type for MCP tool functions
pub type AppResult = Result<serde_json::Value, String>;

// ============================================================================
// Host Function Imports
// ============================================================================

#[link(wasm_import_module = "env")]
extern "C" {
    /// Call the IO Bus from WASM
    /// Takes a pointer to request JSON and length
    /// Returns a u64 with pointer (high 32 bits) and length (low 32 bits)
    fn host_iobus_call(ptr: u32, length: u32) -> u64;
}

// ============================================================================
// Public Request/Response Types
// ============================================================================

/// Request to the IO Bus
///
/// This is the low-level request structure used to communicate with drivers
/// through the IO Bus. Most apps should use the `AppContext::call()` method
/// or driver-specific SDK addons instead of constructing this directly.
#[derive(Debug, Serialize, Deserialize)]
pub struct IOBusRequest {
    pub uri: String,
    pub operation: String,
    #[serde(default)]
    pub args: HashMap<String, serde_json::Value>,
    #[serde(default)]
    pub headers: HashMap<String, String>,
    #[serde(default, serialize_with = "serialize_base64_body")]
    pub body: Vec<u8>,
}

// Custom serializer for base64-encoded body
fn serialize_base64_body<S>(body: &Vec<u8>, serializer: S) -> Result<S::Ok, S::Error>
where
    S: serde::Serializer,
{
    if body.is_empty() {
        serializer.serialize_str("")
    } else {
        serializer.serialize_str(&BASE64_STANDARD.encode(body))
    }
}

// Custom deserializer for base64-encoded body
fn deserialize_base64_body<'de, D>(deserializer: D) -> Result<Vec<u8>, D::Error>
where
    D: Deserializer<'de>,
{
    let s = String::deserialize(deserializer)?;
    if s.is_empty() {
        return Ok(Vec::new());
    }
    BASE64_STANDARD
        .decode(s.as_bytes())
        .map_err(serde::de::Error::custom)
}

/// Response from the IO Bus
///
/// Contains the result of a driver operation. Driver SDK addons typically
/// parse this into more ergonomic types.
#[derive(Debug, Serialize, Deserialize)]
pub struct IOBusResponse {
    pub status_code: u16,
    #[serde(default)]
    pub headers: HashMap<String, String>,
    #[serde(default, deserialize_with = "deserialize_base64_body")]
    pub body: Vec<u8>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
}

/// Internal helper to call IO Bus
pub(crate) fn iobus_call(req: &IOBusRequest) -> Result<IOBusResponse, String> {
    unsafe {
        // Serialize request to JSON
        let req_json = serde_json::to_string(req)
            .map_err(|e| format!("Failed to serialize request: {}", e))?;

        let req_bytes = req_json.as_bytes();
        let req_ptr = req_bytes.as_ptr() as u32;
        let req_len = req_bytes.len() as u32;

        // Call host function
        let result = host_iobus_call(req_ptr, req_len);

        if result == 0 {
            return Err("IO Bus call failed".to_string());
        }

        // Extract response pointer and length
        let resp_ptr = (result >> 32) as u32;
        let resp_len = (result & 0xFFFFFFFF) as u32;

        // Read response from memory
        let resp_slice = std::slice::from_raw_parts(resp_ptr as *const u8, resp_len as usize);

        // Deserialize response
        let resp: IOBusResponse = serde_json::from_slice(resp_slice)
            .map_err(|e| format!("Failed to deserialize response: {}", e))?;

        Ok(resp)
    }
}

// ============================================================================
// Application Context
// ============================================================================

/// Application context providing access to I/O drivers
///
/// The AppContext allows your MCP tool to interact with any IO Bus driver
/// through a generic `call()` method. For ergonomic APIs, use driver-specific
/// SDK addons (wazeos-file, wazeos-shell, etc.).
pub struct AppContext {
    // Context is currently zero-sized - all state is managed by the host
}

impl AppContext {
    /// Generic call to any IO Bus driver
    ///
    /// This is the low-level method that works with any driver. Driver-specific
    /// SDK addons provide more ergonomic wrappers around this method.
    ///
    /// # Arguments
    /// * `uri` - The resource URI (e.g., "file:///tmp/data.txt", "shell://exec", "http://api.example.com")
    /// * `headers` - Optional headers for the request (driver-specific)
    /// * `body` - Optional request body
    ///
    /// # Returns
    /// An `IOBusResponse` containing status code, headers, body, and optional error
    ///
    /// # Examples
    ///
    /// ```ignore
    /// // Read a file
    /// let resp = ctx.call("file:///tmp/data.txt", HashMap::new(), Vec::new())?;
    /// if resp.status_code == 200 {
    ///     let content = String::from_utf8(resp.body).unwrap();
    /// }
    ///
    /// // Write a file
    /// let mut headers = HashMap::new();
    /// headers.insert("operation".to_string(), "write".to_string());
    /// ctx.call("file:///tmp/output.txt", headers, b"content".to_vec())?;
    ///
    /// // Execute shell command
    /// let mut headers = HashMap::new();
    /// headers.insert("command".to_string(), "date".to_string());
    /// let resp = ctx.call("shell://exec", headers, Vec::new())?;
    ///
    /// // HTTP GET
    /// let resp = ctx.call("http://api.example.com/data", HashMap::new(), Vec::new())?;
    ///
    /// // Any custom driver
    /// ctx.call("redis://localhost/GET/mykey", HashMap::new(), Vec::new())?;
    /// ```
    pub fn call(
        &self,
        uri: &str,
        headers: HashMap<String, String>,
        body: Vec<u8>,
    ) -> Result<IOBusResponse, String> {
        let req = IOBusRequest {
            uri: uri.to_string(),
            operation: "call".to_string(),
            args: HashMap::new(),
            headers,
            body,
        };

        iobus_call(&req)
    }
}

/// Register an MCP tool
///
/// This macro generates the necessary exports for your tool to be
/// callable as an MCP tool from Claude and other MCP clients.
///
/// # Example
///
/// ```rust
/// use serde_json::{json, Value};
/// use wazeos_app::{AppContext, AppResult, register_tool};
///
/// #[no_mangle]
/// pub extern "C" fn my_tool(ctx: &AppContext, args: Value) -> AppResult {
///     Ok(json!({"result": "success"}))
/// }
///
/// register_tool!(my_tool);
/// ```
#[macro_export]
macro_rules! register_tool {
    ($tool_fn:ident) => {
        use std::ffi::c_void;

        /// Tool entry point called by the WazeOS runtime
        ///
        /// This is the main entry point that the WazeOS kernel calls
        /// when your tool is invoked from an MCP client.
        #[no_mangle]
        pub extern "C" fn wazeos_tool_invoke(args_ptr: *const u8, args_len: usize) -> *const u8 {
            use $crate::AppContext;

            // Safety: The WazeOS runtime guarantees valid pointers
            let result = unsafe {
                // Read args from WASM memory
                let args_slice = std::slice::from_raw_parts(args_ptr, args_len);
                let args_json = std::str::from_utf8(args_slice).unwrap_or("{}");

                // Parse args
                let args: serde_json::Value = serde_json::from_str(args_json).unwrap_or_else(|_| {
                    serde_json::json!({})
                });

                // Create context (currently empty, future: inject driver access)
                let ctx = AppContext {};

                // Call the user's tool function
                match $tool_fn(&ctx, args) {
                    Ok(result) => {
                        // Success - return the result as JSON
                        let response = serde_json::json!({
                            "success": true,
                            "result": result
                        });
                        serde_json::to_string(&response).unwrap()
                    }
                    Err(error) => {
                        // Error - return error response
                        let response = serde_json::json!({
                            "success": false,
                            "error": error
                        });
                        serde_json::to_string(&response).unwrap()
                    }
                }
            };

            // Convert result to C string (null-terminated)
            let mut result_bytes = result.into_bytes();
            result_bytes.push(0); // Null terminator

            // Leak the string so it persists for the caller to read
            let ptr = result_bytes.as_ptr();
            std::mem::forget(result_bytes);

            ptr
        }

        /// Tool metadata export
        ///
        /// Returns JSON metadata about this tool for MCP discovery
        #[no_mangle]
        pub extern "C" fn wazeos_tool_metadata() -> *const u8 {
            let metadata = serde_json::json!({
                "name": stringify!($tool_fn),
                "version": env!("CARGO_PKG_VERSION"),
            });

            let json_str = serde_json::to_string(&metadata).unwrap();
            let mut bytes = json_str.into_bytes();
            bytes.push(0); // Null terminator

            let ptr = bytes.as_ptr();
            std::mem::forget(bytes);

            ptr
        }
    };
}

// Re-export commonly used types
pub use serde_json::{json, Value};
