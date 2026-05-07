package resource

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wazeos/wazeos/internal/types"
)

// Mock implementations for testing

type mockInvoker struct {
	invokeFn func(ctx context.Context, req *types.InvocationRequest) (*types.InvocationResult, error)
}

func (m *mockInvoker) Invoke(ctx context.Context, req *types.InvocationRequest) (*types.InvocationResult, error) {
	if m.invokeFn != nil {
		return m.invokeFn(ctx, req)
	}
	return &types.InvocationResult{
		RequestID:  req.Context.RequestID,
		Stdout:     []byte("mock output"),
		Stderr:     []byte(""),
		ExitCode:   0,
		Duration:   100 * time.Millisecond,
		MemoryUsed: 1024,
	}, nil
}

type mockPackageManager struct {
	resolveFn func(ctx context.Context, appName string) (string, error)
}

func (m *mockPackageManager) Name() string {
	return "pkg.install"
}

func (m *mockPackageManager) Install(ctx context.Context, zipData []byte) (*types.AppMetadata, error) {
	return nil, nil
}

func (m *mockPackageManager) Uninstall(ctx context.Context, appID string) error {
	return nil
}

func (m *mockPackageManager) List(ctx context.Context) ([]*types.AppMetadata, error) {
	return nil, nil
}

func (m *mockPackageManager) Get(ctx context.Context, appID string) (*types.AppMetadata, error) {
	return nil, types.ErrNotFound
}

func (m *mockPackageManager) Resolve(ctx context.Context, appName string) (string, error) {
	if m.resolveFn != nil {
		return m.resolveFn(ctx, appName)
	}
	return "author/" + appName + "_1.0.0", nil
}

func (m *mockPackageManager) GetWasmBinary(appID string) ([]byte, error) {
	return []byte("mock wasm binary"), nil
}

// Tests

func TestNewFnDriver(t *testing.T) {
	pkg := &mockPackageManager{}
	driver := NewFnDriver(pkg, 5)
	assert.NotNil(t, driver)
	assert.Equal(t, 5, driver.maxDepth)
}

func TestNewFnDriver_DefaultMaxDepth(t *testing.T) {
	pkg := &mockPackageManager{}
	driver := NewFnDriver(pkg, 0)
	assert.NotNil(t, driver)
	assert.Equal(t, 10, driver.maxDepth)
}

func TestFnDriver_Name(t *testing.T) {
	driver := NewFnDriver(&mockPackageManager{}, 0)
	assert.Equal(t, "wazeos/fn", driver.Name())
}

func TestFnDriver_Patterns(t *testing.T) {
	driver := NewFnDriver(&mockPackageManager{}, 0)
	patterns := driver.Patterns()
	assert.Equal(t, []string{"fn://*/*"}, patterns)
}

func TestFnDriver_SetInvoker(t *testing.T) {
	driver := NewFnDriver(&mockPackageManager{}, 0)
	invoker := &mockInvoker{}

	assert.Nil(t, driver.GetInvoker())

	driver.SetInvoker(invoker)
	assert.NotNil(t, driver.GetInvoker())
}

func TestFnDriver_HandleCall_Success(t *testing.T) {
	pkg := &mockPackageManager{}
	invoker := &mockInvoker{}

	driver := NewFnDriver(pkg, 0)
	driver.SetInvoker(invoker)

	ctx := context.Background()
	execCtx := types.NewExecutionContext("req-1", "trace-1", "user:test", types.NewPermissionContext(nil))

	call := &types.ResourceCall{
		Context:    execCtx,
		URI:        "fn://my-app/arg1/arg2",
		Headers:     make(map[string]string),
		Body:        nil,
		Permissions: []string{"execute"},
	}

	result, err := driver.HandleCall(ctx, call)
	assert.NoError(t, err)
	assert.Equal(t, 200, result.StatusCode)
	assert.Equal(t, "mock output", string(result.Body))
}

func TestFnDriver_HandleCall_WithStderr(t *testing.T) {
	pkg := &mockPackageManager{}
	invoker := &mockInvoker{
		invokeFn: func(ctx context.Context, req *types.InvocationRequest) (*types.InvocationResult, error) {
			return &types.InvocationResult{
				RequestID:  req.Context.RequestID,
				Stdout:     []byte("output"),
				Stderr:     []byte("error message"),
				ExitCode:   0,
				Duration:   100 * time.Millisecond,
				MemoryUsed: 1024,
			}, nil
		},
	}

	driver := NewFnDriver(pkg, 0)
	driver.SetInvoker(invoker)

	ctx := context.Background()
	execCtx := types.NewExecutionContext("req-1", "trace-1", "user:test", types.NewPermissionContext(nil))

	call := &types.ResourceCall{
		Context:    execCtx,
		URI:        "fn://my-app",
		Headers:     make(map[string]string),
		Body:        nil,
		Permissions: []string{"execute"},
	}

	result, err := driver.HandleCall(ctx, call)
	assert.NoError(t, err)
	assert.Equal(t, 200, result.StatusCode)
	assert.Contains(t, string(result.Body), "output")
	assert.Contains(t, string(result.Body), "error message")
	assert.Contains(t, string(result.Body), "[stderr]")
}

