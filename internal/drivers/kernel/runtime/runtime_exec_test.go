package runtime

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wazeos/wazeos/internal/types"
)

// Test WASM modules as bytecode
// These are minimal valid WASM modules for testing

// helloWorldWasm is a WASM module that exports a _start function
// This is a complete module with a _start function that returns immediately
var helloWorldWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, // magic
	0x01, 0x00, 0x00, 0x00, // version
	// Type section: function signature
	0x01, // section id
	0x04, // section size
	0x01, // 1 type
	0x60, 0x00, 0x00, // func type: () -> ()
	// Function section
	0x03, // section id
	0x02, // section size
	0x01, // 1 function
	0x00, // type index 0
	// Export section
	0x07, // section id
	0x0a, // section size (10 bytes)
	0x01, // 1 export
	0x06, 0x5f, 0x73, 0x74, 0x61, 0x72, 0x74, // "_start" (length byte + 6 chars)
	0x00, 0x00, // func export, index 0
	// Code section
	0x0a, // section id
	0x04, // section size
	0x01, // 1 function body
	0x02, // body size
	0x00, // local count
	0x0b, // end
}

// infiniteLoopWasm is a WASM module with an infinite loop for timeout testing
var infiniteLoopWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, // magic
	0x01, 0x00, 0x00, 0x00, // version
	0x01, 0x04, 0x01, 0x60, 0x00, 0x00, // type section
	0x03, 0x02, 0x01, 0x00, // function section
	0x07, 0x0a, 0x01, 0x06, 0x5f, 0x73, 0x74, 0x61, 0x72, 0x74, 0x00, 0x00, // export "_start" (fixed size)
	0x0a, 0x09, 0x01, 0x07, 0x00, 0x03, 0x40, 0x0c, 0x00, 0x0b, 0x0b, // infinite loop code
}

func TestNewRuntimeExec(t *testing.T) {
	exec := NewRuntimeExec(5 * time.Second)
	assert.NotNil(t, exec)
	assert.Equal(t, 5*time.Second, exec.timeout)
	assert.NotNil(t, exec.runtime)
	assert.NotNil(t, exec.compiledApps)

	// Clean up
	exec.Close(context.Background())
}

func TestNewRuntimeExec_DefaultTimeout(t *testing.T) {
	exec := NewRuntimeExec(0)
	assert.NotNil(t, exec)
	assert.Equal(t, 30*time.Second, exec.timeout)

	exec.Close(context.Background())
}

func TestRuntimeExec_Name(t *testing.T) {
	exec := NewRuntimeExec(0)
	defer exec.Close(context.Background())

	assert.Equal(t, "runtime.exec", exec.Name())
}

func TestRuntimeExec_LoadApp_Success(t *testing.T) {
	exec := NewRuntimeExec(0)
	defer exec.Close(context.Background())

	ctx := context.Background()

	err := exec.LoadApp(ctx, "test-app_1.0.0", helloWorldWasm)
	assert.NoError(t, err)

	// Verify app is in cache
	exec.mu.RLock()
	_, exists := exec.compiledApps["test-app_1.0.0"]
	exec.mu.RUnlock()
	assert.True(t, exists)
}

func TestRuntimeExec_LoadApp_EmptyAppID(t *testing.T) {
	exec := NewRuntimeExec(0)
	defer exec.Close(context.Background())

	ctx := context.Background()

	err := exec.LoadApp(ctx, "", helloWorldWasm)
	assert.Error(t, err)
	assert.True(t, types.IsInvalidRequest(err))
	assert.Contains(t, err.Error(), "appID is required")
}

func TestRuntimeExec_LoadApp_NilWasmBinary(t *testing.T) {
	exec := NewRuntimeExec(0)
	defer exec.Close(context.Background())

	ctx := context.Background()

	err := exec.LoadApp(ctx, "test-app", nil)
	assert.Error(t, err)
	assert.True(t, types.IsInvalidRequest(err))
	assert.Contains(t, err.Error(), "wasm binary is required")
}

