package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/sys"

	"github.com/wazeos/wazeos/internal/types"
)

// RuntimeExec implements types.RuntimeExec using wazero.
type RuntimeExec struct {
	mu            sync.RWMutex
	runtime       wazero.Runtime
	compiledApps  map[string]wazero.CompiledModule // appID -> compiled module
	driverMeta    map[string]*types.AppMetadata    // appID -> driver metadata (for permission checking)
	hostFunctions map[string]types.HostFunction    // "namespace.name" -> function
	resourceBus   types.ResourceBus
	timeout       time.Duration
}

// NewRuntimeExec creates a new wazero-based runtime executor.
func NewRuntimeExec(timeout time.Duration) *RuntimeExec {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx := context.Background()
	r := wazero.NewRuntime(ctx)

	// Instantiate WASI once for the runtime
	_, err := wasi_snapshot_preview1.Instantiate(ctx, r)
	if err != nil {
		// If WASI instantiation fails, still return the runtime
		// but it won't be able to execute WASI-dependent modules
	}

	return &RuntimeExec{
		runtime:       r,
		compiledApps:  make(map[string]wazero.CompiledModule),
		driverMeta:    make(map[string]*types.AppMetadata),
		hostFunctions: make(map[string]types.HostFunction),
		timeout:       timeout,
	}
}

// Name returns the driver name.
func (r *RuntimeExec) Name() string {
	return "wazeos/exec"
}

// SetResourceBus provides access to the resource driver bus.
func (r *RuntimeExec) SetResourceBus(bus types.ResourceBus) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resourceBus = bus
}

// CompileWASM compiles a WASM binary and returns the compiled module.
// Useful for creating WASM driver wrappers without storing in the app cache.
func (r *RuntimeExec) CompileWASM(ctx context.Context, wasmBytes []byte) (wazero.CompiledModule, error) {
	if wasmBytes == nil || len(wasmBytes) == 0 {
		return nil, fmt.Errorf("wasm binary is required: %w", types.ErrInvalidRequest)
	}

	compiled, err := r.runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to compile wasm module: %w", err)
	}

	return compiled, nil
}

// GetMetadata extracts metadata from a WASM binary by calling its wazeos_metadata() function.
// This allows apps to be self-describing without external metadata files.
func (r *RuntimeExec) GetMetadata(ctx context.Context, wasmBytes []byte) (*types.AppMetadata, error) {
	if wasmBytes == nil || len(wasmBytes) == 0 {
		return nil, fmt.Errorf("wasm binary is required: %w", types.ErrInvalidRequest)
	}

	// Compile the module
	compiled, err := r.runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to compile wasm module: %w", err)
	}
	defer compiled.Close(ctx)

	// Instantiate module with minimal config (no args, no stdio redirection)
	config := wazero.NewModuleConfig().WithName("metadata_loader")
	module, err := r.runtime.InstantiateModule(ctx, compiled, config)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate module: %w", err)
	}
	defer module.Close(ctx)

	// Call the wazeos_metadata() exported function
	metadataFn := module.ExportedFunction("wazeos_metadata")
	if metadataFn == nil {
		return nil, fmt.Errorf("wazeos_metadata function not exported by WASM module")
	}

	// Call the function (no arguments, returns pointer to JSON string)
	results, err := metadataFn.Call(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to call wazeos_metadata: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("wazeos_metadata returned no results")
	}

	// Read JSON string from WASM memory
	// The function returns a pointer to a null-terminated string
	ptr := uint32(results[0])
	if ptr == 0 {
		return nil, fmt.Errorf("wazeos_metadata returned null pointer")
	}

	// Read up to 100KB (generous limit for metadata)
	memory := module.Memory()
	maxSize := uint32(100 * 1024)
	jsonBytes, ok := memory.Read(ptr, maxSize)
	if !ok {
		return nil, fmt.Errorf("failed to read metadata from WASM memory at offset %d", ptr)
	}

	// Find null terminator (C-style string)
	var jsonData []byte
	for i, b := range jsonBytes {
		if b == 0 {
			jsonData = jsonBytes[:i]
			break
		}
	}
	if jsonData == nil {
		// No null terminator found, use all data (up to maxSize)
		jsonData = jsonBytes
	}

	// Parse JSON into AppMetadata
	var metadata types.AppMetadata
	if err := json.Unmarshal(jsonData, &metadata); err != nil {
		return nil, fmt.Errorf("invalid metadata JSON: %w", err)
	}

	// Validate required fields
	if metadata.Name == "" {
		return nil, fmt.Errorf("metadata.name is required")
	}
	if metadata.Version == "" {
		return nil, fmt.Errorf("metadata.version is required")
	}
	if metadata.Author == "" {
		return nil, fmt.Errorf("metadata.author is required")
	}

	return &metadata, nil
}

