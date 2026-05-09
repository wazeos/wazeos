package iobus

import (
	"context"
	"io"
	"time"
)

// ============================================================================
// Core Types
// ============================================================================

// Driver is the base interface all drivers must implement
type Driver interface {
	// Metadata
	URIPattern() string
	Class() DriverClass
	Capabilities() []Capability

	// Lifecycle
	Init(ctx context.Context, config Config) error
	Close() error

	// Core operation (all drivers must implement)
	Call(ctx Context, req Request) (Response, error)
}

// DriverClass defines the type of driver
type DriverClass string

const (
	ConnectDriver DriverClass = "io.connect" // Client connectors
	ListenDriver  DriverClass = "io.listen"  // Server listeners
	RuntimeDriver DriverClass = "runtime"    // Execution runtimes
	KernelDriver  DriverClass = "kernel"     // Kernel plugins
)

// Capability defines what operations a driver supports
type Capability string

const (
	CapCall   Capability = "call"   // Request/response
	CapStream Capability = "stream" // Streaming I/O
	CapHandle Capability = "handle" // Stateful sessions
	CapListen Capability = "listen" // Server binding
	CapPubSub Capability = "pubsub" // Pub/sub messaging
	CapTxn    Capability = "txn"    // Transactions
)

// ============================================================================
// Optional Driver Interfaces
// ============================================================================

// StreamableDriver supports streaming I/O
type StreamableDriver interface {
	Driver
	Stream(ctx Context, req Request) (io.ReadWriteCloser, error)
}

// HandleDriver supports stateful sessions
type HandleDriver interface {
	Driver
	CreateHandle(ctx Context, args map[string]any) (Handle, error)
}

// ListenableDriver supports server binding
type ListenableDriver interface {
	Driver
	Listen(ctx context.Context, addr string) error
}

// PubSubDriver supports publish/subscribe
type PubSubDriver interface {
	Driver
	Subscribe(ctx Context, topic string) (<-chan Message, error)
	Publish(ctx Context, topic string, msg Message) error
}

// TransactionalDriver supports transactions
type TransactionalDriver interface {
	Driver
	BeginTxn(ctx Context) (Transaction, error)
}

// ============================================================================
// Handle Interface
// ============================================================================

// Handle represents a stateful resource session
type Handle interface {
	// Unique identifier (format: "kernel://session/{uuid}")
	ID() string

	// Execute operation on the resource
	Call(ctx Context, args map[string]any) (any, error)

	// Release resources
	Close() error
}

// StreamableHandle supports streaming operations
type StreamableHandle interface {
	Handle
	Stream(ctx Context, mode StreamMode) (io.ReadWriteCloser, error)
}

// StreamMode defines the direction of streaming
type StreamMode int

const (
	StreamRead StreamMode = iota
	StreamWrite
	StreamReadWrite
)

// ============================================================================
// Request/Response Types
// ============================================================================

// Request represents an operation request
type Request struct {
	// Target URI (e.g., "file:///tmp/test.txt", "s3://bucket/key")
	URI string `json:"uri"`

	// Operation type
	Operation Operation `json:"operation"`

	// Optional: Structured arguments
	Args map[string]any `json:"args,omitempty"`

	// Optional: Headers (protocol-specific metadata)
	Headers map[string]string `json:"headers,omitempty"`

	// Optional: Binary body
	Body []byte `json:"body,omitempty"`
}

// Response represents an operation result
type Response struct {
	// HTTP-style status code
	StatusCode int `json:"status_code"`

	// Response headers
	Headers map[string]string `json:"headers"`

	// Response body
	Body []byte `json:"body"`

	// Optional: Error message
	Error string `json:"error,omitempty"`
}

// Operation defines the type of operation
type Operation string

const (
	OpCall         Operation = "call"          // One-shot call
	OpStream       Operation = "stream"        // Streaming operation
	OpCreateHandle Operation = "create_handle" // Create handle
	OpCloseHandle  Operation = "close_handle"  // Close handle
	OpReleaseHandle Operation = "release_handle" // Release handle (decrement ref count)
)

// Message represents a pub/sub message
type Message struct {
	Topic   string
	Payload []byte
	Headers map[string]string
}

// Transaction represents a transactional context
type Transaction interface {
	Commit() error
	Rollback() error
}