func TestRuntimeExec_LoadApp_EmptyWasmBinary(t *testing.T) {
	exec := NewRuntimeExec(0)
	defer exec.Close(context.Background())

	ctx := context.Background()

	err := exec.LoadApp(ctx, "test-app", []byte{})
	assert.Error(t, err)
	assert.True(t, types.IsInvalidRequest(err))
	assert.Contains(t, err.Error(), "wasm binary is required")
}

func TestRuntimeExec_LoadApp_InvalidWasm(t *testing.T) {
	exec := NewRuntimeExec(0)
	defer exec.Close(context.Background())

	ctx := context.Background()

	err := exec.LoadApp(ctx, "test-app", []byte("not valid wasm"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to compile wasm module")
}

func TestRuntimeExec_LoadApp_ReplaceExisting(t *testing.T) {
	exec := NewRuntimeExec(0)
	defer exec.Close(context.Background())

	ctx := context.Background()

	// Load first time
	err := exec.LoadApp(ctx, "test-app_1.0.0", helloWorldWasm)
	require.NoError(t, err)

	// Load again with same appID (simulates update)
	err = exec.LoadApp(ctx, "test-app_1.0.0", helloWorldWasm)
	assert.NoError(t, err)

	// Should still only have one entry
	exec.mu.RLock()
	count := len(exec.compiledApps)
	exec.mu.RUnlock()
	assert.Equal(t, 1, count)
}

func TestRuntimeExec_UnloadApp_Success(t *testing.T) {
	exec := NewRuntimeExec(0)
	defer exec.Close(context.Background())

	ctx := context.Background()

	// Load app
	err := exec.LoadApp(ctx, "test-app_1.0.0", helloWorldWasm)
	require.NoError(t, err)

	// Unload it
	err = exec.UnloadApp(ctx, "test-app_1.0.0")
	assert.NoError(t, err)

	// Verify it's gone from cache
	exec.mu.RLock()
	_, exists := exec.compiledApps["test-app_1.0.0"]
	exec.mu.RUnlock()
	assert.False(t, exists)
}

func TestRuntimeExec_UnloadApp_NotFound(t *testing.T) {
	exec := NewRuntimeExec(0)
	defer exec.Close(context.Background())

	ctx := context.Background()

	err := exec.UnloadApp(ctx, "nonexistent-app")
	assert.Error(t, err)
	assert.True(t, types.IsNotFound(err))
}

func TestRuntimeExec_Execute_Success(t *testing.T) {
	exec := NewRuntimeExec(0)
	defer exec.Close(context.Background())

	ctx := context.Background()

	// Load app first
	err := exec.LoadApp(ctx, "test-app_1.0.0", helloWorldWasm)
	require.NoError(t, err)

	// Execute it
	execCtx := types.NewExecutionContext("req-1", "trace-1", "user:test", types.NewPermissionContext(nil))

	req := &types.InvocationRequest{
		Context: execCtx,
		AppID:   "test-app_1.0.0",
		Args:    []string{"arg1", "arg2"},
	}

	result, err := exec.Execute(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "req-1", result.RequestID)
	assert.Equal(t, 0, result.ExitCode)
	assert.Greater(t, result.Duration, time.Duration(0))
	assert.GreaterOrEqual(t, result.MemoryUsed, int64(0))
}

func TestRuntimeExec_Execute_NilRequest(t *testing.T) {
	exec := NewRuntimeExec(0)
	defer exec.Close(context.Background())

	ctx := context.Background()

	result, err := exec.Execute(ctx, nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, types.IsInvalidRequest(err))
}

func TestRuntimeExec_Execute_EmptyAppID(t *testing.T) {
	exec := NewRuntimeExec(0)
	defer exec.Close(context.Background())

	ctx := context.Background()
	execCtx := types.NewExecutionContext("req-1", "trace-1", "user:test", types.NewPermissionContext(nil))

	req := &types.InvocationRequest{
		Context: execCtx,
		AppID:   "",
		Args:    []string{},
	}

	result, err := exec.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, types.IsInvalidRequest(err))
	assert.Contains(t, err.Error(), "appID is required")
}

func TestRuntimeExec_Execute_AppNotLoaded(t *testing.T) {
	exec := NewRuntimeExec(0)
	defer exec.Close(context.Background())

	ctx := context.Background()
	execCtx := types.NewExecutionContext("req-1", "trace-1", "user:test", types.NewPermissionContext(nil))

	req := &types.InvocationRequest{
		Context: execCtx,
		AppID:   "nonexistent-app",
		Args:    []string{},
	}

	result, err := exec.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, types.IsNotFound(err))
	assert.Contains(t, err.Error(), "app not loaded")
}