// LoadApp compiles and prepares a wasm binary for execution.
func (r *RuntimeExec) LoadApp(ctx context.Context, appID string, wasmBytes []byte) error {
	if appID == "" {
		return fmt.Errorf("appID is required: %w", types.ErrInvalidRequest)
	}

	if wasmBytes == nil || len(wasmBytes) == 0 {
		return fmt.Errorf("wasm binary is required: %w", types.ErrInvalidRequest)
	}

	// Compile the module
	compiled, err := r.runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		return fmt.Errorf("failed to compile wasm module: %w", err)
	}

	// Store in cache
	r.mu.Lock()
	defer r.mu.Unlock()

	// Close old module if exists
	if old, exists := r.compiledApps[appID]; exists {
		old.Close(ctx)
	}

	r.compiledApps[appID] = compiled
	return nil
}

// LoadDriver compiles and prepares a driver wasm binary with metadata.
// Metadata is stored for permission checking during execution.
func (r *RuntimeExec) LoadDriver(ctx context.Context, appID string, wasmBytes []byte, metadata *types.AppMetadata) error {
	// First load as regular app
	if err := r.LoadApp(ctx, appID, wasmBytes); err != nil {
		return err
	}

	// Store driver metadata for permission checking
	r.mu.Lock()
	defer r.mu.Unlock()
	r.driverMeta[appID] = metadata

	return nil
}

// RegisterHostFunction registers a host function for WASM drivers to call.
func (r *RuntimeExec) RegisterHostFunction(namespace, name string, fn types.HostFunction) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("namespace and name are required: %w", types.ErrInvalidRequest)
	}

	if fn == nil {
		return fmt.Errorf("function is required: %w", types.ErrInvalidRequest)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("%s.%s", namespace, name)
	r.hostFunctions[key] = fn

	// Rebuild the host module with the new function
	return r.rebuildHostModule(namespace)
}

// rebuildHostModule creates or updates the host module for a namespace.
// Must be called with r.mu held.
func (r *RuntimeExec) rebuildHostModule(namespace string) error {
	// Close existing module if present
	existingMod := r.runtime.Module(namespace)
	if existingMod != nil {
		existingMod.Close(context.Background())
	}

	// Collect all functions for this namespace
	builder := r.runtime.NewHostModuleBuilder(namespace)

	for key, fn := range r.hostFunctions {
		parts := strings.SplitN(key, ".", 2)
		if len(parts) != 2 || parts[0] != namespace {
			continue
		}

		funcName := parts[1]
		hostFn := fn // Capture for closure

		// Create wazero-compatible wrapper
		// The WASM signature is: func(paramsPtr, paramsLen, resultBufPtr, resultBufCap uint32) uint32
		// Returns: actual result length written to resultBuf
		wrapperFn := func(ctx context.Context, mod api.Module, paramsPtr, paramsLen, resultBufPtr, resultBufCap uint32) uint32 {
			// Read params from WASM memory
			paramsData, ok := mod.Memory().Read(paramsPtr, paramsLen)
			if !ok {
				return 0
			}

			// Call the host function
			resultData, err := hostFn(ctx, paramsData)
			if err != nil {
				// On error, return error as JSON
				resultData, _ = json.Marshal(map[string]string{
					"error": err.Error(),
				})
			}

			resultLen := uint32(len(resultData))

			// Check if result fits in provided buffer
			if resultLen > resultBufCap {
				// Write truncated error message
				errMsg := []byte(`{"error":"result too large"}`)
				mod.Memory().Write(resultBufPtr, errMsg)
				return uint32(len(errMsg))
			}

			// Write result to the provided buffer
			if !mod.Memory().Write(resultBufPtr, resultData) {
				return 0
			}

			return resultLen
		}

		builder.NewFunctionBuilder().
			WithFunc(wrapperFn).
			Export(funcName)
	}

	// Instantiate the host module
	_, err := builder.Instantiate(context.Background())
	return err
}

