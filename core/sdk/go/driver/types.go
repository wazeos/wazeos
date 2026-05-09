// Package driver provides the stable SDK interface for building WazeOS native Go drivers.
//
// This package defines all the types and interfaces needed to build drivers without
// requiring the core kernel source code. Drivers import this SDK package, and the
// kernel implements these interfaces.
//
// # Example Driver
//
//	package main
//
//	import (
//	    "context"
//	    driver "github.com/wazeos/wazeos/core/sdk/go/driver"
//	)
//
//	type MyDriver struct {}
//
//	func init() {
//	    driver.Register(driver.Spec{
//	        Name:         "my-driver",
//	        Version:      "1.0.0",
//	        Class:        driver.ClassConnect,
//	        URIPattern:   "mydriver://**",
//	        Capabilities: []driver.Capability{driver.CapCall},
//	        Runtime:      driver.RuntimeNative,
//	        Factory:      func() driver.Driver { return &MyDriver{} },
//	    })
//	}
//
//	func (d *MyDriver) URIPattern() string { return "mydriver://**" }
//	func (d *MyDriver) Class() driver.Class { return driver.ClassConnect }
//	func (d *MyDriver) Capabilities() []driver.Capability { return []driver.Capability{driver.CapCall} }
//	func (d *MyDriver) Init(ctx context.Context, config driver.Config) error { return nil }
//	func (d *MyDriver) Close() error { return nil }
//	func (d *MyDriver) Call(ctx driver.Context, req driver.Request) (driver.Response, error) {
//	    return driver.NewResponse(200, []byte("Hello from driver")), nil
//	}
//
package driver

import (
	"context"
	"io"
	"time"
)

// ============================================================================
// Core Driver Interface
// ============================================================================

// Driver is the base interface all drivers must implement.
//
// Drivers extend the WazeOS kernel by handling specific URI patterns.
// They can be written in Go (as plugins) or any language that compiles to WASM.
type Driver interface {
	// Metadata
	URIPattern() string
	Class() Class
	Capabilities() []Capability

	// Lifecycle
	Init(ctx context.Context, config Config) error
	Close() error

	// Core operation (all drivers must implement)
	Call(ctx Context, req Request) (Response, error)
}

// ============================================================================
// Driver Classes
// ============================================================================

// Class defines the type/purpose of a driver
type Class string

const (
	ClassConnect Class = "io.connect" // Client connectors (file, http, db, etc.)
	ClassListen  Class = "io.listen"  // Server listeners (future)
	ClassRuntime Class = "runtime"    // Execution runtimes (wasm, plugin, etc.)
	ClassKernel  Class = "kernel"     // Kernel plugins (future)
)

// ============================================================================
// Driver Capabilities
// ============================================================================

// Capability defines what operations a driver supports
type Capability string

const (
	CapCall   Capability = "call"   // Request/response (required for all drivers)
	CapStream Capability = "stream" // Streaming I/O
	CapHandle Capability = "handle" // Stateful sessions
	CapListen Capability = "listen" // Server binding
	CapPubSub Capability = "pubsub" // Pub/sub messaging
	CapTxn    Capability = "txn"    // Transactions
)

// ============================================================================
// Optional Driver Interfaces
// ============================================================================

// StreamableDriver supports streaming I/O operations
type StreamableDriver interface {
	Driver
	Stream(ctx Context, req Request) (io.ReadWriteCloser, error)
}

// HandleDriver supports stateful sessions
type HandleDriver interface {
	Driver
	CreateHandle(ctx Context, args map[string]any) (Handle, error)
}

// Handle represents a stateful resource session
type Handle interface {
	ID() string                                // Unique identifier
	Call(ctx Context, args map[string]any) (any, error) // Execute operation
	Close() error                              // Release resources
}

// ============================================================================
// Request/Response Types
// ============================================================================

// Request represents an operation request
type Request struct {
	URI       string            `json:"uri"`       // Target URI (e.g., "file:///tmp/test.txt")
	Operation Operation         `json:"operation"` // Operation type
	Args      map[string]any    `json:"args,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Body      []byte            `json:"body,omitempty"`
}

// Response represents an operation result
type Response struct {
	StatusCode int               `json:"status_code"` // HTTP-style status code
	Headers    map[string]string `json:"headers"`
	Body       []byte            `json:"body"`
	Error      string            `json:"error,omitempty"`
}

// Operation defines the type of operation
type Operation string

const (
	OpCall         Operation = "call"
	OpStream       Operation = "stream"
	OpCreateHandle Operation = "create_handle"
	OpCloseHandle  Operation = "close_handle"
)

// ============================================================================
// Context Interface
// ============================================================================

// Context provides access to request metadata and allows nested driver calls
type Context interface {
	context.Context

	// Principal is the authenticated identity (e.g., "mcp-tool:my-tool")
	Principal() string

	// RequestID is the unique identifier for this request
	RequestID() string

	// TraceID is the distributed tracing identifier
	TraceID() string

	// HasPermission checks if the principal has access to the URI
	HasPermission(uri string, perms ...string) bool

	// Call makes a nested call to another driver through the IO Bus
	//
	// This allows drivers to compose functionality:
	//   resp, err := ctx.Call(Request{
	//       URI: "file:///config.json",
	//       Operation: OpCall,
	//   })
	Call(req Request) (Response, error)
}

// ============================================================================
// Configuration
// ============================================================================

// Config holds driver configuration
type Config struct {
	Options     map[string]any // Driver-specific configuration
	Permissions []string       // Permissions this driver needs
	HandleTTL   time.Duration  // TTL for handles (if applicable)
}

// ============================================================================
// Driver Registration
// ============================================================================

// Spec defines a driver registration specification.
//
// This is passed to Register() to add a driver to the system.
type Spec struct {
	// Metadata
	Name    string // Unique driver name (e.g., "file-driver")
	Version string // Version (e.g., "1.0.0")
	Class   Class  // Driver class/purpose

	// Routing
	URIPattern   string       // URI pattern to match (e.g., "file://**")
	Capabilities []Capability // Supported operations

	// Runtime
	Runtime Runtime // How to load this driver ("native" or "wasm")
	Binary  string  // Path to binary (for WASM/plugin runtimes)
	Factory func() Driver // Factory function (for native runtime)

	// Security
	Permissions []string // URIs this driver can access
}

// Runtime specifies how a driver is loaded and executed
type Runtime = string

const (
	RuntimeNative Runtime = "native" // Go plugin compiled into binary
	RuntimeWASM   Runtime = "wasm"   // WebAssembly module
)

// Register registers a driver with the kernel.
//
// This should be called from the driver's init() function:
//
//	func init() {
//	    driver.Register(driver.Spec{
//	        Name:    "my-driver",
//	        Runtime: driver.RuntimeNative,
//	        Factory: func() driver.Driver { return &MyDriver{} },
//	        // ...
//	    })
//	}
//
// The kernel will call this when the driver plugin loads.
var Register = func(spec Spec) {
	// This is a stub that will be overridden by the kernel at runtime.
	// When a driver plugin loads, the kernel provides its own Register
	// implementation that adds the driver to the IO Bus.
	panic("driver.Register called outside of kernel context - this is a bug in the driver loader")
}

// ============================================================================
// Response Helpers
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

// ============================================================================
// Common Errors
// ============================================================================

// Common error status codes
const (
	StatusOK                  = 200
	StatusBadRequest          = 400
	StatusForbidden           = 403
	StatusNotFound            = 404
	StatusInternalServerError = 500
)