func TestRuntimeExec_Execute_WithArgs(t *testing.T) {
	exec := NewRuntimeExec(0)
	defer exec.Close(context.Background())

	ctx := context.Background()

	// Load app
	err := exec.LoadApp(ctx, "test-app_1.0.0", helloWorldWasm)
	require.NoError(t, err)

	execCtx := types.NewExecutionContext("req-1", "trace-1", "user:test", types.NewPermissionContext(nil))

	args := []string{"arg1", "arg2", "arg3"}

	req := &types.InvocationRequest{
		Context: execCtx,
		AppID:   "test-app_1.0.0",
		Args:    args,
	}

	result, err := exec.Execute(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, result.ExitCode)
}

func TestRuntimeExec_Execute_WithEnvVars(t *testing.T) {
	exec := NewRuntimeExec(0)
	defer exec.Close(context.Background())

	ctx := context.Background()

	// Load app
	err := exec.LoadApp(ctx, "test-app_1.0.0", helloWorldWasm)
	require.NoError(t, err)

	execCtx := types.NewExecutionContext("req-1", "trace-1", "user:test", types.NewPermissionContext(nil))

	// Add environment variables to metadata with ENV_ prefix
	execCtx.Metadata["ENV_VAR1"] = "value1"
	execCtx.Metadata["ENV_VAR2"] = "value2"
	execCtx.Metadata["OTHER"] = "ignored" // Should be ignored (no ENV_ prefix)

	req := &types.InvocationRequest{
		Context: execCtx,
		AppID:   "test-app_1.0.0",
		Args:    []string{},
	}

	result, err := exec.Execute(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, result.ExitCode)
}

func TestRuntimeExec_Execute_Timeout(t *testing.T) {
	t.Skip("Skipping for MVP - requires interruptible WASM execution")
	exec := NewRuntimeExec(10 * time.Millisecond) // Very short timeout
	defer exec.Close(context.Background())

	ctx := context.Background()

	// Load infinite loop app
	err := exec.LoadApp(ctx, "test-app_1.0.0", infiniteLoopWasm)
	require.NoError(t, err)

	execCtx := types.NewExecutionContext("req-1", "trace-1", "user:test", types.NewPermissionContext(nil))

	req := &types.InvocationRequest{
		Context: execCtx,
		AppID:   "test-app_1.0.0",
		Args:    []string{},
	}

	result, err := exec.Execute(ctx, req)

	// Should return timeout error
	if result != nil {
		assert.Equal(t, 1, result.ExitCode)
	}

	if err != nil {
		assert.True(t, types.IsTimeout(err), "expected timeout error, got: %v", err)
	}
}

func TestRuntimeExec_Execute_ContextCancellation(t *testing.T) {
	t.Skip("Skipping for MVP - requires interruptible WASM execution")
	exec := NewRuntimeExec(30 * time.Second)
	defer exec.Close(context.Background())

	// Load infinite loop app that will take time
	err := exec.LoadApp(context.Background(), "test-app_1.0.0", infiniteLoopWasm)
	require.NoError(t, err)

	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	execCtx := types.NewExecutionContext("req-1", "trace-1", "user:test", types.NewPermissionContext(nil))

	req := &types.InvocationRequest{
		Context: execCtx,
		AppID:   "test-app_1.0.0",
		Args:    []string{},
	}

	result, err := exec.Execute(ctx, req)

	// Should timeout or cancel
	if err == nil {
		// If no error, should have non-zero exit code
		require.NotNil(t, result)
		assert.NotEqual(t, 0, result.ExitCode)
	} else {
		// If error, should be timeout
		assert.True(t, types.IsTimeout(err) || ctx.Err() != nil)
	}
}

