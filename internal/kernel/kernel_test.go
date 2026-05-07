package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wazeos/wazeos/internal/kernel/iobus"
	"github.com/wazeos/wazeos/internal/types"
)

// Mock implementations for testing

type mockRequestDriver struct {
	name     string
	patterns []string
	invoker  types.InvocationHandler
	started  bool
	stopped  bool
}

func (m *mockRequestDriver) Name() string {
	return m.name
}

func (m *mockRequestDriver) Patterns() []string {
	return m.patterns
}

func (m *mockRequestDriver) Start(ctx context.Context) error {
	m.started = true
	return nil
}

func (m *mockRequestDriver) Stop(ctx context.Context) error {
	m.stopped = true
	return nil
}

func (m *mockRequestDriver) SetInvoker(invoker types.InvocationHandler) {
	m.invoker = invoker
}

type mockResourceDriver struct {
	name     string
	patterns []string
}

func (m *mockResourceDriver) Name() string {
	return m.name
}

func (m *mockResourceDriver) Patterns() []string {
	return m.patterns
}

func (m *mockResourceDriver) HandleCall(ctx context.Context, call *types.ResourceCall) (*types.ResourceResult, error) {
	return &types.ResourceResult{
		StatusCode: 200,
		Headers:    make(map[string]string),
		Body:       []byte("mock response"),
	}, nil
}

type mockSecurityAuthn struct {
	name string
}

func (m *mockSecurityAuthn) Name() string {
	return m.name
}

func (m *mockSecurityAuthn) Authenticate(ctx context.Context, payload *types.AuthPayload) (string, error) {
	return "user:test", nil
}

type mockSecurityAuthz struct{}

func (m *mockSecurityAuthz) Name() string {
	return "kernel.security.authz"
}

func (m *mockSecurityAuthz) GetPermissions(ctx context.Context, principal string) (*types.PermissionContext, error) {
	return types.NewPermissionContext(nil), nil
}

func (m *mockSecurityAuthz) SetPermissions(ctx context.Context, principal string, permissions *types.PermissionContext) error {
	return nil
}

func (m *mockSecurityAuthz) CheckAccess(uri string, requiredPermissions []string, permissions *types.PermissionContext) error {
	return nil
}

type mockPackageManager struct{}

func (m *mockPackageManager) Name() string {
	return "kernel.pkg"
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
	// For testing, return a mock appID for "test" app
	if appName == "test" {
		return "test/app_1.0.0", nil
	}
	return "", types.ErrNotFound
}

func (m *mockPackageManager) GetWasmBinary(appID string) ([]byte, error) {
	return []byte("mock wasm binary"), nil
}

type mockRuntimeExec struct {
	bus types.ResourceBus
}

func (m *mockRuntimeExec) Name() string {
	return "kernel.runtime.exec"
}

func (m *mockRuntimeExec) LoadApp(ctx context.Context, appID string, wasmBytes []byte) error {
	return nil
}

func (m *mockRuntimeExec) LoadDriver(ctx context.Context, appID string, wasmBytes []byte, metadata *types.AppMetadata) error {
	return nil
}

func (m *mockRuntimeExec) UnloadApp(ctx context.Context, appID string) error {
	return nil
}

func (m *mockRuntimeExec) Execute(ctx context.Context, req *types.InvocationRequest) (*types.InvocationResult, error) {
	return &types.InvocationResult{
		RequestID:  req.Context.RequestID,
		Stdout:     []byte("test output"),
		Stderr:     []byte(""),
		ExitCode:   0,
		Duration:   100 * time.Millisecond,
		MemoryUsed: 1024,
	}, nil
}

func (m *mockRuntimeExec) RegisterHostFunction(namespace, name string, fn types.HostFunction) error {
	return nil
}

func (m *mockRuntimeExec) SetResourceBus(bus types.ResourceBus) {
	m.bus = bus
}

func (m *mockRuntimeExec) GetMetadata(ctx context.Context, wasmBytes []byte) (*types.AppMetadata, error) {
	return &types.AppMetadata{
		Name:    "test-app",
		Version: "1.0.0",
		Author:  "test",
	}, nil
}

type mockTelemetry struct{}

func (m *mockTelemetry) Name() string {
	return "kernel.runtime.telemetry"
}

func (m *mockTelemetry) RecordInvocation(appID string, duration time.Duration, success bool) {}

func (m *mockTelemetry) RecordMemoryUsage(appID string, bytes int64) {}

func (m *mockTelemetry) RecordCacheEvent(appID string, eventType types.CacheEventType) {}

