package iobus

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ============================================================================
// IO Bus - Central Routing and Orchestration
// ============================================================================

// IOBus is the central routing layer for all I/O operations
type IOBus struct {
	router         *Router
	sessions       *SessionManager
	drivers        map[string]DriverSpec // Registry of all drivers
	runtimeLoaders map[Runtime]RuntimeLoader // Registry of runtime loaders
	mu             sync.RWMutex
	logger         *slog.Logger
	auditFunc      AuditFunc // Optional audit function
}

// AuditFunc is called for each operation (for logging/monitoring)
type AuditFunc func(ctx Context, event AuditEvent)

// AuditEvent represents an auditable event
type AuditEvent struct {
	Timestamp     time.Time
	Principal     string
	Action        string
	URI           string
	Result        string
	Error         error
	Duration      time.Duration
	BytesTransferred int64
}

// defaultBus is the global IO Bus instance
var defaultBus *IOBus
var defaultBusMu sync.Mutex

// Register registers a driver with the default IO Bus
// This is meant to be called from init() functions in driver packages
func Register(spec DriverSpec) error {
	defaultBusMu.Lock()
	defer defaultBusMu.Unlock()

	if defaultBus == nil {
		defaultBus = NewIOBus(slog.Default())
	}
	return defaultBus.Register(spec)
}

// GetDefaultBus returns the default IO Bus instance
func GetDefaultBus() *IOBus {
	defaultBusMu.Lock()
	defer defaultBusMu.Unlock()

	if defaultBus == nil {
		defaultBus = NewIOBus(slog.Default())
	}
	return defaultBus
}

// NewIOBus creates a new IO Bus
func NewIOBus(logger *slog.Logger) *IOBus {
	bus := &IOBus{
		router:         NewRouter(),
		sessions:       NewSessionManager(logger, 5*time.Minute, 1*time.Hour),
		drivers:        make(map[string]DriverSpec),
		runtimeLoaders: make(map[Runtime]RuntimeLoader),
		logger:         logger,
	}

	// Register built-in runtime loaders
	if err := RegisterNativeLoader(bus); err != nil {
		logger.Error("failed to register native loader", "error", err)
	}

	return bus
}

// SetAuditFunc sets the audit function
func (bus *IOBus) SetAuditFunc(fn AuditFunc) {
	bus.auditFunc = fn
}

// RegisterRuntimeLoader registers a loader for a specific runtime type.
//
// This enables the IO Bus to load drivers from different execution environments
// (native Go, WASM, plugins, etc.). Runtime packages call this at init time
// to register their loader implementation.
//
// Parameters:
//   - runtime: String identifier for the runtime (e.g., "native", "wasm", "plugin")
//   - loader: Implementation of RuntimeLoader for this runtime type
//
// Returns an error if the runtime is already registered.
//
// Example usage (from a runtime package's init() function):
//
//	func init() {
//	    bus := iobus.GetDefaultBus()
//	    bus.RegisterRuntimeLoader("plugin", &pluginLoader{})
//	}
//
// Thread-safe: Can be called concurrently from multiple package init() functions.
func (bus *IOBus) RegisterRuntimeLoader(runtime string, loader RuntimeLoader) error {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	// Check for duplicate registration
	if _, exists := bus.runtimeLoaders[runtime]; exists {
		return fmt.Errorf("runtime loader for %s already registered", runtime)
	}

	// Store the loader
	bus.runtimeLoaders[runtime] = loader

	// Log successful registration
	bus.logger.Info("runtime loader registered", "runtime", runtime)

	return nil
}

// ============================================================================
// Driver Registration
// ============================================================================
//
// Driver registration happens in three phases:
//
// 1. **Validation**: Check that the spec is complete and not duplicate
// 2. **Loading**: Use the RuntimeLoader to create a driver instance
// 3. **Routing**: Register the driver's URI pattern with the router
//
// Flow diagram:
//
//	iobus.Register(spec)
//	    │
//	    ├─> Validate spec (name, URIPattern)
//	    │
//	    ├─> Look up RuntimeLoader for spec.Runtime
//	    │   (e.g., "wasm" → wasmLoader)
//	    │
//	    ├─> loader.LoadDriver(spec, bus)
//	    │   (Creates and initializes driver instance)
//	    │
//	    ├─> router.Register(URIPattern, driver)
//	    │   (Maps URI patterns to driver for request dispatch)
//	    │
//	    └─> Store spec for later introspection
//
// ============================================================================

