package wasm

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/wazeos/wazeos/core/kernel/iobus"
)

// Driver implements the WASM runtime driver for loading and executing WebAssembly modules
type Driver struct {
	runtime    wazero.Runtime
	mu         sync.RWMutex
	config     iobus.Config
	uriPattern string
	class      iobus.DriverClass
	caps       []iobus.Capability
}

// WASMHandle represents a loaded WASM module instance
type WASMHandle struct {
	id       string
	module   api.Module
	runtime  wazero.Runtime
	compiled wazero.CompiledModule
	ctx      iobus.Context
}

// init registers both the WASM runtime loader AND the WASM runtime driver.
//
// This is called automatically when the package is imported. Two registrations happen:
//
// 1. **WASM Runtime Loader** ("wasm" runtime type)
//    - Registers the RuntimeLoader that handles Runtime: "wasm" drivers
//    - Enables I/O drivers to be loaded from .wasm binaries
//    - Example: http-driver.wasm, file-driver.wasm, etc.
//
// 2. **WASM Runtime Driver** (wasm:// protocol handler)
//    - A native Go driver that provides WASM execution environment
//    - Uses wazero library to load and run WebAssembly modules
//    - Handles URIs like "wasm://load" to create WASM module instances
//    - Provides host functions (host_iobus_call, etc.) to WASM modules
//
// Why both are needed:
//
//	┌─────────────────────────────────────────────────────────────┐
//	│ WASM I/O Driver (e.g., http-driver.wasm)                   │
//	│ Runtime: "wasm"  ← Uses WASM runtime loader                │
//	└──────────────────────┬──────────────────────────────────────┘
//	                       │
//	                       │ Loaded by WASM runtime loader
//	                       │ which calls...
//	                       ▼
//	┌─────────────────────────────────────────────────────────────┐
//	│ WASM Runtime Driver (this package)                         │
//	│ URIPattern: "wasm://**"  ← Handles WASM execution          │
//	│ Runtime: "native"  ← Uses native runtime loader            │
//	└─────────────────────────────────────────────────────────────┘
//
// Import this package with a blank import to auto-register:
//
//	import _ "github.com/wazeos/wazeos/drivers/runtime/wasm"
//
func init() {
	// ========== Step 1: Register WASM Runtime Loader ==========
	//
	// This enables drivers to use Runtime: "wasm"
	// When iobus.Register sees Runtime: "wasm", it will use this loader
	// to load drivers from .wasm binary files
	bus := iobus.GetDefaultBus()
	if err := RegisterWASMLoader(bus); err != nil {
		panic(fmt.Sprintf("failed to register WASM runtime loader: %v", err))
	}

	// ========== Step 2: Register WASM Runtime Driver ==========
	//
	// This is the actual driver that executes WASM modules
	// It's a native Go driver (Runtime: "native") that uses wazero
	// to provide WebAssembly execution environment
	//
	// This driver handles requests to:
	//   - Load WASM binaries: OpCreateHandle to create module instances
	//   - Execute WASM functions: OpCall to invoke exported functions
	//   - Cleanup: OpCloseHandle to release module instances
	iobus.Register(iobus.DriverSpec{
		Name:    "wasm-runtime",
		Version: "1.0.0",

		// This is a RuntimeDriver - it provides execution environment
		// (not an I/O driver like file/http/shell)
		Class: iobus.RuntimeDriver,

		// Handle URIs like "wasm://load", "wasm://execute", etc.
		URIPattern: "wasm://**",

		// Supports CapHandle for stateful WASM module instances
		// (each loaded WASM module is a handle that can be called)
		Capabilities: []iobus.Capability{iobus.CapHandle},

		// This driver itself is native Go code
		// Runtime drivers are always native - they provide the execution
		// environment for other runtimes
		Runtime: "native",

		// Factory creates new WASM runtime driver instances
		Factory: func() iobus.Driver {
			return &Driver{
				uriPattern: "wasm://**",
				class:      iobus.RuntimeDriver,
				caps:       []iobus.Capability{iobus.CapHandle},
			}
		},
	})
}

