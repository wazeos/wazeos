//! WazeOS Driver SDK for Rust
//!
//! This SDK provides the foundation for writing WASM-based drivers
//! that can be loaded and executed by the WazeOS kernel.

use serde::{Deserialize, Serialize};
use std::collections::HashMap;

// ============================================================================
// Core Types
// ============================================================================

/// Request represents an operation request from the kernel
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Request {
    /// Target URI (e.g., "shell://exec", "http://example.com")
    pub uri: String,

    /// Operation type
    pub operation: String,

    /// Optional structured arguments
    #[serde(default)]
    pub args: HashMap<String, serde_json::Value>,

    /// Optional headers (protocol-specific metadata)
    #[serde(default)]
    pub headers: HashMap<String, String>,

    /// Optional binary body
    #[serde(default)]
    #[serde(with = "serde_bytes_base64")]
    pub body: Vec<u8>,
}

/// Response represents an operation result
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Response {
    /// HTTP-style status code
    pub status_code: u16,

    /// Response headers
    #[serde(default)]
    pub headers: HashMap<String, String>,

    /// Response body
    #[serde(default)]
    #[serde(with = "serde_bytes_base64")]
    pub body: Vec<u8>,

    /// Optional error message
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
}

/// Driver metadata
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DriverMetadata {
    pub name: String,
    pub version: String,
    pub class: String,
    pub uri_pattern: String,
    pub capabilities: Vec<String>,
}

// ============================================================================
// Driver Trait
// ============================================================================

/// Driver trait that all WASM drivers must implement
pub trait Driver {
    /// Returns driver metadata
    fn metadata(&self) -> DriverMetadata;

    /// Initialize the driver with configuration
    fn init(&mut self, config: HashMap<String, serde_json::Value>) -> Result<(), String>;

    /// Handle a call request
    fn call(&mut self, req: Request) -> Result<Response, String>;

    /// Close/cleanup the driver (optional)
    fn close(&mut self) -> Result<(), String> {
        Ok(())
    }
}

// ============================================================================
// Response Builders
// ============================================================================

impl Response {
    /// Create a successful response
    pub fn success(body: Vec<u8>) -> Self {
        Self {
            status_code: 200,
            headers: HashMap::new(),
            body,
            error: None,
        }
    }

    /// Create a successful response with text
    pub fn success_text(text: &str) -> Self {
        Self::success(text.as_bytes().to_vec())
    }

    /// Create a successful response with JSON
    pub fn success_json<T: Serialize>(data: &T) -> Result<Self, String> {
        let body = serde_json::to_vec(data)
            .map_err(|e| format!("Failed to serialize response: {}", e))?;
        Ok(Self::success(body))
    }

    /// Create an error response
    pub fn error(status_code: u16, message: &str) -> Self {
        Self {
            status_code,
            headers: HashMap::new(),
            body: message.as_bytes().to_vec(),
            error: Some(message.to_string()),
        }
    }
}

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

/// Call the IO Bus from Rust
pub fn iobus_call(req: &Request) -> Result<Response, String> {
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
        let resp: Response = serde_json::from_slice(resp_slice)
            .map_err(|e| format!("Failed to deserialize response: {}", e))?;

        Ok(resp)
    }
}

// ============================================================================
// Driver Registration Macro
// ============================================================================