// ============================================================================
// Context
// ============================================================================

// Context provides access to request metadata and IO Bus
type Context interface {
	context.Context

	// Principal is the authenticated identity (e.g., "mcp-tool:transcribe-audio")
	Principal() string

	// RequestID is the unique identifier for this request
	RequestID() string

	// TraceID is the distributed tracing identifier
	TraceID() string

	// HasPermission checks if the principal has access to the URI with given permissions
	HasPermission(uri string, perms ...string) bool

	// IOBus returns the IO Bus for making nested calls
	IOBus() *IOBus

	// WithValue returns a new context with the given key-value pair
	WithValue(key, value any) Context
}

// ============================================================================
// Configuration
// ============================================================================

// Config holds driver configuration
type Config struct {
	// Driver-specific configuration
	Options map[string]any

	// Permissions this driver needs
	Permissions []string

	// TTL for handles created by this driver (if applicable)
	HandleTTL time.Duration
}

// DriverSpec defines a driver registration with the IO Bus.
// It contains all metadata needed to load, route, and execute a driver.
//
// The spec is used in two phases:
//   1. Registration: Passed to iobus.Register() to add driver to the system
//   2. Loading: Passed to RuntimeLoader.LoadDriver() to create driver instance
//
// Example - Native Go driver:
//
//	iobus.Register(iobus.DriverSpec{
//	    Name:         "file-driver",
//	    Runtime:      "native",
//	    Factory:      func() Driver { return &FileDriver{} },
//	    URIPattern:   "file://**",
//	    Capabilities: []Capability{CapCall},
//	})
//
// Example - WASM driver:
//
//	iobus.Register(iobus.DriverSpec{
//	    Name:       "http-driver",
//	    Runtime:    "wasm",
//	    Binary:     "/path/to/http_driver.wasm",
//	    URIPattern: "http://**",
//	})
type DriverSpec struct {
	// ========== Metadata ==========

	// Name uniquely identifies this driver (e.g., "file-driver-wasm")
	Name string

	// Version for compatibility tracking (e.g., "1.0.0")
	Version string

	// Class categorizes the driver's purpose
	//   - ConnectDriver: Client I/O (file, http, shell, etc.)
	//   - RuntimeDriver: Execution environment (wasm, plugin, etc.)
	//   - ListenDriver: Server/listener (future)
	//   - KernelDriver: System extensions (future)
	Class DriverClass

	// ========== Routing ==========

	// URIPattern defines which URIs this driver handles
	// Uses glob-style matching (e.g., "file://**", "http://api.*/v1/**")
	// The IO Bus router uses this to dispatch requests
	URIPattern string

	// Capabilities declares what operations the driver supports
	// Common: CapCall (request/response), CapHandle (stateful sessions)
	Capabilities []Capability

	// ========== Runtime Configuration ==========

	// Runtime specifies how to load this driver (e.g., "native", "wasm")
	// The IO Bus looks up the corresponding RuntimeLoader and delegates
	Runtime Runtime

	// Binary is the path to the driver binary (used by WASM, plugin runtimes)
	// For "wasm" runtime: path to .wasm file
	// For "native" runtime: not used (use Factory instead)
	Binary string

	// Factory creates native Go driver instances (used by "native" runtime)
	// For "native" runtime: function that returns new driver instance
	// For "wasm" runtime: not used (use Binary instead)
	//
	// Example:
	//   Factory: func() Driver { return &MyDriver{} }
	Factory func() Driver

	// ========== Security ==========

	// Permissions lists what URIs this driver can access
	// Used during driver initialization to set up security context
	// Example: []string{"file://**", "http://api.example.com/**"}
	Permissions []string
}