// Register adds a driver to the IO Bus.
//
// This is the main entry point for registering drivers, whether they're native
// Go code, WASM modules, plugins, or other runtime types. The registration
// process delegates to the appropriate RuntimeLoader based on spec.Runtime.
//
// Parameters:
//   - spec: Complete driver specification including metadata, runtime info,
//           routing patterns, and capabilities
//
// Returns an error if:
//   - Required fields (Name, URIPattern) are missing
//   - Driver name is already registered
//   - Runtime type is not registered (no RuntimeLoader available)
//   - Driver loading fails (invalid binary, initialization error, etc.)
//   - URI pattern registration fails
//
// Example usage:
//
//	// Register a native Go driver
//	iobus.Register(iobus.DriverSpec{
//	    Name:       "file-driver",
//	    Runtime:    "native",
//	    Factory:    func() Driver { return &FileDriver{} },
//	    URIPattern: "file://**",
//	})
//
//	// Register a WASM driver
//	iobus.Register(iobus.DriverSpec{
//	    Name:       "http-driver",
//	    Runtime:    "wasm",
//	    Binary:     "/path/to/http.wasm",
//	    URIPattern: "http://**",
//	})
//
// Thread-safe: Can be called concurrently.
func (bus *IOBus) Register(spec DriverSpec) error {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	// ========== Phase 1: Validation ==========

	// Ensure driver has a name
	if spec.Name == "" {
		return fmt.Errorf("driver name is required")
	}

	// Ensure driver has a URI pattern for routing
	if spec.URIPattern == "" {
		return fmt.Errorf("URI pattern is required")
	}

	// Check for duplicate driver names
	if _, exists := bus.drivers[spec.Name]; exists {
		return fmt.Errorf("driver %s already registered", spec.Name)
	}

	// ========== Phase 2: Loading ==========

	// Look up the RuntimeLoader for this runtime type
	// Example: "wasm" → wasmLoader, "native" → nativeLoader
	runtimeLoader, loaderExists := bus.runtimeLoaders[spec.Runtime]
	if !loaderExists {
		return fmt.Errorf(
			"no loader registered for runtime '%s' (driver: %s)\n"+
				"Hint: Make sure the runtime package is imported (e.g., _ \"package/runtime/%s\")",
			spec.Runtime, spec.Name, spec.Runtime,
		)
	}

	// Delegate to the RuntimeLoader to create the driver instance
	// The loader knows how to:
	//   - Load the driver (from Factory, Binary, etc.)
	//   - Initialize it with proper configuration
	//   - Return a working Driver interface
	driverInstance, err := runtimeLoader.LoadDriver(spec, bus)
	if err != nil {
		return fmt.Errorf("failed to load driver %s (runtime: %s): %w",
			spec.Name, spec.Runtime, err)
	}

	// ========== Phase 3: Routing ==========

	// Register the driver's URI pattern with the router
	// This enables the IO Bus to dispatch incoming requests to this driver
	// based on URI matching (e.g., "file://**" routes to file driver)
	if err := bus.router.Register(spec.URIPattern, driverInstance); err != nil {
		return fmt.Errorf("failed to register URI pattern %s: %w",
			spec.URIPattern, err)
	}

	// Store the spec for introspection (e.g., ListDrivers())
	bus.drivers[spec.Name] = spec

	bus.logger.Info("driver registered",
		"name", spec.Name,
		"version", spec.Version,
		"class", spec.Class,
		"pattern", spec.URIPattern,
		"capabilities", spec.Capabilities,
	)

	return nil
}