func TestFnDriver_HandleCall_NonZeroExitCode(t *testing.T) {
	pkg := &mockPackageManager{}
	invoker := &mockInvoker{
		invokeFn: func(ctx context.Context, req *types.InvocationRequest) (*types.InvocationResult, error) {
			return &types.InvocationResult{
				RequestID:  req.Context.RequestID,
				Stdout:     []byte("error occurred"),
				Stderr:     []byte(""),
				ExitCode:   1,
				Duration:   100 * time.Millisecond,
				MemoryUsed: 1024,
			}, nil
		},
	}

	driver := NewFnDriver(pkg, 0)
	driver.SetInvoker(invoker)

	ctx := context.Background()
	execCtx := types.NewExecutionContext("req-1", "trace-1", "user:test", types.NewPermissionContext(nil))

	call := &types.ResourceCall{
		Context:    execCtx,
		URI:        "fn://my-app",
		Headers:     make(map[string]string),
		Body:        nil,
		Permissions: []string{"execute"},
	}

	result, err := driver.HandleCall(ctx, call)
	assert.NoError(t, err)
	assert.Equal(t, 500, result.StatusCode)
	assert.Equal(t, "1", result.Headers["X-Exit-Code"])
}

func TestFnDriver_HandleCall_AppNotFound(t *testing.T) {
	pkg := &mockPackageManager{
		resolveFn: func(ctx context.Context, appName string) (string, error) {
			return "", types.ErrNotFound
		},
	}

	driver := NewFnDriver(pkg, 0)
	driver.SetInvoker(&mockInvoker{})

	ctx := context.Background()
	execCtx := types.NewExecutionContext("req-1", "trace-1", "user:test", types.NewPermissionContext(nil))

	call := &types.ResourceCall{
		Context:    execCtx,
		URI:        "fn://nonexistent-app",
		Headers:     make(map[string]string),
		Body:        nil,
		Permissions: []string{"execute"},
	}

	result, err := driver.HandleCall(ctx, call)
	assert.Error(t, err)
	assert.True(t, types.IsNotFound(err))
	assert.Equal(t, 404, result.StatusCode)
}

func TestFnDriver_HandleCall_MaxDepthExceeded(t *testing.T) {
	pkg := &mockPackageManager{}
	invoker := &mockInvoker{}

	driver := NewFnDriver(pkg, 2) // Max depth of 2
	driver.SetInvoker(invoker)

	ctx := context.Background()

	// Create deeply nested request ID to exceed max depth
	execCtx := types.NewExecutionContext("req-1.app1.app2.app3", "trace-1", "user:test", types.NewPermissionContext(nil))

	call := &types.ResourceCall{
		Context:    execCtx,
		URI:        "fn://my-app",
		Headers:     make(map[string]string),
		Body:        nil,
		Permissions: []string{"execute"},
	}

	result, err := driver.HandleCall(ctx, call)
	assert.Error(t, err)
	assert.True(t, types.IsMaxDepthExceeded(err))
	assert.Equal(t, 508, result.StatusCode)
	assert.Contains(t, string(result.Body), "maximum call depth")
}

func TestFnDriver_HandleCall_NilCall(t *testing.T) {
	driver := NewFnDriver(&mockPackageManager{}, 0)
	driver.SetInvoker(&mockInvoker{})

	ctx := context.Background()

	result, err := driver.HandleCall(ctx, nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, types.IsInvalidRequest(err))
}

func TestFnDriver_HandleCall_NoInvoker(t *testing.T) {
	driver := NewFnDriver(&mockPackageManager{}, 0)
	// Don't set invoker

	ctx := context.Background()
	execCtx := types.NewExecutionContext("req-1", "trace-1", "user:test", types.NewPermissionContext(nil))

	call := &types.ResourceCall{
		Context:    execCtx,
		URI:        "fn://my-app",
		Headers:     make(map[string]string),
		Body:        nil,
		Permissions: []string{"execute"},
	}

	result, err := driver.HandleCall(ctx, call)
	assert.Error(t, err)
	assert.True(t, types.IsInternal(err))
	assert.Equal(t, 500, result.StatusCode)
	assert.Contains(t, string(result.Body), "invoker not set")
}

func TestFnDriver_HandleCall_InvalidURI(t *testing.T) {
	driver := NewFnDriver(&mockPackageManager{}, 0)
	driver.SetInvoker(&mockInvoker{})

	ctx := context.Background()
	execCtx := types.NewExecutionContext("req-1", "trace-1", "user:test", types.NewPermissionContext(nil))

	call := &types.ResourceCall{
		Context:    execCtx,
		URI:        ":::invalid:::URI:::",
		Headers:     make(map[string]string),
		Body:        nil,
		Permissions: []string{"execute"},
	}

	result, err := driver.HandleCall(ctx, call)
	assert.Error(t, err)
	assert.True(t, types.IsInvalidRequest(err))
	assert.Equal(t, 400, result.StatusCode)
}