// UnloadApp removes a loaded app from the runtime.
func (r *RuntimeExec) UnloadApp(ctx context.Context, appID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	compiled, exists := r.compiledApps[appID]
	if !exists {
		return types.ErrNotFound
	}

	err := compiled.Close(ctx)
	delete(r.compiledApps, appID)
	delete(r.driverMeta, appID) // Also clean up driver metadata if present
	return err
}

// Execute runs a wasm app with the given arguments.
func (r *RuntimeExec) Execute(ctx context.Context, req *types.InvocationRequest) (*types.InvocationResult, error) {
	if req == nil {
		return nil, types.ErrInvalidRequest
	}

	if req.AppID == "" {
		return nil, fmt.Errorf("appID is required: %w", types.ErrInvalidRequest)
	}

	// Get compiled module from cache
	r.mu.RLock()
	compiled, exists := r.compiledApps[req.AppID]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("app not loaded: %s: %w", req.AppID, types.ErrNotFound)
	}

	// Create timeout context
	execCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// Capture stdout and stderr
	var stdout, stderr strings.Builder

	// Build module config with args
	args := append([]string{req.AppID}, req.Args...)
	moduleConfig := wazero.NewModuleConfig().
		WithStdout(&stdout).
		WithStderr(&stderr).
		WithArgs(args...).
		WithName(req.AppID)

	// Add environment variables from context metadata
	if req.Context.Metadata != nil {
		for key, value := range req.Context.Metadata {
			// Only pass environment variables (filter out headers, etc.)
			if strings.HasPrefix(key, "ENV_") {
				envKey := strings.TrimPrefix(key, "ENV_")
				moduleConfig = moduleConfig.WithEnv(envKey, value)
			}
		}
	}

	// Track execution metrics
	startTime := time.Now()

	// Instantiate and run the module
	module, err := r.runtime.InstantiateModule(execCtx, compiled, moduleConfig)
	if err != nil {
		// Check for timeout
		if execCtx.Err() == context.DeadlineExceeded {
			return &types.InvocationResult{
				RequestID:  req.Context.RequestID,
				Stdout:     []byte(stdout.String()),
				Stderr:     []byte(stderr.String()),
				ExitCode:   1,
				Duration:   time.Since(startTime),
				MemoryUsed: 0,
				Error:      types.ErrTimeout,
			}, types.ErrTimeout
		}

		// Check for exit code in error
		exitCode := 1
		if exitErr, ok := err.(*sys.ExitError); ok {
			exitCode = int(exitErr.ExitCode())
		}

		duration := time.Since(startTime)

		return &types.InvocationResult{
			RequestID:  req.Context.RequestID,
			Stdout:     []byte(stdout.String()),
			Stderr:     []byte(stderr.String()),
			ExitCode:   exitCode,
			Duration:   duration,
			MemoryUsed: 0,
			Error:      nil,
		}, nil
	}
	defer module.Close(execCtx)

	// Get memory usage (best effort - some minimal modules may not export memory)
	var memoryUsed int64
	func() {
		defer func() {
			if r := recover(); r != nil {
				// If getting memory size fails, just use 0
				memoryUsed = 0
			}
		}()
		if mem := module.Memory(); mem != nil {
			memoryUsed = int64(mem.Size())
		}
	}()

	duration := time.Since(startTime)

	return &types.InvocationResult{
		RequestID:  req.Context.RequestID,
		Stdout:     []byte(stdout.String()),
		Stderr:     []byte(stderr.String()),
		ExitCode:   0,
		Duration:   duration,
		MemoryUsed: memoryUsed,
		Error:      nil,
	}, nil
}

// Close releases resources held by the runtime.
func (r *RuntimeExec) Close(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Close all compiled modules
	for appID, compiled := range r.compiledApps {
		compiled.Close(ctx)
		delete(r.compiledApps, appID)
	}

	// Close runtime
	if r.runtime != nil {
		return r.runtime.Close(ctx)
	}
	return nil
}