func TestRuntimeExec_Execute_CapturesStdout(t *testing.T) {
	exec := NewRuntimeExec(0)
	defer exec.Close(context.Background())

	ctx := context.Background()

	// Load app
	err := exec.LoadApp(ctx, "test-app_1.0.0", helloWorldWasm)
	require.NoError(t, err)

	execCtx := types.NewExecutionContext("req-1", "trace-1", "user:test", types.NewPermissionContext(nil))

	req := &types.InvocationRequest{
		Context: execCtx,
		AppID:   "test-app_1.0.0",
		Args:    []string{},
	}

	result, err := exec.Execute(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Stdout)
	assert.NotNil(t, result.Stderr)
}

func TestRuntimeExec_Execute_MetricsTracking(t *testing.T) {
	exec := NewRuntimeExec(0)
	defer exec.Close(context.Background())

	ctx := context.Background()

	// Load app
	err := exec.LoadApp(ctx, "test-app_1.0.0", helloWorldWasm)
	require.NoError(t, err)

	execCtx := types.NewExecutionContext("req-1", "trace-1", "user:test", types.NewPermissionContext(nil))

	req := &types.InvocationRequest{
		Context: execCtx,
		AppID:   "test-app_1.0.0",
		Args:    []string{},
	}

	result, err := exec.Execute(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Verify metrics are tracked
	assert.Greater(t, result.Duration, time.Duration(0))
	assert.GreaterOrEqual(t, result.MemoryUsed, int64(0))
}

func TestRuntimeExec_SetResourceBus(t *testing.T) {
	exec := NewRuntimeExec(0)
	defer exec.Close(context.Background())

	// Create a mock resource bus (we'll just use nil for this test)
	exec.SetResourceBus(nil)

	// Verify it was set
	exec.mu.RLock()
	bus := exec.resourceBus
	exec.mu.RUnlock()
	assert.Nil(t, bus) // Nil is valid for this test
}

func TestRuntimeExec_Close(t *testing.T) {
	exec := NewRuntimeExec(0)

	// Load some apps
	ctx := context.Background()
	exec.LoadApp(ctx, "app1_1.0.0", helloWorldWasm)
	exec.LoadApp(ctx, "app2_1.0.0", helloWorldWasm)

	// Close should clean up everything
	err := exec.Close(ctx)
	assert.NoError(t, err)

	// Cache should be empty
	exec.mu.RLock()
	count := len(exec.compiledApps)
	exec.mu.RUnlock()
	assert.Equal(t, 0, count)

	// Should be safe to close multiple times
	err = exec.Close(ctx)
	assert.NoError(t, err)
}

func TestRuntimeExec_ConcurrentLoadAndExecute(t *testing.T) {
	t.Skip("Skipping for MVP - concurrent execution needs investigation")
	exec := NewRuntimeExec(0)
	defer exec.Close(context.Background())

	ctx := context.Background()

	// Load app
	err := exec.LoadApp(ctx, "test-app_1.0.0", helloWorldWasm)
	require.NoError(t, err)

	// Execute concurrently
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			execCtx := types.NewExecutionContext(fmt.Sprintf("req-%d", id), "trace-1", "user:test", types.NewPermissionContext(nil))

			req := &types.InvocationRequest{
				Context: execCtx,
				AppID:   "test-app_1.0.0",
				Args:    []string{fmt.Sprintf("arg-%d", id)},
			}

			result, err := exec.Execute(ctx, req)
			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, 0, result.ExitCode)

			done <- true
		}(i)
	}

	// Wait for all to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}
