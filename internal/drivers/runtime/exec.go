package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/wazeos/wazeos/internal/types"
)

// ExecDriver handles fn:// calls - app execution requests
// This routes MCP tool calls and inter-app calls through kernel.iobus
type ExecDriver struct {
	packageMgr       types.PackageManager
	resourceBus      types.ResourceBus
	lifecycleManager types.LifecycleManager
	sharedRuntime    wazero.Runtime // Shared runtime for compiling and caching modules
	ctx              context.Context // Context for the shared runtime
}

// NewExecDriver creates a new execution driver
func NewExecDriver(packageMgr types.PackageManager, resourceBus types.ResourceBus, lifecycleManager types.LifecycleManager) *ExecDriver {
	if lifecycleManager == nil {
		lifecycleManager = NewNoopLifecycleManager()
	}

	// Create a long-lived context for the shared runtime
	ctx := context.Background()

	// Create shared runtime for compiling modules (kept alive for caching)
	sharedRuntime := wazero.NewRuntime(ctx)

	// Instantiate WASI once on the shared runtime
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, sharedRuntime); err != nil {
		fmt.Printf("Warning: failed to instantiate WASI: %v\n", err)
	}

	driver := &ExecDriver{
		packageMgr:       packageMgr,
		resourceBus:      resourceBus,
		lifecycleManager: lifecycleManager,
		sharedRuntime:    sharedRuntime,
		ctx:              ctx,
	}

	// Register host functions once on the shared runtime
	if err := driver.registerHostFunctions(ctx, sharedRuntime); err != nil {
		fmt.Printf("Warning: failed to register host functions: %v\n", err)
	}

	return driver
}

// Name returns the driver name
func (d *ExecDriver) Name() string {
	return "kernel.runtime.exec"
}

// Patterns returns URI patterns this driver handles
func (d *ExecDriver) Patterns() []string {
	return []string{"fn://**"}
}