func TestFnDriver_HandleCall_NoAppName(t *testing.T) {
	driver := NewFnDriver(&mockPackageManager{}, 0)
	driver.SetInvoker(&mockInvoker{})

	ctx := context.Background()
	execCtx := types.NewExecutionContext("req-1", "trace-1", "user:test", types.NewPermissionContext(nil))

	call := &types.ResourceCall{
		Context:    execCtx,
		URI:        "fn:///arg1/arg2", // No app name
		Headers:     make(map[string]string),
		Body:        nil,
		Permissions: []string{"execute"},
	}

	result, err := driver.HandleCall(ctx, call)
	assert.Error(t, err)
	assert.True(t, types.IsInvalidRequest(err))
	assert.Equal(t, 400, result.StatusCode)
	assert.Contains(t, string(result.Body), "must specify app name")
}

func TestFnDriver_HandleCall_ParsesArgs(t *testing.T) {
	pkg := &mockPackageManager{}

	var capturedArgs []string
	invoker := &mockInvoker{
		invokeFn: func(ctx context.Context, req *types.InvocationRequest) (*types.InvocationResult, error) {
			capturedArgs = req.Args
			return &types.InvocationResult{
				RequestID:  req.Context.RequestID,
				Stdout:     []byte("ok"),
				Stderr:     []byte(""),
				ExitCode:   0,
				Duration:   100 * time.Millisecond,
				MemoryUsed: 1024,
			}, nil
		},
	}

	driver := NewFnDriver(pkg, 0)
	driver.SetInvoker(invoker)

	ctx := context.Background()
	execCtx := types.NewExecutionContext("req-1", "trace-1", "user:test", types.NewPermissionContext(nil))

	call := &types.ResourceCall{
		Context:    execCtx,
		URI:        "fn://my-app/arg1/arg2/arg3",
		Headers:     make(map[string]string),
		Body:        nil,
		Permissions: []string{"execute"},
	}

	result, err := driver.HandleCall(ctx, call)
	assert.NoError(t, err)
	assert.Equal(t, 200, result.StatusCode)

	require.Equal(t, 3, len(capturedArgs))
	assert.Equal(t, "arg1", capturedArgs[0])
	assert.Equal(t, "arg2", capturedArgs[1])
	assert.Equal(t, "arg3", capturedArgs[2])
}

func TestFnDriver_HandleCall_PermissionIntersection(t *testing.T) {
	pkg := &mockPackageManager{}

	var capturedPerms *types.PermissionContext
	invoker := &mockInvoker{
		invokeFn: func(ctx context.Context, req *types.InvocationRequest) (*types.InvocationResult, error) {
			capturedPerms = req.Context.PermissionContext
			return &types.InvocationResult{
				RequestID:  req.Context.RequestID,
				Stdout:     []byte("ok"),
				Stderr:     []byte(""),
				ExitCode:   0,
				Duration:   100 * time.Millisecond,
				MemoryUsed: 1024,
			}, nil
		},
	}

	driver := NewFnDriver(pkg, 0)
	driver.SetInvoker(invoker)

	ctx := context.Background()

	// Create parent context with permissions
	parentPerms := types.NewPermissionContext([]types.PermissionEntry{
		{URIPattern: "file:///data/*", Permissions: []string{"read", "write"}},
	})

	execCtx := types.NewExecutionContext("req-1", "trace-1", "user:test", parentPerms)

	call := &types.ResourceCall{
		Context:    execCtx,
		URI:        "fn://my-app",
		Headers:     make(map[string]string),
		Body:        nil,
		Permissions: []string{"execute"},
	}

	result, err := driver.HandleCall(ctx, call)
	assert.NoError(t, err)
	assert.Equal(t, 200, result.StatusCode)

	// Verify child got permissions (intersected with itself for MVP)
	assert.NotNil(t, capturedPerms)
	assert.Equal(t, 1, len(capturedPerms.Entries))
}

func TestFnDriver_GetCallDepth(t *testing.T) {
	driver := NewFnDriver(&mockPackageManager{}, 0)

	tests := []struct {
		name      string
		requestID string
		want      int
	}{
		{
			name:      "top level",
			requestID: "req-1",
			want:      0,
		},
		{
			name:      "depth 1",
			requestID: "req-1.app1",
			want:      1,
		},
		{
			name:      "depth 2",
			requestID: "req-1.app1.app2",
			want:      2,
		},
		{
			name:      "depth 3",
			requestID: "req-1.app1.app2.app3",
			want:      3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := driver.getCallDepth("trace-1", tt.requestID)
			assert.Equal(t, tt.want, got)
		})
	}
}
