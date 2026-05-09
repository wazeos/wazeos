package iobus

import (
	"context"
	"fmt"
	"time"
)

// ============================================================================
// Native Runtime Loader
// ============================================================================
//
// The native runtime loader handles "native" runtime drivers - Go code that
// is compiled directly into the binary and runs with full OS access.
//
// How it works:
//   1. Driver code is written in Go as a struct implementing the Driver interface
//   2. DriverSpec provides a Factory function that creates new driver instances
//   3. nativeLoader calls the Factory to get a driver instance
//   4. nativeLoader initializes the driver with Config
//   5. Returns the initialized driver to the IO Bus
//
// Example driver registration using native runtime:
//
//	type FileDriver struct { /* ... */ }
//
//	func (d *FileDriver) Init(ctx context.Context, config Config) error { /* ... */ }
//	func (d *FileDriver) Call(ctx Context, req Request) (Response, error) { /* ... */ }
//	// ... other Driver interface methods
//
//	func main() {
//	    iobus.Register(iobus.DriverSpec{
//	        Name:       "file-driver",
//	        Runtime:    "native",
//	        Factory:    func() Driver { return &FileDriver{} },
//	        URIPattern: "file://**",
//	    })
//	}
//
// Advantages of native runtime:
//   - Maximum performance (no sandbox overhead)
//   - Full access to OS APIs and Go ecosystem
//   - Direct memory sharing with IO Bus
//   - Compile-time type safety
//
// Disadvantages:
//   - No sandboxing (drivers have full system access)
//   - Can't hot-reload (must restart process)
//   - Must be Go code (no multi-language support)
//
// ============================================================================

// nativeLoader implements RuntimeLoader for native Go drivers.
//
// Native drivers are Go structs compiled into the binary. They're created
// via a Factory function provided in the DriverSpec and initialized with
// a Config containing permissions and options.
type nativeLoader struct{}

// LoadDriver creates and initializes a native Go driver from a spec.
//
// Process:
//   1. Validate that spec.Factory is provided
//   2. Call Factory() to create a new driver instance
//   3. Build a Config with permissions and default settings
//   4. Call driver.Init(ctx, config) to initialize it
//   5. Return the initialized driver
//
// Parameters:
//   - spec: Must contain a Factory function that returns a Driver
//   - bus: The IO Bus (not used by native loader, but required by interface)
//
// Returns:
//   - A fully initialized Driver ready to handle requests
//   - Error if Factory is missing or initialization fails
//
// Example spec for native driver:
//
//	DriverSpec{
//	    Name:        "file-driver",
//	    Runtime:     "native",
//	    Factory:     func() Driver { return &FileDriver{} },
//	    Permissions: []string{"file://**"},
//	}
func (l *nativeLoader) LoadDriver(spec DriverSpec, bus *IOBus) (Driver, error) {
	// ========== Validation ==========

	// Native drivers MUST provide a Factory function
	// This is the function that creates new instances of the driver
	if spec.Factory == nil {
		return nil, fmt.Errorf(
			"native driver %s requires Factory function\n"+
				"Example: Factory: func() Driver { return &MyDriver{} }",
			spec.Name,
		)
	}

	// ========== Driver Creation ==========

	// Call the Factory function to create a new driver instance
	// The Factory returns a zero-value driver that needs initialization
	driverInstance := spec.Factory()

	// ========== Configuration ==========

	// Build initialization config
	// - Options: Driver-specific settings (currently empty, but extensible)
	// - Permissions: URIs this driver is allowed to access
	// - HandleTTL: Default timeout for stateful handle sessions
	config := Config{
		Options:     make(map[string]any),
		Permissions: spec.Permissions,
		HandleTTL:   1 * time.Hour, // Default TTL for handles
	}

	// ========== Initialization ==========

	// Initialize the driver with the config
	// This is where the driver:
	//   - Sets up internal state
	//   - Validates permissions
	//   - Opens resources (files, connections, etc.)
	//   - Prepares to handle requests
	ctx := context.Background()
	if err := driverInstance.Init(ctx, config); err != nil {
		return nil, fmt.Errorf("failed to initialize native driver %s: %w",
			spec.Name, err)
	}

	// Return the fully initialized driver
	// It's now ready to receive requests via Call()
	return driverInstance, nil
}

// RegisterNativeLoader registers the native runtime loader with the IO Bus.
//
// This function is called automatically during IO Bus initialization (in NewIOBus).
// It makes the "native" runtime type available for driver registration.
//
// After this runs, drivers can be registered with Runtime: "native"
//
// Parameters:
//   - bus: The IO Bus instance to register with
//
// Returns an error if "native" runtime is already registered.
func RegisterNativeLoader(bus *IOBus) error {
	return bus.RegisterRuntimeLoader("native", &nativeLoader{})
}
