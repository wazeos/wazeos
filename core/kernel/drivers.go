package kernel

import (
	"path/filepath"

	"github.com/wazeos/wazeos/core/kernel/iobus"

	// Blank import auto-registers the WASM runtime system at init time
	// This runs the init() function in drivers/runtime/wasm/driver.go which:
	//   1. Registers the WASM runtime loader (enables Runtime: "wasm")
	//   2. Registers the WASM runtime driver (provides WASM execution environment)
	_ "github.com/wazeos/wazeos/drivers/runtime/wasm"
)

// ============================================================================
// System Driver Registration
// ============================================================================
//
// InitDrivers registers all system drivers with the IO Bus.
// This includes both native Go drivers and WASM drivers.
//
// Driver Loading Order:
//
//	1. Package init() functions run (via blank imports)
//	   - WASM runtime driver registers itself
//	   - WASM runtime loader registers itself
//	   - Native runtime loader is already registered in NewIOBus()
//
//	2. InitDrivers() runs
//	   - Registers native I/O drivers (fallback implementations)
//	   - Registers WASM I/O drivers (primary implementations)
//
// WASM Drivers provide:
//   - Sandboxed, portable, multi-language support
//   - URIs: file://** http://** shell://**
//   - Loaded from: drivers/*/target/wasm32-wasip1/release/*.wasm
//
// ============================================================================

// InitDrivers registers all system WASM drivers with the IO Bus.
//
// This function should be called during kernel initialization, after the
// IO Bus is created and runtime loaders are registered.
//
// If WASM driver registration fails (binary not found, load error, etc.),
// the system will log the error but continue to function.
//
// Returns an error if any critical driver registration fails.
func InitDrivers() error {
	// ========== WASM Drivers ==========
	//
	// Register WebAssembly implementations of I/O drivers
	// These are the primary drivers that provide sandboxed execution
	// They use standard URI schemes (file://, http://, shell://)
	//
	// Note: The WASM runtime driver was auto-registered via blank import above
	// It handles the wasm:// protocol and provides WASM execution environment

	// Define WASM drivers to load
	wasmDrivers := []struct {
		name       string       // Driver identifier
		uriPattern string       // URI pattern to handle (e.g., "file://**")
		binary     string       // Path to .wasm file (relative to project root)
	}{
		{
			name:       "shell-driver-wasm",
			uriPattern: "shell://**",
			binary:     "drivers/shell/target/wasm32-wasip1/release/wazeos_shell_driver.wasm",
		},
		{
			name:       "http-driver-wasm",
			uriPattern: "http://**",
			binary:     "drivers/http/target/wasm32-wasip1/release/wazeos_http_driver.wasm",
		},
		{
			name:       "file-driver-wasm",
			uriPattern: "file://**",
			binary:     "drivers/file/target/wasm32-wasip1/release/wazeos_file_driver.wasm",
		},
	}

	// Register each WASM driver
	for _, driverSpec := range wasmDrivers {
		// Convert relative path to absolute path
		// WASM loader needs absolute paths to load binaries
		absPath, err := filepath.Abs(driverSpec.binary)
		if err != nil {
			return err
		}

		// Register the WASM driver
		// This will:
		//   1. Look up the "wasm" RuntimeLoader (registered in init)
		//   2. Call wasmLoader.LoadDriver() which:
		//      - Loads the .wasm binary from disk
		//      - Creates a WASM module handle via wasm-runtime driver
		//      - Initializes the driver by calling driver_init()
		//   3. Register the URI pattern with the router
		if err := iobus.Register(iobus.DriverSpec{
			Name:    driverSpec.name,
			Version: "1.0.0",

			// I/O driver (not a runtime driver)
			Class: iobus.ConnectDriver,

			// URI pattern this driver handles
			// Example: "file://**" matches all file:// URIs
			URIPattern: driverSpec.uriPattern,

			// Currently only supports simple request/response
			// Future: CapHandle for stateful file handles, etc.
			Capabilities: []iobus.Capability{iobus.CapCall},

			// Use WASM runtime loader
			// This tells IO Bus to use wasmLoader.LoadDriver()
			Runtime: "wasm",

			// Path to compiled .wasm binary
			// Built by: cd drivers/X && cargo build --release --target wasm32-wasip1
			Binary: absPath,
		}); err != nil {
			return err
		}
	}

	return nil
}