// HandleCall processes fn:// execution requests
func (d *ExecDriver) HandleCall(ctx context.Context, call *types.ResourceCall) (*types.ResourceResult, error) {
	fmt.Printf("[EXEC] Processing execution request\n")
	fmt.Printf("[EXEC]   URI: %s\n", call.URI)
	fmt.Printf("[EXEC]   Permissions: %v\n", call.Permissions)

	// Parse fn://app-id where app-id can be:
	// - Full ID: author/name:version (e.g., "deniscoady/test:1.0.0")
	// - Short form: author/name (resolved to latest version)
	// - Just name: name (for backward compatibility, searches by name)
	appNameOrID := strings.TrimPrefix(call.URI, "fn://")
	// Remove any trailing path (e.g., /function)
	if idx := strings.Index(appNameOrID, "/"); idx > 0 && strings.Contains(appNameOrID[:idx], ":") {
		// Skip - this is a version separator, not a path
	} else if idx := strings.LastIndex(appNameOrID, "/"); idx > 0 && !strings.Contains(appNameOrID, ":") {
		// This might be a function path, extract the app part
		// But only if there's no version (no colon before the slash)
		appNameOrID = appNameOrID[:idx]
	}

	fmt.Printf("[EXEC]   App: %s\n", appNameOrID)

	// Find app by ID or name
	var appID string
	var appMetadata *types.AppMetadata
	if d.packageMgr != nil {
		// Try to resolve using package manager (handles author/name:version format)
		if strings.Contains(appNameOrID, "/") {
			// Full or partial ID - use Resolve
			resolvedID, err := d.packageMgr.Resolve(ctx, appNameOrID)
			if err == nil {
				appID = resolvedID
				appMetadata, err = d.packageMgr.Get(ctx, appID)
				if err != nil {
					return &types.ResourceResult{
						StatusCode: 404,
						Body:       []byte(fmt.Sprintf(`{"error":"app metadata not found: %s"}`, appID)),
					}, nil
				}
				fmt.Printf("[EXEC]   Resolved app: %s\n", appID)
			} else {
				return &types.ResourceResult{
					StatusCode: 404,
					Body:       []byte(fmt.Sprintf(`{"error":"app not found: %s"}`, appNameOrID)),
				}, nil
			}
		} else {
			// Just a name - search by name (backward compatibility)
			apps, err := d.packageMgr.List(ctx)
			if err != nil {
				return &types.ResourceResult{
					StatusCode: 500,
					Body:       []byte(fmt.Sprintf(`{"error":"failed to list apps: %v"}`, err)),
				}, nil
			}

			found := false
			for _, app := range apps {
				if app.Name == appNameOrID {
					// Use the canonical AppID from metadata
					appID = app.AppID()
					appMetadata = app
					found = true
					fmt.Printf("[EXEC]   Found app: %s (appID: %s)\n", app.Name, appID)
					break
				}
			}

			if !found {
				return &types.ResourceResult{
					StatusCode: 404,
					Body:       []byte(fmt.Sprintf(`{"error":"app not found: %s"}`, appNameOrID)),
				}, nil
			}
		}
	} else {
		return &types.ResourceResult{
			StatusCode: 500,
			Body:       []byte(`{"error":"package manager not available"}`),
		}, nil
	}

	// Parse arguments from body if present
	var args map[string]interface{}
	if len(call.Body) > 0 {
		if err := json.Unmarshal(call.Body, &args); err != nil {
			return &types.ResourceResult{
				StatusCode: 400,
				Body:       []byte(fmt.Sprintf(`{"error":"invalid arguments: %v"}`, err)),
			}, nil
		}
		fmt.Printf("[EXEC]   Arguments: %d provided\n", len(args))
	}

	// Get WASM binary from package manager
	wasmBinary, err := d.packageMgr.GetWasmBinary(appID)
	if err != nil {
		return &types.ResourceResult{
			StatusCode: 404,
			Body:       []byte(fmt.Sprintf(`{"error":"WASM binary not found: %v"}`, err)),
		}, nil
	}

	fmt.Printf("[EXEC]   WASM binary loaded: %d bytes\n", len(wasmBinary))

	// Execute WASM via wazero
	result, err := d.executeWASM(ctx, appMetadata.Name, wasmBinary, args)
	if err != nil {
		fmt.Printf("[EXEC]   ❌ Execution failed: %v\n", err)
		return &types.ResourceResult{
			StatusCode: 500,
			Body:       []byte(fmt.Sprintf(`{"error":"WASM execution failed: %v"}`, err)),
		}, nil
	}

	fmt.Printf("[EXEC] ✓ Execution completed successfully\n")

	return result, nil
}