func (m *mockTelemetry) GetMetrics() *types.MetricsSnapshot {
	return &types.MetricsSnapshot{
		InvocationCounts: make(map[string]int64),
		AverageDurations: make(map[string]float64),
		MemoryUsage:      make(map[string]int64),
	}
}

// Tests

func TestNew(t *testing.T) {
	k := New()
	assert.NotNil(t, k)
}

func TestKernel_RegisterRequestDriver(t *testing.T) {
	tests := []struct {
		name    string
		driver  types.RequestDriver
		wantErr bool
	}{
		{
			name: "valid driver",
			driver: &mockRequestDriver{
				name:     "test-driver",
				patterns: []string{"http://*/test"},
			},
			wantErr: false,
		},
		{
			name:    "nil driver",
			driver:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := New()
			err := k.RegisterRequestDriver(tt.driver)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestKernel_RegisterRequestDriver_Duplicate(t *testing.T) {
	k := New()

	driver1 := &mockRequestDriver{name: "test", patterns: []string{"*"}}
	driver2 := &mockRequestDriver{name: "test", patterns: []string{"*"}}

	err := k.RegisterRequestDriver(driver1)
	assert.NoError(t, err)

	err = k.RegisterRequestDriver(driver2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestKernel_RegisterRequestDriver_AfterStart(t *testing.T) {
	k := New()

	// Set up required components
	require.NoError(t, k.RegisterRequestDriver(&mockRequestDriver{name: "test", patterns: []string{"*"}}))
	require.NoError(t, k.SetSecurityAuthz(&mockSecurityAuthz{}))
	require.NoError(t, k.SetPackageManager(&mockPackageManager{}))
	require.NoError(t, k.SetRuntimeExec(&mockRuntimeExec{}))

	ctx := context.Background()
	err := k.Start(ctx)
	require.NoError(t, err)

	// Try to register after start
	err = k.RegisterRequestDriver(&mockRequestDriver{name: "new", patterns: []string{"*"}})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "after kernel has started")

	_ = k.Stop(ctx)
}

func TestKernel_RegisterResourceDriver(t *testing.T) {
	k := New()

	driver := &mockResourceDriver{
		name:     "test-resource",
		patterns: []string{"file:///*"},
	}

	err := k.RegisterResourceDriver(driver)
	assert.NoError(t, err)
}

func TestKernel_RegisterResourceDriver_Nil(t *testing.T) {
	k := New()
	err := k.RegisterResourceDriver(nil)
	assert.Error(t, err)
}

func TestKernel_RegisterSecurityAuthn(t *testing.T) {
	k := New()

	driver := &mockSecurityAuthn{name: "test-authn"}

	err := k.RegisterSecurityAuthn(driver)
	assert.NoError(t, err)
}

func TestKernel_RegisterSecurityAuthn_Duplicate(t *testing.T) {
	k := New()

	driver1 := &mockSecurityAuthn{name: "test"}
	driver2 := &mockSecurityAuthn{name: "test"}

	err := k.RegisterSecurityAuthn(driver1)
	assert.NoError(t, err)

	err = k.RegisterSecurityAuthn(driver2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestKernel_SetSecurityAuthz(t *testing.T) {
	k := New()

	authz := &mockSecurityAuthz{}

	err := k.SetSecurityAuthz(authz)
	assert.NoError(t, err)
}

func TestKernel_SetSecurityAuthz_Nil(t *testing.T) {
	k := New()
	err := k.SetSecurityAuthz(nil)
	assert.Error(t, err)
}

func TestKernel_SetSecurityAuthz_AlreadySet(t *testing.T) {
	k := New()

	authz1 := &mockSecurityAuthz{}
	authz2 := &mockSecurityAuthz{}

	err := k.SetSecurityAuthz(authz1)
	assert.NoError(t, err)

	err = k.SetSecurityAuthz(authz2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already set")
}

func TestKernel_SetPackageManager(t *testing.T) {
	k := New()

	pkg := &mockPackageManager{}

	err := k.SetPackageManager(pkg)
	assert.NoError(t, err)
}

func TestKernel_SetPackageManager_Nil(t *testing.T) {
	k := New()
	err := k.SetPackageManager(nil)
	assert.Error(t, err)
}

func TestKernel_SetRuntimeExec(t *testing.T) {
	k := New()

	exec := &mockRuntimeExec{}

	err := k.SetRuntimeExec(exec)
	assert.NoError(t, err)
}

func TestKernel_SetRuntimeExec_Nil(t *testing.T) {
	k := New()
	err := k.SetRuntimeExec(nil)
	assert.Error(t, err)
}

func TestKernel_SetTelemetry(t *testing.T) {
	k := New()

	tel := &mockTelemetry{}

	err := k.SetTelemetry(tel)
	assert.NoError(t, err)
}

func TestKernel_SetTelemetry_Nil(t *testing.T) {
	k := New()
	err := k.SetTelemetry(nil)
	assert.Error(t, err)
}

func TestKernel_Start(t *testing.T) {
	k := New()

	requestDriver := &mockRequestDriver{name: "test", patterns: []string{"*"}}
	require.NoError(t, k.RegisterRequestDriver(requestDriver))
	require.NoError(t, k.SetSecurityAuthz(&mockSecurityAuthz{}))
	require.NoError(t, k.SetPackageManager(&mockPackageManager{}))

	runtime := &mockRuntimeExec{}
	require.NoError(t, k.SetRuntimeExec(runtime))

	ctx := context.Background()
	err := k.Start(ctx)
	assert.NoError(t, err)

	// Check that driver was started
	assert.True(t, requestDriver.started)

	// Check that invoker was set
	assert.NotNil(t, requestDriver.invoker)

	// Check that runtime exec has resource bus
	assert.NotNil(t, runtime.bus)

	_ = k.Stop(ctx)
}

func TestKernel_Start_MissingComponents(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(k types.Kernel)
		wantErr string
	}{
		{
			name: "missing runtime exec",
			setup: func(k types.Kernel) {
				_ = k.RegisterRequestDriver(&mockRequestDriver{name: "test", patterns: []string{"*"}})
				_ = k.SetSecurityAuthz(&mockSecurityAuthz{})
				_ = k.SetPackageManager(&mockPackageManager{})
			},
			wantErr: "runtime exec not set",
		},
		{
			name: "missing authz",
			setup: func(k types.Kernel) {
				_ = k.RegisterRequestDriver(&mockRequestDriver{name: "test", patterns: []string{"*"}})
				_ = k.SetRuntimeExec(&mockRuntimeExec{})
				_ = k.SetPackageManager(&mockPackageManager{})
			},
			wantErr: "security authz not set",
		},
		{
			name: "missing package manager",
			setup: func(k types.Kernel) {
				_ = k.RegisterRequestDriver(&mockRequestDriver{name: "test", patterns: []string{"*"}})
				_ = k.SetSecurityAuthz(&mockSecurityAuthz{})
				_ = k.SetRuntimeExec(&mockRuntimeExec{})
			},
			wantErr: "package manager not set",
		},
		{
			name: "no request drivers",
			setup: func(k types.Kernel) {
				_ = k.SetSecurityAuthz(&mockSecurityAuthz{})
				_ = k.SetPackageManager(&mockPackageManager{})
				_ = k.SetRuntimeExec(&mockRuntimeExec{})
			},
			wantErr: "no request drivers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := New()
			tt.setup(k)

			ctx := context.Background()
			err := k.Start(ctx)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestKernel_Start_AlreadyStarted(t *testing.T) {
	k := New()

	require.NoError(t, k.RegisterRequestDriver(&mockRequestDriver{name: "test", patterns: []string{"*"}}))
	require.NoError(t, k.SetSecurityAuthz(&mockSecurityAuthz{}))
	require.NoError(t, k.SetPackageManager(&mockPackageManager{}))
	require.NoError(t, k.SetRuntimeExec(&mockRuntimeExec{}))

	ctx := context.Background()
	err := k.Start(ctx)
	require.NoError(t, err)

	// Try to start again
	err = k.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already started")

	_ = k.Stop(ctx)
}

func TestKernel_Stop(t *testing.T) {
	k := New()

	requestDriver := &mockRequestDriver{name: "test", patterns: []string{"*"}}
	require.NoError(t, k.RegisterRequestDriver(requestDriver))
	require.NoError(t, k.SetSecurityAuthz(&mockSecurityAuthz{}))
	require.NoError(t, k.SetPackageManager(&mockPackageManager{}))
	require.NoError(t, k.SetRuntimeExec(&mockRuntimeExec{}))

	ctx := context.Background()
	err := k.Start(ctx)
	require.NoError(t, err)

	err = k.Stop(ctx)
	assert.NoError(t, err)

	// Check that driver was stopped
	assert.True(t, requestDriver.stopped)
}

func TestKernel_Stop_NotStarted(t *testing.T) {
	k := New()

	ctx := context.Background()
	err := k.Stop(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not started")
}

func TestInvocationHandler_Invoke(t *testing.T) {
	handler := &invocationHandler{
		runtimeExec: &mockRuntimeExec{},
		telemetry:   &mockTelemetry{},
		authz:       &mockSecurityAuthz{},
	}

	ctx := context.Background()
	execCtx := types.NewExecutionContext("req-1", "trace-1", "user:test", types.NewPermissionContext(nil))

	req := &types.InvocationRequest{
		Context: execCtx,
		AppID:   "test/app_1.0.0",
		Args:    []string{"arg1", "arg2"},
	}

	result, err := handler.Invoke(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "req-1", result.RequestID)
	assert.Equal(t, "test output", string(result.Stdout))
	assert.Equal(t, 0, result.ExitCode)
}

func TestInvocationHandler_Invoke_NilRequest(t *testing.T) {
	handler := &invocationHandler{
		runtimeExec: &mockRuntimeExec{},
	}

	ctx := context.Background()
	result, err := handler.Invoke(ctx, nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, types.IsInvalidRequest(err))
}

type failingRuntimeExec struct {
	err error
}

func (f *failingRuntimeExec) Name() string {
	return "kernel.runtime.exec"
}

func (f *failingRuntimeExec) LoadApp(ctx context.Context, appID string, wasmBytes []byte) error {
	return nil
}

func (f *failingRuntimeExec) LoadDriver(ctx context.Context, appID string, wasmBytes []byte, metadata *types.AppMetadata) error {
	return nil
}

func (f *failingRuntimeExec) UnloadApp(ctx context.Context, appID string) error {
	return nil
}

func (f *failingRuntimeExec) Execute(ctx context.Context, req *types.InvocationRequest) (*types.InvocationResult, error) {
	return nil, f.err
}

func (f *failingRuntimeExec) RegisterHostFunction(namespace, name string, fn types.HostFunction) error {
	return nil
}

func (f *failingRuntimeExec) SetResourceBus(bus types.ResourceBus) {
}

func (f *failingRuntimeExec) GetMetadata(ctx context.Context, wasmBytes []byte) (*types.AppMetadata, error) {
	return &types.AppMetadata{
		Name:    "test-app",
		Version: "1.0.0",
		Author:  "test",
	}, nil
}

func TestInvocationHandler_Invoke_ExecutionError(t *testing.T) {
	execErr := errors.New("execution failed")
	failingExec := &failingRuntimeExec{err: execErr}

	handler := &invocationHandler{
		runtimeExec: failingExec,
		telemetry:   &mockTelemetry{},
	}

	ctx := context.Background()
	execCtx := types.NewExecutionContext("req-1", "trace-1", "user:test", types.NewPermissionContext(nil))

	req := &types.InvocationRequest{
		Context: execCtx,
		AppID:   "test/app_1.0.0",
		Args:    []string{},
	}

	result, err := handler.Invoke(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, execErr, err)
}

// Mock RuntimeExec that tracks registered host functions
type trackingRuntimeExec struct {
	hostFunctions map[string]types.HostFunction
	resourceBus   types.ResourceBus
}

func newTrackingRuntimeExec() *trackingRuntimeExec {
	return &trackingRuntimeExec{
		hostFunctions: make(map[string]types.HostFunction),
	}
}

func (t *trackingRuntimeExec) Name() string {
	return "kernel.runtime.exec"
}

func (t *trackingRuntimeExec) LoadApp(ctx context.Context, appID string, wasmBytes []byte) error {
	return nil
}

func (t *trackingRuntimeExec) LoadDriver(ctx context.Context, appID string, wasmBytes []byte, metadata *types.AppMetadata) error {
	return nil
}

func (t *trackingRuntimeExec) UnloadApp(ctx context.Context, appID string) error {
	return nil
}

func (t *trackingRuntimeExec) Execute(ctx context.Context, req *types.InvocationRequest) (*types.InvocationResult, error) {
	return &types.InvocationResult{
		RequestID: req.Context.RequestID,
		ExitCode:  0,
		Duration:  time.Millisecond,
	}, nil
}

func (t *trackingRuntimeExec) RegisterHostFunction(namespace, name string, fn types.HostFunction) error {
	key := namespace + "." + name
	t.hostFunctions[key] = fn
	return nil
}

func (t *trackingRuntimeExec) SetResourceBus(bus types.ResourceBus) {
	t.resourceBus = bus
}

func (t *trackingRuntimeExec) GetMetadata(ctx context.Context, wasmBytes []byte) (*types.AppMetadata, error) {
	return &types.AppMetadata{
		Name:    "test-app",
		Version: "1.0.0",
		Author:  "test",
	}, nil
}

func TestKernel_RegisterHostFunctions(t *testing.T) {
	k := New()
	trackingExec := newTrackingRuntimeExec()

	// Set up kernel
	err := k.SetRuntimeExec(trackingExec)
	require.NoError(t, err)

	err = k.SetSecurityAuthz(&mockSecurityAuthz{})
	require.NoError(t, err)

	err = k.SetPackageManager(&mockPackageManager{})
	require.NoError(t, err)

	err = k.RegisterRequestDriver(&mockRequestDriver{
		name:     "io.request.http",
		patterns: []string{"http://*"},
	})
	require.NoError(t, err)

	// Start kernel (this should register host functions)
	ctx := context.Background()
	err = k.Start(ctx)
	require.NoError(t, err)
	defer k.Stop(ctx)

	// Verify host functions were registered
	assert.Contains(t, trackingExec.hostFunctions, "kernel.resource_call")
	assert.Contains(t, trackingExec.hostFunctions, "kernel.authz_check")
	assert.Contains(t, trackingExec.hostFunctions, "kernel.pkg_resolve")
	assert.Len(t, trackingExec.hostFunctions, 3)
}

func TestKernel_HostResourceCall(t *testing.T) {
	k := New().(*kernel)

	// Set up resource bus with a mock driver
	mockDriver := &mockResourceDriver{
		name:     "io.resource.test",
		patterns: []string{"test://*"},
	}
	k.resourceBus = iobus.New(nil)
	err := k.resourceBus.RegisterDriver(mockDriver)
	require.NoError(t, err)

	ctx := context.Background()
	execCtx := types.NewExecutionContext("req-1", "trace-1", "user:test", types.NewPermissionContext(nil))

	// Create a resource call
	call := &types.ResourceCall{
		Context:     execCtx,
		URI:         "test://example",
		Headers:     make(map[string]string),
		Body:        []byte{},
		Permissions: []string{"read"},
	}

	// Marshal to JSON
	callJSON, err := json.Marshal(call)
	require.NoError(t, err)

	// Call host function
	resultJSON, err := k.hostResourceCall(ctx, callJSON)
	require.NoError(t, err)

	// Verify result
	var result types.ResourceResult
	err = json.Unmarshal(resultJSON, &result)
	require.NoError(t, err)

	assert.Equal(t, 200, result.StatusCode)
	assert.Equal(t, []byte("mock response"), result.Body)
}

func TestKernel_HostAuthzCheck(t *testing.T) {
	k := New().(*kernel)
	k.authz = &mockSecurityAuthz{}

	ctx := context.Background()
	permissions := types.NewPermissionContext([]types.PermissionEntry{
		{URIPattern: "file:///*", Permissions: []string{"read"}},
	})

	// Test allowed access
	input := struct {
		URI         string                   `json:"uri"`
		Mode        string                   `json:"mode"`
		Permissions *types.PermissionContext `json:"permissions"`
	}{
		URI:         "file:///test.txt",
		Mode:        "r",
		Permissions: permissions,
	}

	inputJSON, err := json.Marshal(input)
	require.NoError(t, err)

	outputJSON, err := k.hostAuthzCheck(ctx, inputJSON)
	require.NoError(t, err)

	var output struct {
		Allowed bool   `json:"allowed"`
		Error   string `json:"error,omitempty"`
	}
	err = json.Unmarshal(outputJSON, &output)
	require.NoError(t, err)

	assert.True(t, output.Allowed)
	assert.Empty(t, output.Error)
}

func TestKernel_HostPkgResolve(t *testing.T) {
	k := New().(*kernel)
	k.pkg = &mockPackageManager{}

	ctx := context.Background()

	// Test resolving an app name
	input := struct {
		Name string `json:"name"`
	}{
		Name: "test",
	}

	inputJSON, err := json.Marshal(input)
	require.NoError(t, err)

	outputJSON, err := k.hostPkgResolve(ctx, inputJSON)
	require.NoError(t, err)

	var output struct {
		AppID string `json:"appID,omitempty"`
		Error string `json:"error,omitempty"`
	}
	err = json.Unmarshal(outputJSON, &output)
	require.NoError(t, err)

	assert.Equal(t, "test/app_1.0.0", output.AppID)
	assert.Empty(t, output.Error)
}
