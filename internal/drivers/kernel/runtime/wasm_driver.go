package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tetratelabs/wazero"

	"github.com/wazeos/wazeos/internal/types"
)

// WasmResourceDriver wraps a WASM driver module and implements types.ResourceDriver.
// It communicates with the WASM driver via stdin/stdout JSON protocol.
type WasmResourceDriver struct {
	name           string
	patterns       []string
	runtime        *RuntimeExec
	compiledModule wazero.CompiledModule
	timeout        time.Duration
}

// NewWasmResourceDriver creates a new WASM resource driver wrapper.
func NewWasmResourceDriver(name string, patterns []string, runtime *RuntimeExec, compiled wazero.CompiledModule) *WasmResourceDriver {
	return &WasmResourceDriver{
		name:           name,
		patterns:       patterns,
		runtime:        runtime,
		compiledModule: compiled,
		timeout:        30 * time.Second,
	}
}

// Name returns the driver name in author/name format.
func (w *WasmResourceDriver) Name() string {
	return w.name
}

// Patterns returns URI patterns this driver handles.
func (w *WasmResourceDriver) Patterns() []string {
	return w.patterns
}

// HandleCall invokes the WASM driver with the given resource call.
func (w *WasmResourceDriver) HandleCall(ctx context.Context, call *types.ResourceCall) (*types.ResourceResult, error) {
	if call == nil {
		return nil, types.ErrInvalidRequest
	}

	// Serialize call to JSON
	callJSON, err := json.Marshal(call)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resource call: %w", err)
	}

	// Create timeout context
	execCtx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	// Prepare stdin/stdout buffers
	stdin := bytes.NewReader(callJSON)
	var stdout, stderr bytes.Buffer

	// Create module config with stdin/stdout and filesystem access
	moduleConfig := wazero.NewModuleConfig().
		WithStdin(stdin).
		WithStdout(&stdout).
		WithStderr(&stderr).
		WithName(w.name).
		WithFSConfig(wazero.NewFSConfig().
			WithDirMount("/", "/").           // Mount root filesystem
			WithDirMount("/tmp", "/tmp"))     // Mount /tmp explicitly

	// Instantiate and run the module
	module, err := w.runtime.runtime.InstantiateModule(execCtx, w.compiledModule, moduleConfig)
	if err != nil {
		// Check for timeout
		if execCtx.Err() == context.DeadlineExceeded {
			return &types.ResourceResult{
				StatusCode: 504,
				Headers:    make(map[string]string),
				Body:       []byte("driver timeout"),
				Error:      types.ErrTimeout.Error(),
			}, types.ErrTimeout
		}

		return &types.ResourceResult{
			StatusCode: 500,
			Headers:    make(map[string]string),
			Body:       []byte(fmt.Sprintf("driver error: %v", err)),
			Error:      err.Error(),
		}, err
	}
	defer module.Close(execCtx)

	// Parse result from stdout
	// Use intermediate struct since WASM drivers return Error as string, not error type
	var sdkResult struct {
		StatusCode int               `json:"statusCode"`
		Headers    map[string]string `json:"headers"`
		Body       []byte            `json:"body"`
		Error      string            `json:"error,omitempty"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &sdkResult); err != nil {
		stderrOutput := stderr.String()
		return &types.ResourceResult{
			StatusCode: 500,
			Headers:    make(map[string]string),
			Body:       []byte(fmt.Sprintf("failed to parse driver response: %v\nstderr: %s", err, stderrOutput)),
			Error:      fmt.Sprintf("invalid driver response: %v", err),
		}, err
	}

	// Convert SDK result to backend types
	result := &types.ResourceResult{
		StatusCode: sdkResult.StatusCode,
		Headers:    sdkResult.Headers,
		Body:       sdkResult.Body,
	}
	if sdkResult.Error != "" {
		result.Error = fmt.Sprintf("%s", sdkResult.Error)
	}

	return result, nil
}

// WasmAuthDriver wraps a WASM authentication driver module and implements types.SecurityAuthn.
type WasmAuthDriver struct {
	name           string
	runtime        *RuntimeExec
	compiledModule wazero.CompiledModule
	timeout        time.Duration
}

// NewWasmAuthDriver creates a new WASM authentication driver wrapper.
func NewWasmAuthDriver(name string, runtime *RuntimeExec, compiled wazero.CompiledModule) *WasmAuthDriver {
	return &WasmAuthDriver{
		name:           name,
		runtime:        runtime,
		compiledModule: compiled,
		timeout:        10 * time.Second,
	}
}

// Name returns the driver name in author/name format.
func (w *WasmAuthDriver) Name() string {
	return w.name
}

// Authenticate invokes the WASM auth driver with the given payload.
func (w *WasmAuthDriver) Authenticate(ctx context.Context, payload *types.AuthPayload) (string, error) {
	if payload == nil {
		return "", types.ErrInvalidRequest
	}

	// Serialize payload to JSON
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal auth payload: %w", err)
	}

	// Create timeout context
	execCtx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	// Prepare stdin/stdout buffers
	stdin := bytes.NewReader(payloadJSON)
	var stdout, stderr bytes.Buffer

	// Create module config with stdin/stdout
	moduleConfig := wazero.NewModuleConfig().
		WithStdin(stdin).
		WithStdout(&stdout).
		WithStderr(&stderr).
		WithName(w.name)

	// Instantiate and run the module
	module, err := w.runtime.runtime.InstantiateModule(execCtx, w.compiledModule, moduleConfig)
	if err != nil {
		// Check for timeout
		if execCtx.Err() == context.DeadlineExceeded {
			return "", types.ErrTimeout
		}
		return "", fmt.Errorf("driver error: %w", err)
	}
	defer module.Close(execCtx)

	// Parse result from stdout
	var result struct {
		Principal string `json:"principal,omitempty"`
		Error     string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		stderrOutput := stderr.String()
		return "", fmt.Errorf("invalid driver response: %v\nstderr: %s", err, stderrOutput)
	}

	// Check for auth errors
	if result.Error != "" {
		if strings.Contains(strings.ToLower(result.Error), "abstain") {
			return "", types.ErrAbstain
		}
		return "", fmt.Errorf("authentication failed: %s", result.Error)
	}

	if result.Principal == "" {
		return "", types.ErrAbstain
	}

	return result.Principal, nil
}

// LoadInstalledResourceDrivers loads and compiles all installed WASM resource drivers.
// Returns a slice of drivers ready to be registered with the resource bus.
func LoadInstalledResourceDrivers(ctx context.Context, pkgMgr types.PackageManager, runtime *RuntimeExec) ([]types.ResourceDriver, error) {
	if pkgMgr == nil {
		return nil, fmt.Errorf("package manager cannot be nil")
	}
	if runtime == nil {
		return nil, fmt.Errorf("runtime cannot be nil")
	}

	// List all installed packages
	packages, err := pkgMgr.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list packages: %w", err)
	}

	drivers := make([]types.ResourceDriver, 0)

	// Filter for resource drivers with URI patterns
	for _, metadata := range packages {
		if !metadata.IsDriver() {
			continue
		}

		if len(metadata.URIPatterns) == 0 {
			// Driver without patterns - not a resource driver (e.g., authn, audit)
			continue
		}

		// Read WASM binary
		wasmBytes, err := pkgMgr.GetWasmBinary(metadata.AppID())
		if err != nil {
			return nil, fmt.Errorf("failed to read WASM for driver %s: %w", metadata.AppID(), err)
		}

		// Compile WASM module
		compiled, err := runtime.CompileWASM(ctx, wasmBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to compile driver for patterns %v: %w", metadata.URIPatterns, err)
		}

		// Construct driver name in author/name format
		driverName := fmt.Sprintf("%s/%s", metadata.Author, metadata.Name)

		// Create WASM resource driver wrapper
		driver := NewWasmResourceDriver(
			driverName,
			metadata.URIPatterns,
			runtime,
			compiled,
		)

		drivers = append(drivers, driver)
	}

	return drivers, nil
}