// ============================================================================
// Runtime System
// ============================================================================
//
// The runtime system provides a pluggable architecture for loading and executing
// drivers in different execution environments. This enables:
//
// 1. **Native Go Drivers**: Compiled into the binary, fast, full OS access
// 2. **WASM Drivers**: Sandboxed, portable, multi-language support
// 3. **Future Runtimes**: Plugins, containers, remote processes, etc.
//
// Architecture:
//
//	┌─────────────────────────────────────────────────────────┐
//	│ Application Code                                        │
//	│ iobus.Register(DriverSpec{Runtime: "wasm", ...})       │
//	└──────────────────────┬──────────────────────────────────┘
//	                       │
//	                       ▼
//	┌─────────────────────────────────────────────────────────┐
//	│ IO Bus Registry                                         │
//	│ - Looks up RuntimeLoader for "wasm"                    │
//	│ - Calls loader.LoadDriver(spec, bus)                  │
//	└──────────────────────┬──────────────────────────────────┘
//	                       │
//	                       ▼
//	┌─────────────────────────────────────────────────────────┐
//	│ Runtime Loader (e.g., wasmLoader)                      │
//	│ - Validates spec (e.g., Binary path exists)            │
//	│ - Creates driver instance (e.g., loads WASM module)    │
//	│ - Returns initialized Driver interface                 │
//	└─────────────────────────────────────────────────────────┘
//
// Adding a New Runtime (Example: Go Plugins):
//
//	// 1. Create loader package
//	package plugin
//
//	type pluginLoader struct{}
//
//	func (l *pluginLoader) LoadDriver(spec iobus.DriverSpec, bus *iobus.IOBus) (iobus.Driver, error) {
//	    p, err := plugin.Open(spec.Binary)
//	    // ... load and return driver
//	}
//
//	// 2. Register at init time
//	func init() {
//	    bus := iobus.GetDefaultBus()
//	    bus.RegisterRuntimeLoader("plugin", &pluginLoader{})
//	}
//
//	// 3. Use immediately - zero core edits!
//	iobus.Register(iobus.DriverSpec{
//	    Runtime: "plugin",
//	    Binary:  "my-driver.so",
//	})
//
// ============================================================================

// Runtime specifies how a driver is loaded and executed.
// It's a string identifier (e.g., "native", "wasm", "plugin") that maps to
// a RuntimeLoader implementation. Runtime types are dynamically registered,
// so new execution environments can be added without modifying core code.
//
// Common runtime types:
//   - "native": Go code compiled into the binary (via Factory function)
//   - "wasm": WebAssembly modules loaded from disk (via Binary path)
//   - Future: "plugin", "docker", "remote", etc.
type Runtime = string

// RuntimeLoader is the interface that execution environments must implement
// to load and initialize drivers. Each runtime type (native, WASM, etc.)
// provides its own loader that knows how to:
//   1. Validate the DriverSpec for that runtime
//   2. Load/create the driver instance
//   3. Initialize it and return a working Driver interface
//
// Loaders are registered at package init time using bus.RegisterRuntimeLoader(),
// making the runtime system fully dynamic and extensible.
type RuntimeLoader interface {
	// LoadDriver creates and initializes a driver instance from a spec.
	//
	// Parameters:
	//   - spec: The driver specification containing runtime-specific info
	//           (e.g., Binary path for WASM, Factory for native)
	//   - bus: The IO Bus instance (needed for driver initialization)
	//
	// Returns:
	//   - Driver: A fully initialized driver ready to handle requests
	//   - error: If loading or initialization fails
	//
	// Example implementations:
	//   - Native loader: Calls spec.Factory(), then driver.Init()
	//   - WASM loader: Loads spec.Binary, creates WASMDriver wrapper
	LoadDriver(spec DriverSpec, bus *IOBus) (Driver, error)
}

// ============================================================================
// Errors
// ============================================================================

// Common errors
var (
	ErrDriverNotFound     = NewError(404, "driver not found for URI")
	ErrPermissionDenied   = NewError(403, "permission denied")
	ErrHandleNotFound     = NewError(404, "handle not found")
	ErrHandleExpired      = NewError(410, "handle expired")
	ErrUnsupportedOp      = NewError(400, "unsupported operation")
	ErrInvalidArgs        = NewError(400, "invalid arguments")
)

// Error represents an IO Bus error
type Error struct {
	Code    int
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

// NewError creates a new error
func NewError(code int, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

// NewResponse creates a successful response
func NewResponse(statusCode int, body []byte) Response {
	return Response{
		StatusCode: statusCode,
		Headers:    make(map[string]string),
		Body:       body,
	}
}

// NewErrorResponse creates an error response
func NewErrorResponse(statusCode int, message string) Response {
	return Response{
		StatusCode: statusCode,
		Headers:    make(map[string]string),
		Body:       []byte(message),
		Error:      message,
	}
}
