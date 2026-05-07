package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/wazeos/wazeos/internal/types"
)

// WazeroRuntime provides WASM execution with wazero and host functions
type WazeroRuntime struct {
	runtime    wazero.Runtime
	resourceBus types.ResourceBus
	ipcQueue    map[uint32]*types.ResourceResult // Call ID -> Result
	nextCallID  uint32
	mu          sync.Mutex
}

// NewWazeroRuntime creates a new wazero-based WASM runtime
func NewWazeroRuntime(ctx context.Context, resourceBus types.ResourceBus) (*WazeroRuntime, error) {
	rt := wazero.NewRuntime(ctx)

	// Instantiate WASI
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		return nil, fmt.Errorf("failed to instantiate WASI: %w", err)
	}

	runtime := &WazeroRuntime{
		runtime:    rt,
		resourceBus: resourceBus,
		ipcQueue:   make(map[uint32]*types.ResourceResult),
		nextCallID: 1,
	}

	// Register custom host functions
	if err := runtime.registerHostFunctions(ctx); err != nil {
		return nil, fmt.Errorf("failed to register host functions: %w", err)
	}

	return runtime, nil
}

// registerHostFunctions registers WazeOS host functions for IPC
func (r *WazeroRuntime) registerHostFunctions(ctx context.Context) error {
	// Create host module "kernel" to match SDK expectations
	_, err := r.runtime.NewHostModuleBuilder("kernel").
		NewFunctionBuilder().
		WithFunc(r.hostResourceCall).
		Export("resource_call").
		Instantiate(ctx)

	return err
}

// hostResourceCall matches the SDK's expected signature:
// hostResourceCall(paramsPtr, paramsLen, resultBufPtr, resultBufCap uint32) uint32
func (r *WazeroRuntime) hostResourceCall(ctx context.Context, mod api.Module, paramsPtr, paramsLen, resultBufPtr, resultBufCap uint32) uint32 {
	// Read ResourceCall JSON from WASM memory
	data, ok := mod.Memory().Read(paramsPtr, paramsLen)
	if !ok {
		fmt.Printf("[host] Failed to read memory at %d (len %d)\n", paramsPtr, paramsLen)
		return 0
	}

	// Parse ResourceCall
	var call types.ResourceCall
	if err := json.Unmarshal(data, &call); err != nil {
		fmt.Printf("[host] Failed to parse ResourceCall: %v\n", err)
		return 0
	}

	// Execute call synchronously through resource bus (with authz layer)
	result, err := r.resourceBus.Call(ctx, &call)
	if err != nil {
		result = &types.ResourceResult{
			StatusCode: 500,
			Body:       []byte(fmt.Sprintf(`{"error":"resource call failed: %v"}`, err)),
		}
	}

	// Serialize result to JSON
	resultJSON, err := json.Marshal(result)
	if err != nil {
		fmt.Printf("[host] Failed to marshal result: %v\n", err)
		return 0
	}

	// Write result to output buffer in WASM memory
	actualLen := uint32(len(resultJSON))
	if actualLen > resultBufCap {
		actualLen = resultBufCap // Truncate if buffer too small
	}

	if !mod.Memory().Write(resultBufPtr, resultJSON[:actualLen]) {
		fmt.Printf("[host] Failed to write to WASM memory\n")
		return 0
	}

	return actualLen
}

// Execute runs a WASM binary
func (r *WazeroRuntime) Execute(ctx context.Context, wasmBinary []byte, args []string) error {
	// Compile WASM module
	compiled, err := r.runtime.CompileModule(ctx, wasmBinary)
	if err != nil {
		return fmt.Errorf("failed to compile WASM: %w", err)
	}

	// Instantiate with args
	config := wazero.NewModuleConfig().WithArgs(args...)
	mod, err := r.runtime.InstantiateModule(ctx, compiled, config)
	if err != nil {
		return fmt.Errorf("failed to instantiate module: %w", err)
	}
	defer mod.Close(ctx)

	return nil
}

// Close shuts down the runtime
func (r *WazeroRuntime) Close(ctx context.Context) error {
	return r.runtime.Close(ctx)
}