// executeWASM executes a WASM binary with wazero
func (d *ExecDriver) executeWASM(ctx context.Context, appName string, wasmBinary []byte, args map[string]interface{}) (*types.ResourceResult, error) {
	// Try to get compiled module from cache
	appID := appName // Use appName as cache key (simplified)
	cachedModule, err := d.lifecycleManager.Get(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("lifecycle manager error: %w", err)
	}

	// Get or compile the WASM module (using shared runtime)
	var compiled wazero.CompiledModule
	if cachedModule != nil {
		// Use cached compiled module
		wazeromCompiled, ok := cachedModule.(*WazeroCompiledModule)
		if !ok {
			return nil, fmt.Errorf("invalid cached module type")
		}
		compiled = wazeromCompiled.CompiledModule()
		fmt.Printf("[EXEC]   Using cached compiled module for %s\n", appName)
	} else {
		// Compile fresh on shared runtime
		fmt.Printf("[EXEC]   Compiling WASM module for %s\n", appName)
		compiled, err = d.sharedRuntime.CompileModule(ctx, wasmBinary)
		if err != nil {
			return nil, fmt.Errorf("failed to compile WASM: %w", err)
		}

		// Store in cache
		wrappedModule := NewWazeroCompiledModule(compiled, appName)
		if err := d.lifecycleManager.Put(ctx, appID, wrappedModule); err != nil {
			fmt.Printf("[EXEC]   Warning: failed to cache module: %v\n", err)
			// Continue anyway, we have the compiled module
		}
	}

	// Instantiate WASI with stdout/stderr capture
	var stdout, stderr bytes.Buffer
	wasiConfig := wazero.NewModuleConfig().
		WithStdout(&stdout).
		WithStderr(&stderr).
		WithName(appName)

	// Convert args map to JSON and pass as stdin
	if len(args) > 0 {
		argsJSON, err := json.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal arguments: %w", err)
		}
		wasiConfig = wasiConfig.WithStdin(bytes.NewReader(argsJSON))
	}

	// Note: WASI and host functions should be instantiated once on the shared runtime
	// For now, we'll check if they need to be re-instantiated (wazero handles this gracefully)

	// Instantiate the compiled module on shared runtime with this execution's config
	mod, err := d.sharedRuntime.InstantiateModule(ctx, compiled, wasiConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate module: %w", err)
	}
	defer mod.Close(ctx)

	// Check for stderr output
	if stderr.Len() > 0 {
		fmt.Printf("[EXEC]   stderr: %s\n", stderr.String())
	}

	// Create InvocationResult with captured stdout/stderr
	invResult := types.InvocationResult{
		RequestID: "", // Will be set by caller
		Stdout:    stdout.Bytes(),
		Stderr:    stderr.Bytes(),
		ExitCode:  0, // Successful completion
		Duration:  0, // TODO: track execution time
		MemoryUsed: 0, // TODO: track memory usage
		Error:     nil,
	}

	// Marshal InvocationResult to JSON for transport
	resultJSON, err := json.Marshal(invResult)
	if err != nil {
		return &types.ResourceResult{
			StatusCode: 500,
			Body:       []byte(fmt.Sprintf(`{"error":"failed to marshal result: %v"}`, err)),
		}, nil
	}

	return &types.ResourceResult{
		StatusCode: 200,
		Body:       resultJSON,
	}, nil
}

// registerHostFunctions registers WazeOS host functions for resource calls
func (d *ExecDriver) registerHostFunctions(ctx context.Context, rt wazero.Runtime) error {
	// Create host module "kernel" to match SDK expectations
	_, err := rt.NewHostModuleBuilder("kernel").
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, mod api.Module, paramsPtr, paramsLen, resultBufPtr, resultBufCap uint32) uint32 {
			return d.hostResourceCall(ctx, mod, paramsPtr, paramsLen, resultBufPtr, resultBufCap)
		}).
		Export("resource_call").
		Instantiate(ctx)

	return err
}

// hostResourceCall is the host function that WASM apps call to make resource calls
func (d *ExecDriver) hostResourceCall(ctx context.Context, mod api.Module, paramsPtr, paramsLen, resultBufPtr, resultBufCap uint32) uint32 {
	// Read ResourceCall JSON from WASM memory
	data, ok := mod.Memory().Read(paramsPtr, paramsLen)
	if !ok {
		fmt.Printf("[EXEC]   Failed to read memory at %d (len %d)\n", paramsPtr, paramsLen)
		return 0
	}

	// Parse ResourceCall
	var call types.ResourceCall
	if err := json.Unmarshal(data, &call); err != nil {
		fmt.Printf("[EXEC]   Failed to parse ResourceCall: %v\n", err)
		return 0
	}

	fmt.Printf("[EXEC]   Host function call: %s (permissions: %v)\n", call.URI, call.Permissions)

	// Execute call through resource bus (with authz layer)
	result, err := d.resourceBus.Call(ctx, &call)
	if err != nil {
		result = &types.ResourceResult{
			StatusCode: 500,
			Body:       []byte(fmt.Sprintf(`{"error":"resource call failed: %v"}`, err)),
		}
	}

	// Serialize result to JSON
	resultJSON, err := json.Marshal(result)
	if err != nil {
		fmt.Printf("[EXEC]   Failed to marshal result: %v\n", err)
		return 0
	}

	// Write result to output buffer in WASM memory
	actualLen := uint32(len(resultJSON))
	if actualLen > resultBufCap {
		actualLen = resultBufCap // Truncate if buffer too small
	}

	if !mod.Memory().Write(resultBufPtr, resultJSON[:actualLen]) {
		fmt.Printf("[EXEC]   Failed to write to WASM memory\n")
		return 0
	}

	return actualLen
}