// RegisterWASMRuntime registers the WASM runtime driver and loader with a custom IOBus.
// This is useful for creating isolated test environments with wazeos dev run.
func RegisterWASMRuntime(bus *iobus.IOBus) error {
	// Register the WASM runtime loader
	if err := RegisterWASMLoader(bus); err != nil {
		return fmt.Errorf("failed to register WASM loader: %w", err)
	}

	// Register the WASM runtime driver
	return bus.Register(iobus.DriverSpec{
		Name:         "wasm-runtime",
		Version:      "1.0.0",
		Class:        iobus.RuntimeDriver,
		URIPattern:   "wasm://**",
		Capabilities: []iobus.Capability{iobus.CapHandle},
		Runtime:      "native",
		Factory: func() iobus.Driver {
			return &Driver{
				uriPattern: "wasm://**",
				class:      iobus.RuntimeDriver,
				caps:       []iobus.Capability{iobus.CapHandle},
			}
		},
	})
}

// ============================================================================
// Driver Interface Implementation
// ============================================================================

// URIPattern returns the URI pattern this driver handles
func (d *Driver) URIPattern() string {
	return d.uriPattern
}

// Class returns the driver class
func (d *Driver) Class() iobus.DriverClass {
	return d.class
}

// Capabilities returns the capabilities this driver supports
func (d *Driver) Capabilities() []iobus.Capability {
	return d.caps
}

// Init initializes the driver
func (d *Driver) Init(ctx context.Context, config iobus.Config) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.config = config
	d.runtime = wazero.NewRuntime(ctx)

	// Instantiate WASI for all modules
	wasi_snapshot_preview1.MustInstantiate(ctx, d.runtime)

	return nil
}

// Close shuts down the driver
func (d *Driver) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.runtime != nil {
		return d.runtime.Close(context.Background())
	}
	return nil
}

// Call is not used for runtime drivers (they only support handle operations)
func (d *Driver) Call(ctx iobus.Context, req iobus.Request) (iobus.Response, error) {
	return iobus.NewErrorResponse(400, "WASM runtime driver only supports handle operations"), nil
}

// ============================================================================
// HandleDriver Interface Implementation
// ============================================================================

// CreateHandle loads and instantiates a WASM module
func (d *Driver) CreateHandle(ctx iobus.Context, args map[string]any) (iobus.Handle, error) {
	// Get WASM binary from args
	wasmBytes, ok := args["binary"].([]byte)
	if !ok {
		// Try to get from "wasm" key
		wasmBytes, ok = args["wasm"].([]byte)
		if !ok {
			return nil, fmt.Errorf("wasm binary required in args['binary'] or args['wasm']")
		}
	}

	if len(wasmBytes) == 0 {
		return nil, fmt.Errorf("wasm binary is empty")
	}

	// Create a new runtime for this handle (isolates each WASM module)
	handleRuntime := wazero.NewRuntime(ctx)

	// Instantiate WASI
	wasi_snapshot_preview1.MustInstantiate(ctx, handleRuntime)

	// Compile the WASM module
	compiled, err := handleRuntime.CompileModule(ctx, wasmBytes)
	if err != nil {
		handleRuntime.Close(ctx)
		return nil, fmt.Errorf("failed to compile WASM module: %w", err)
	}

	// Create module configuration
	config := wazero.NewModuleConfig().
		WithStdout(nil).
		WithStderr(nil)

	// Get the IO Bus reference
	bus := ctx.IOBus()

	// Create a host module with IO Bus functions
	// Note: Must be named "env" to match WASM imports
	hostBuilder := handleRuntime.NewHostModuleBuilder("env")

	// Export host_iobus_call function
	hostBuilder.NewFunctionBuilder().
		WithFunc(func(ctxParam context.Context, m api.Module, ptr, length uint32) uint64 {
			return hostIOBusCall(ctxParam, m, ptr, length, ctx, bus)
		}).
		Export("host_iobus_call")

	// Export host_iobus_create_handle function
	hostBuilder.NewFunctionBuilder().
		WithFunc(func(ctxParam context.Context, m api.Module, ptr, length uint32) uint64 {
			return hostIOBusCreateHandle(ctxParam, m, ptr, length, ctx, bus)
		}).
		Export("host_iobus_create_handle")

	// Export host_iobus_close_handle function
	hostBuilder.NewFunctionBuilder().
		WithFunc(func(ctxParam context.Context, m api.Module, uriPtr, uriLen uint32) uint32 {
			return hostIOBusCloseHandle(ctxParam, m, uriPtr, uriLen, ctx, bus)
		}).
		Export("host_iobus_close_handle")

	// Instantiate the host module
	_, err = hostBuilder.Instantiate(ctx)
	if err != nil {
		handleRuntime.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate host module: %w", err)
	}

	// Instantiate the WASM module
	module, err := handleRuntime.InstantiateModule(ctx, compiled, config)
	if err != nil {
		handleRuntime.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate WASM module: %w", err)
	}

	return &WASMHandle{
		module:   module,
		runtime:  handleRuntime,
		compiled: compiled,
		ctx:      ctx,
	}, nil
}