// ListDrivers returns all registered drivers
func (bus *IOBus) ListDrivers() []DriverSpec {
	bus.mu.RLock()
	defer bus.mu.RUnlock()

	drivers := make([]DriverSpec, 0, len(bus.drivers))
	for _, spec := range bus.drivers {
		drivers = append(drivers, spec)
	}

	return drivers
}

// ============================================================================
// Core Operations
// ============================================================================

// Call executes an operation on a resource
func (bus *IOBus) Call(ctx Context, req Request) (Response, error) {
	startTime := time.Now()

	// Handle special operations
	switch req.Operation {
	case OpCreateHandle:
		return bus.createHandle(ctx, req)
	case OpCloseHandle:
		return bus.closeHandle(ctx, req)
	case OpReleaseHandle:
		return bus.releaseHandle(ctx, req)
	}

	// Check if URI is a handle
	if isHandleURI(req.URI) {
		return bus.callHandle(ctx, req)
	}

	// Find driver
	driver := bus.router.Match(req.URI)
	if driver == nil {
		bus.audit(ctx, "driver_not_found", req.URI, "", nil, time.Since(startTime), 0)
		return Response{}, ErrDriverNotFound
	}

	// Permission check
	if !ctx.HasPermission(req.URI, "call") {
		bus.audit(ctx, "permission_denied", req.URI, "denied", nil, time.Since(startTime), 0)
		return Response{}, ErrPermissionDenied
	}

	// Route based on operation
	var resp Response
	var err error

	switch req.Operation {
	case OpCall:
		resp, err = driver.Call(ctx, req)

	case OpStream:
		// Check if driver supports streaming
		if sd, ok := driver.(StreamableDriver); ok {
			stream, streamErr := sd.Stream(ctx, req)
			if streamErr != nil {
				err = streamErr
			} else {
				// For now, return a special response indicating streaming
				resp = Response{
					StatusCode: 200,
					Headers: map[string]string{
						"transfer-encoding": "chunked",
					},
					Body: []byte(fmt.Sprintf("streaming:%p", stream)),
				}
			}
		} else {
			err = ErrUnsupportedOp
		}

	default:
		err = ErrUnsupportedOp
	}

	// Audit
	result := "success"
	if err != nil {
		result = "error"
	}
	bytesTransferred := int64(len(resp.Body))
	bus.audit(ctx, string(req.Operation), req.URI, result, err, time.Since(startTime), bytesTransferred)

	return resp, err
}

// ============================================================================
// Handle Operations
// ============================================================================

// createHandle creates a new handle
func (bus *IOBus) createHandle(ctx Context, req Request) (Response, error) {
	startTime := time.Now()

	// Find driver
	driver := bus.router.Match(req.URI)
	if driver == nil {
		bus.audit(ctx, "create_handle", req.URI, "driver_not_found", nil, time.Since(startTime), 0)
		return Response{}, ErrDriverNotFound
	}

	// Check if driver supports handles
	hd, ok := driver.(HandleDriver)
	if !ok {
		bus.audit(ctx, "create_handle", req.URI, "unsupported", nil, time.Since(startTime), 0)
		return Response{}, ErrUnsupportedOp
	}

	// Permission check
	if !ctx.HasPermission(req.URI, "handle") {
		bus.audit(ctx, "create_handle", req.URI, "permission_denied", nil, time.Since(startTime), 0)
		return Response{}, ErrPermissionDenied
	}

	// Create handle
	handle, err := hd.CreateHandle(ctx, req.Args)
	if err != nil {
		bus.audit(ctx, "create_handle", req.URI, "error", err, time.Since(startTime), 0)
		return Response{}, err
	}

	// Parse TTL from headers
	ttl := 1 * time.Hour // Default
	if ttlStr, ok := req.Headers["ttl"]; ok {
		if parsedTTL, err := time.ParseDuration(ttlStr + "s"); err == nil {
			ttl = parsedTTL
		}
	}

	// Store handle
	handleID := bus.sessions.Store(handle, ctx.Principal(), ctx, ttl)

	bus.audit(ctx, "create_handle", req.URI, "success", nil, time.Since(startTime), 0)

	return Response{
		StatusCode: 200,
		Body:       []byte(handleID),
	}, nil
}

