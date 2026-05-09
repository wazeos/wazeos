package wasm

import (
	"fmt"

	"github.com/wazeos/wazeos/core/kernel/iobus"
)

// ============================================================================
// WASM Runtime Loader
// ============================================================================
//
// The WASM runtime loader handles "wasm" runtime drivers - WebAssembly modules
// that are loaded from disk and executed in a sandboxed environment.
//
// Architecture:
//
//	┌────────────────────────────────────────────────────────────┐
//	│ WASM Driver Binary (e.g., http_driver.wasm)               │
//	│ - Compiled from Rust/C/AssemblyScript/etc.                │
//	│ - Exports: driver_metadata, driver_init, driver_call      │
//	│ - Imports: host_iobus_call (for making nested IO calls)   │
//	└──────────────────────┬─────────────────────────────────────┘
//	                       │
//	                       │ wasmLoader.LoadDriver()
//	                       ▼
//	┌────────────────────────────────────────────────────────────┐
//	│ iobus.WASMDriver (Go wrapper)                             │
//	│ - Loads the .wasm binary                                   │
//	│ - Creates WASM module instance via wasm-runtime driver    │
//	│ - Exposes Driver interface to IO Bus                      │
//	│ - Marshals requests to/from JSON for WASM                 │
//	└────────────────────────────────────────────────────────────┘
//
// How it works:
//   1. DriverSpec provides path to .wasm file via Binary field
//   2. wasmLoader delegates to iobus.NewWASMDriver()
//   3. NewWASMDriver loads the binary and creates a handle via wasm-runtime driver
//   4. NewWASMDriver calls driver_metadata and driver_init in the WASM module
//   5. Returns WASMDriver wrapper that implements Driver interface
//
// Example driver registration using WASM runtime:
//
//	iobus.Register(iobus.DriverSpec{
//	    Name:       "http-driver",
//	    Runtime:    "wasm",
//	    Binary:     "/path/to/http_driver.wasm",
//	    URIPattern: "http://**",
//	})
//
// Advantages of WASM runtime:
//   - Sandboxed execution (drivers can't access OS without permission)
//   - Multi-language support (Rust, C, AssemblyScript, etc.)
//   - Portable (same binary works on any platform)
//   - Hot-reloadable (can reload WASM without restarting)
//
// Disadvantages:
//   - Performance overhead (sandbox boundary crossing)
//   - Limited OS access (must go through host functions)
//   - More complex development (need WASM toolchain)
//
// WASM Driver Contract:
//
// A WASM driver must export these functions:
//
//	// Returns JSON with driver metadata (URIPattern, Class, Capabilities)
//	driver_metadata() -> *const u8  // JSON string pointer
//
//	// Initializes the driver with config JSON
//	driver_init(config_ptr: u32, config_len: u32) -> u32  // 0 = success
//
//	// Handles a request (JSON in, JSON out)
//	driver_call(request_ptr: u32, request_len: u32) -> *const u8  // JSON response
//
// And may import these host functions:
//
//	// Make nested IO Bus calls from WASM
//	host_iobus_call(request_ptr: u32, request_len: u32) -> u64  // ptr:len packed
//
//	// Create handles to other drivers
//	host_iobus_create_handle(uri_ptr: u32, uri_len: u32) -> u64  // ptr:len packed
//
//	// Close handles
//	host_iobus_close_handle(uri_ptr: u32, uri_len: u32) -> u32  // 0 = success
//
// ============================================================================

// wasmLoader implements RuntimeLoader for WebAssembly drivers.
//
// WASM drivers are portable, sandboxed modules that can be written in any
// language that compiles to WebAssembly (Rust, C, AssemblyScript, etc.).
//
// The loader delegates most work to iobus.NewWASMDriver(), which handles:
//   - Loading the .wasm binary from disk
//   - Creating a WASM module handle via the wasm-runtime driver
//   - Calling driver initialization functions
//   - Wrapping the module in a Driver interface
type wasmLoader struct{}

// LoadDriver creates a WASM driver instance from a binary file.
//
// Process:
//   1. Validate that spec.Binary path is provided
//   2. Delegate to iobus.NewWASMDriver() which:
//      - Loads the .wasm binary from disk
//      - Creates a WASM runtime handle
//      - Calls driver_metadata() to get driver info
//      - Calls driver_init() to initialize the driver
//      - Returns a WASMDriver wrapper
//   3. Return the wrapper (implements Driver interface)
//
// Parameters:
//   - spec: Must contain Binary path to .wasm file
//   - bus: The IO Bus instance (needed to create WASM handles)
//
// Returns:
//   - iobus.WASMDriver wrapper around the loaded WASM module
//   - Error if Binary is missing or loading fails
//
// Example spec for WASM driver:
//
//	DriverSpec{
//	    Name:       "http-driver",
//	    Runtime:    "wasm",
//	    Binary:     "/path/to/http_driver.wasm",
//	    URIPattern: "http://**",
//	}
//
// Note: The actual WASM loading and execution is handled by the wasm-runtime
// driver (see v2/drivers/runtime/wasm/driver.go), which uses the wazero
// library to provide a sandboxed WebAssembly environment.
func (l *wasmLoader) LoadDriver(spec iobus.DriverSpec, bus *iobus.IOBus) (iobus.Driver, error) {
	// ========== Validation ==========

	// WASM drivers MUST provide a Binary path
	// This is the path to the compiled .wasm file on disk
	if spec.Binary == "" {
		return nil, fmt.Errorf(
			"WASM driver %s requires Binary path\n"+
				"Example: Binary: \"/path/to/driver.wasm\"",
			spec.Name,
		)
	}

	// ========== Driver Loading ==========

	// Delegate to iobus.NewWASMDriver() to create the wrapper
	// This function:
	//   1. Reads the .wasm binary from disk
	//   2. Makes a Call to wasm-runtime driver to create a handle
	//   3. Calls driver_metadata() to get driver info
	//   4. Calls driver_init() to initialize the WASM driver
	//   5. Returns a WASMDriver wrapper that implements Driver interface
	//
	// The WASMDriver wrapper handles:
	//   - Marshaling Request/Response to/from JSON for WASM
	//   - Managing the WASM module lifecycle
	//   - Providing host functions for nested IO Bus calls
	wasmDriver, err := iobus.NewWASMDriver(spec, bus)
	if err != nil {
		return nil, fmt.Errorf("failed to create WASM driver %s: %w",
			spec.Name, err)
	}

	// Return the WASM driver wrapper
	// It implements the Driver interface and can handle requests
	return wasmDriver, nil
}

// RegisterWASMLoader registers the WASM runtime loader with the IO Bus.
//
// This function should be called from init() in the wasm runtime driver package.
// It makes the "wasm" runtime type available for driver registration.
//
// After this runs, drivers can be registered with Runtime: "wasm"
//
// Parameters:
//   - bus: The IO Bus instance to register with
//
// Returns an error if "wasm" runtime is already registered.
//
// Note: This is called from v2/drivers/runtime/wasm/driver.go init() function,
// which ensures the WASM runtime driver is loaded before any WASM drivers
// are registered.
func RegisterWASMLoader(bus *iobus.IOBus) error {
	return bus.RegisterRuntimeLoader("wasm", &wasmLoader{})
}