// ============================================================================
// Handle Interface Implementation
// ============================================================================

// ID returns the handle ID
func (h *WASMHandle) ID() string {
	return h.id
}

// Call invokes an exported function in the WASM module
func (h *WASMHandle) Call(ctx iobus.Context, args map[string]any) (any, error) {
	// Extract function name from args
	functionName, ok := args["function"].(string)
	if !ok {
		return nil, fmt.Errorf("function name required in args['function']")
	}

	// Get the exported function
	fn := h.module.ExportedFunction(functionName)
	if fn == nil {
		return nil, fmt.Errorf("function '%s' not found in WASM module", functionName)
	}

	// Handle different function signatures
	var results []uint64
	var err error

	switch functionName {
	case "driver_metadata":
		// No parameters
		results, err = fn.Call(ctx)

	case "driver_init":
		// Parameters: (ptr: u32, length: u32) -> u32
		// Get config from args
		config := args["config"]
		if config == nil {
			config = map[string]any{}
		}

		// Serialize config to JSON
		configJSON, jsonErr := json.Marshal(config)
		if jsonErr != nil {
			return nil, fmt.Errorf("failed to serialize config: %w", jsonErr)
		}

		// Write to WASM memory
		ptr, writeErr := h.writeToMemory(configJSON)
		if writeErr != nil {
			return nil, fmt.Errorf("failed to write config to memory: %w", writeErr)
		}

		// Call function with ptr and length
		results, err = fn.Call(ctx, uint64(ptr), uint64(len(configJSON)))

	case "driver_call":
		// Parameters: (ptr: u32, length: u32) -> *const u8
		// Get request from args
		request, ok := args["request"].(string)
		if !ok {
			return nil, fmt.Errorf("request required in args['request']")
		}

		// Write to WASM memory
		ptr, writeErr := h.writeToMemory([]byte(request))
		if writeErr != nil {
			return nil, fmt.Errorf("failed to write request to memory: %w", writeErr)
		}

		// Call function with ptr and length
		results, err = fn.Call(ctx, uint64(ptr), uint64(len(request)))

	case "wazeos_tool_invoke":
		// MCP tool invocation: (ptr: u32, length: u32) -> *const u8
		// Get args from args
		argsData, ok := args["args"].(string)
		if !ok {
			return nil, fmt.Errorf("args required in args['args']")
		}

		// Write to WASM memory
		ptr, writeErr := h.writeToMemory([]byte(argsData))
		if writeErr != nil {
			return nil, fmt.Errorf("failed to write args to memory: %w", writeErr)
		}

		// Call function with ptr and length
		results, err = fn.Call(ctx, uint64(ptr), uint64(len(argsData)))

	default:
		// Unknown function - try calling with no parameters
		results, err = fn.Call(ctx)
	}

	if err != nil {
		return nil, fmt.Errorf("WASM function call failed: %w", err)
	}

	// If no results, return empty response
	if len(results) == 0 {
		return map[string]any{
			"status": "success",
			"result": nil,
		}, nil
	}

	// Get the first result
	result := results[0]

	// Try to read from memory with panic recovery
	memResult := h.tryReadFromMemory(result)
	if memResult != nil {
		return memResult, nil
	}

	// Couldn't read from memory - return raw integer result
	return map[string]any{
		"status": "success",
		"result": result,
	}, nil
}