// callHandle calls an operation on a handle
func (bus *IOBus) callHandle(ctx Context, req Request) (Response, error) {
	startTime := time.Now()

	// Get handle
	handle, handleCtx, err := bus.sessions.Get(req.URI, ctx.Principal())
	if err != nil {
		bus.audit(ctx, "call_handle", req.URI, "not_found", err, time.Since(startTime), 0)
		return Response{}, err
	}
	defer bus.sessions.Release(req.URI)

	// Call handle with original context (preserves permissions)
	result, err := handle.Call(handleCtx, req.Args)
	if err != nil {
		bus.audit(ctx, "call_handle", req.URI, "error", err, time.Since(startTime), 0)
		return Response{}, err
	}

	// Serialize result
	body, err := serializeResult(result)
	if err != nil {
		return Response{}, err
	}

	bus.audit(ctx, "call_handle", req.URI, "success", nil, time.Since(startTime), int64(len(body)))

	return Response{
		StatusCode: 200,
		Body:       body,
	}, nil
}

// closeHandle closes a handle
func (bus *IOBus) closeHandle(ctx Context, req Request) (Response, error) {
	startTime := time.Now()

	err := bus.sessions.Delete(req.URI)
	if err != nil {
		bus.audit(ctx, "close_handle", req.URI, "error", err, time.Since(startTime), 0)
		return Response{}, err
	}

	bus.audit(ctx, "close_handle", req.URI, "success", nil, time.Since(startTime), 0)

	return Response{
		StatusCode: 200,
	}, nil
}

// releaseHandle releases a handle reference
func (bus *IOBus) releaseHandle(ctx Context, req Request) (Response, error) {
	startTime := time.Now()

	err := bus.sessions.Release(req.URI)
	if err != nil {
		bus.audit(ctx, "release_handle", req.URI, "error", err, time.Since(startTime), 0)
		return Response{}, err
	}

	bus.audit(ctx, "release_handle", req.URI, "success", nil, time.Since(startTime), 0)

	return Response{
		StatusCode: 200,
	}, nil
}

// ============================================================================
// Utilities
// ============================================================================

// audit records an audit event
func (bus *IOBus) audit(ctx Context, action, uri, result string, err error, duration time.Duration, bytesTransferred int64) {
	if bus.auditFunc == nil {
		return
	}

	event := AuditEvent{
		Timestamp:        time.Now(),
		Principal:        ctx.Principal(),
		Action:           action,
		URI:              uri,
		Result:           result,
		Error:            err,
		Duration:         duration,
		BytesTransferred: bytesTransferred,
	}

	bus.auditFunc(ctx, event)
}

// isHandleURI checks if a URI is a handle reference
func isHandleURI(uri string) bool {
	return len(uri) > 17 && uri[:17] == "kernel://session/"
}

// serializeResult serializes a result to bytes
func serializeResult(result any) ([]byte, error) {
	// For now, use JSON serialization
	// TODO: Support MessagePack and other formats
	if b, ok := result.([]byte); ok {
		return b, nil
	}
	if s, ok := result.(string); ok {
		return []byte(s), nil
	}

	// Fall back to JSON encoding for structured data
	return json.Marshal(result)
}

// ============================================================================
// Shutdown
// ============================================================================

// Shutdown closes all drivers and cleans up resources
func (bus *IOBus) Shutdown() error {
	bus.logger.Info("shutting down IO Bus")

	// Shutdown session manager
	if err := bus.sessions.Shutdown(); err != nil {
		return err
	}

	// Close all drivers
	bus.mu.Lock()
	defer bus.mu.Unlock()

	for name, spec := range bus.drivers {
		driver := spec.Factory()
		if err := driver.Close(); err != nil {
			bus.logger.Error("failed to close driver",
				"name", name,
				"error", err,
			)
		} else {
			bus.logger.Info("driver closed", "name", name)
		}
	}

	bus.logger.Info("IO Bus shutdown complete")
	return nil
}