/// Macro to register a driver implementation
///
/// This generates the required WASM exports for the driver
///
/// # Example
///
/// ```
/// use wazeos_driver::{Driver, DriverMetadata, Request, Response, register_driver};
///
/// struct MyDriver;
///
/// impl Driver for MyDriver {
///     fn metadata(&self) -> DriverMetadata {
///         DriverMetadata {
///             name: "my-driver".to_string(),
///             version: "1.0.0".to_string(),
///             class: "io.connect".to_string(),
///             uri_pattern: "mydriver://**".to_string(),
///             capabilities: vec!["call".to_string()],
///         }
///     }
///
///     fn init(&mut self, _config: HashMap<String, serde_json::Value>) -> Result<(), String> {
///         Ok(())
///     }
///
///     fn call(&mut self, req: Request) -> Result<Response, String> {
///         Response::success_text("Hello from my driver!")
///     }
/// }
///
/// register_driver!(MyDriver);
/// ```
#[macro_export]
macro_rules! register_driver {
    ($driver_type:ty) => {
        static mut DRIVER: Option<$driver_type> = None;

        #[no_mangle]
        pub extern "C" fn driver_metadata() -> *const u8 {
            unsafe {
                if DRIVER.is_none() {
                    DRIVER = Some(<$driver_type>::default());
                }

                let driver = DRIVER.as_ref().unwrap();
                let metadata = driver.metadata();
                let json = serde_json::to_string(&metadata).unwrap();
                // Add null terminator for C-style string
                let mut bytes = json.into_bytes();
                bytes.push(0);
                let ptr = bytes.as_ptr();
                std::mem::forget(bytes);
                ptr
            }
        }

        #[no_mangle]
        pub extern "C" fn driver_init(ptr: u32, length: u32) -> u32 {
            unsafe {
                if DRIVER.is_none() {
                    DRIVER = Some(<$driver_type>::default());
                }

                let driver = DRIVER.as_mut().unwrap();
                let config_slice = std::slice::from_raw_parts(ptr as *const u8, length as usize);
                let config: HashMap<String, serde_json::Value> =
                    serde_json::from_slice(config_slice).unwrap_or_default();

                match driver.init(config) {
                    Ok(_) => 0,
                    Err(_) => 1,
                }
            }
        }

        #[no_mangle]
        pub extern "C" fn driver_call(ptr: u32, length: u32) -> *const u8 {
            unsafe {
                let driver = DRIVER.as_mut().unwrap();
                let req_slice = std::slice::from_raw_parts(ptr as *const u8, length as usize);
                let req: Request = serde_json::from_slice(req_slice).unwrap();

                let resp = match driver.call(req) {
                    Ok(r) => r,
                    Err(e) => Response::error(500, &e),
                };

                let json = serde_json::to_string(&resp).unwrap();
                // Add null terminator for C-style string
                let mut bytes = json.into_bytes();
                bytes.push(0);
                let ptr = bytes.as_ptr();
                std::mem::forget(bytes);
                ptr
            }
        }
    };
}

// ============================================================================
// Base64 serialization for byte arrays
// ============================================================================

mod serde_bytes_base64 {
    use serde::{Deserialize, Deserializer, Serializer};
    use std::collections::HashMap;

    pub fn serialize<S>(bytes: &[u8], serializer: S) -> Result<S::Ok, S::Error>
    where
        S: Serializer,
    {
        serializer.serialize_str(&base64_encode(bytes))
    }

    pub fn deserialize<'de, D>(deserializer: D) -> Result<Vec<u8>, D::Error>
    where
        D: Deserializer<'de>,
    {
        let s = String::deserialize(deserializer)?;
        base64_decode(&s).map_err(serde::de::Error::custom)
    }

    fn base64_encode(bytes: &[u8]) -> String {
        // Simple base64 encoding
        const CHARS: &[u8] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
        let mut result = String::new();

        for chunk in bytes.chunks(3) {
            let b1 = chunk[0];
            let b2 = chunk.get(1).copied().unwrap_or(0);
            let b3 = chunk.get(2).copied().unwrap_or(0);

            result.push(CHARS[(b1 >> 2) as usize] as char);
            result.push(CHARS[(((b1 & 0x03) << 4) | (b2 >> 4)) as usize] as char);
            result.push(if chunk.len() > 1 {
                CHARS[(((b2 & 0x0F) << 2) | (b3 >> 6)) as usize] as char
            } else {
                '='
            });
            result.push(if chunk.len() > 2 {
                CHARS[(b3 & 0x3F) as usize] as char
            } else {
                '='
            });
        }

        result
    }

    fn base64_decode(s: &str) -> Result<Vec<u8>, String> {
        // Simple base64 decoding
        let chars: HashMap<char, u8> = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
            .chars()
            .enumerate()
            .map(|(i, c)| (c, i as u8))
            .collect();

        let mut result = Vec::new();
        let bytes: Result<Vec<u8>, String> = s.chars()
            .filter(|&c| c != '=')
            .map(|c| chars.get(&c).copied().ok_or_else(|| "Invalid base64 character".to_string()))
            .collect();
        let bytes = bytes?;

        for chunk in bytes.chunks(4) {
            if chunk.len() < 2 {
                break;
            }

            let b1 = chunk[0];
            let b2 = chunk[1];
            result.push((b1 << 2) | (b2 >> 4));

            if chunk.len() > 2 {
                let b3 = chunk[2];
                result.push((b2 << 4) | (b3 >> 2));

                if chunk.len() > 3 {
                    let b4 = chunk[3];
                    result.push((b3 << 6) | b4);
                }
            }
        }

        Ok(result)
    }
}