// Close releases the WASM module resources
func (h *WASMHandle) Close() error {
	if h.module != nil {
		h.module.Close(context.Background())
	}
	if h.runtime != nil {
		return h.runtime.Close(context.Background())
	}
	return nil
}

// writeToMemory allocates memory in WASM and writes data to it
func (h *WASMHandle) writeToMemory(data []byte) (uint32, error) {
	memory := h.module.Memory()
	if memory == nil {
		return 0, fmt.Errorf("WASM module has no memory")
	}

	// Allocate memory at the end of current memory
	// This is a simple approach - in production you'd want a proper allocator
	memSize := memory.Size()
	dataLen := uint32(len(data))

	// Ensure we have enough space
	if memSize < dataLen {
		// Grow memory if needed (each page is 64KB)
		pagesNeeded := (dataLen / 65536) + 1
		memory.Grow(pagesNeeded)
		memSize = memory.Size()
	}

	// Write at offset (leave some space for stack)
	offset := memSize - dataLen - 1024
	if offset < 0 {
		offset = 1024 // Start after first KB
	}

	ok := memory.Write(uint32(offset), data)
	if !ok {
		return 0, fmt.Errorf("failed to write to WASM memory")
	}

	return uint32(offset), nil
}

// tryReadFromMemory attempts to read a result from WASM memory
// Returns nil if memory is not available or read fails
func (h *WASMHandle) tryReadFromMemory(result uint64) any {
	// Recover from any panics when accessing memory
	defer func() {
		recover()
	}()

	// Check if module has memory
	memory := h.module.Memory()
	if memory == nil {
		return nil
	}

	// Check memory size (may panic if memory not initialized)
	memorySize := memory.Size()
	if memorySize == 0 {
		return nil
	}

	// Module has memory - treat result as a pointer
	ptr := uint32(result)
	if ptr == 0 {
		// Null pointer
		return map[string]any{
			"status": "success",
			"result": nil,
		}
	}

	// Validate pointer is within memory bounds
	if ptr >= memorySize {
		return nil
	}

	// Read the JSON string from memory (read up to 64KB or remaining memory)
	maxSize := uint32(64 * 1024)
	if ptr+maxSize > memorySize {
		maxSize = memorySize - ptr
	}

	data, ok := memory.Read(ptr, maxSize)
	if !ok {
		return nil
	}

	// Find the null terminator or JSON end
	length := 0
	for i, b := range data {
		if b == 0 {
			length = i
			break
		}
	}
	if length == 0 {
		length = len(data)
	}

	resultJSON := data[:length]

	// Try to parse as JSON and return structured data
	var parsedResult any
	if err := json.Unmarshal(resultJSON, &parsedResult); err == nil {
		return parsedResult
	}

	// If not JSON, return as string
	return string(resultJSON)
}

// ============================================================================
// Host Function Implementations
// ============================================================================

// hostIOBusCall allows WASM to call other drivers via the IO Bus
func hostIOBusCall(ctxParam context.Context, m api.Module, ptr, length uint32, wazeosCtx iobus.Context, bus *iobus.IOBus) uint64 {
	// Read request JSON from WASM memory
	reqBytes, ok := m.Memory().Read(ptr, length)
	if !ok {
		return 0
	}

	var req iobus.Request
	if err := json.Unmarshal(reqBytes, &req); err != nil {
		return 0
	}

	// Set operation if not specified
	if req.Operation == "" {
		req.Operation = iobus.OpCall
	}

	// Call the IO Bus
	resp, err := bus.Call(wazeosCtx, req)
	if err != nil {
		resp = iobus.Response{
			StatusCode: 500,
			Error:      err.Error(),
			Headers:    make(map[string]string),
			Body:       []byte{},
		}
	}

	// Ensure headers is not nil (prevents JSON serialization issues)
	if resp.Headers == nil {
		resp.Headers = make(map[string]string)
	}
	if resp.Body == nil {
		resp.Body = []byte{}
	}

	// Serialize response
	respBytes, err := json.Marshal(resp)
	if err != nil {
		// Return a minimal error response
		respBytes = []byte(`{"status_code":500,"error":"failed to serialize response","headers":{},"body":""}`)
	}

	// Write response to WASM memory
	// Allocate at a high memory offset to avoid conflicts with stack/heap
	respSize := uint32(len(respBytes))
	memSize := m.Memory().Size()

	// Start from a safe high offset (leave space for stack and Rust heap)
	// Use last 256KB of memory for host-to-wasm communication
	if memSize < 256*1024 {
		// Need to grow memory
		pagesNeeded := uint32(((256 * 1024) / 65536) + 1)
		_, ok := m.Memory().Grow(pagesNeeded)
		if !ok {
			return 0
		}
		memSize = m.Memory().Size()
	}

	// Write response near end of memory
	respPtr := memSize - respSize - 4096  // Leave 4KB buffer at end

	if !m.Memory().Write(respPtr, respBytes) {
		return 0
	}

	// Return pointer (high 32 bits) and length (low 32 bits)
	return (uint64(respPtr) << 32) | uint64(respSize)
}

// hostIOBusCreateHandle allows WASM to create handles in other drivers
func hostIOBusCreateHandle(ctxParam context.Context, m api.Module, ptr, length uint32, wazeosCtx iobus.Context, bus *iobus.IOBus) uint64 {
	// Read request JSON from WASM memory
	reqBytes, ok := m.Memory().Read(ptr, length)
	if !ok {
		return 0
	}

	var req iobus.Request
	if err := json.Unmarshal(reqBytes, &req); err != nil {
		return 0
	}

	// Set operation
	req.Operation = iobus.OpCreateHandle

	// Call the IO Bus to create handle
	resp, err := bus.Call(wazeosCtx, req)
	if err != nil || resp.StatusCode != 200 {
		return 0
	}

	// The handle ID is in the response body
	handleID := string(resp.Body)

	// Write handle ID to memory
	handleIDBytes := []byte(handleID)
	idPtr := uint32(2 * 1024 * 1024) // 2MB offset
	idSize := uint32(len(handleIDBytes))

	if !m.Memory().Write(idPtr, handleIDBytes) {
		return 0
	}

	// Return pointer (high 32 bits) and length (low 32 bits)
	return (uint64(idPtr) << 32) | uint64(idSize)
}

// hostIOBusCloseHandle allows WASM to close handles
func hostIOBusCloseHandle(ctxParam context.Context, m api.Module, uriPtr, uriLen uint32, wazeosCtx iobus.Context, bus *iobus.IOBus) uint32 {
	// Read handle URI from memory
	uriBytes, ok := m.Memory().Read(uriPtr, uriLen)
	if !ok {
		return 1 // Error
	}

	uri := string(uriBytes)

	// Create close request
	req := iobus.Request{
		URI:       uri,
		Operation: iobus.OpCloseHandle,
	}

	// Call the IO Bus
	_, err := bus.Call(wazeosCtx, req)
	if err != nil {
		return 1 // Error
	}

	return 0 // Success
}
