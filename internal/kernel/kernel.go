package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/wazeos/wazeos/internal/bus"
	"github.com/wazeos/wazeos/internal/types"
)

// kernel implements the types.Kernel interface.
type kernel struct {
	mu sync.RWMutex

	// Drivers
	requestDrivers []types.RequestDriver
	resourceBus    *bus.Bus
	authnDrivers   []types.SecurityAuthn
	authz          types.SecurityAuthz
	pkg            types.PackageManager
	runtimeExec    types.RuntimeExec
	telemetry      types.RuntimeTelemetry
	auditDrivers   []types.AuditDriver

	// Driver policy management
	policyRegistry types.DriverPolicyRegistry
	driverCounts   map[string]int // driver class -> count

	// State
	started bool
	ctx     context.Context
	cancel  context.CancelFunc
}

// New creates a new kernel instance.
func New() types.Kernel {
	driverCounts := make(map[string]int)
	// The internal resource bus counts as the io.bus driver
	driverCounts["io.bus"] = 1

	return &kernel{
		requestDrivers: make([]types.RequestDriver, 0),
		resourceBus: bus.New(&bus.Config{
			Logger: &bus.StderrLogger{
				Prefix: "[KERNEL-BUS]",
				Writer: os.Stderr,
			},
		}),
		authnDrivers:   make([]types.SecurityAuthn, 0),
		policyRegistry: NewDriverPolicyRegistry(),
		driverCounts:   driverCounts,
	}
}

// RegisterRequestDriver adds an ingress driver.
func (k *kernel) RegisterRequestDriver(driver types.RequestDriver) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.started {
		return fmt.Errorf("cannot register drivers after kernel has started")
	}

	if driver == nil {
		return fmt.Errorf("driver cannot be nil")
	}

	// Check for duplicate driver names
	for _, existing := range k.requestDrivers {
		if existing.Name() == driver.Name() {
			return fmt.Errorf("request driver %q already registered", driver.Name())
		}
	}

	// Track driver count by class
	driverClass := extractDriverClass(driver.Name())
	k.driverCounts[driverClass]++

	// Validate cardinality policy (before adding)
	if err := k.policyRegistry.ValidateDriverCount(driverClass, k.driverCounts[driverClass]); err != nil {
		k.driverCounts[driverClass]-- // Rollback
		return fmt.Errorf("policy violation: %w", err)
	}

	k.requestDrivers = append(k.requestDrivers, driver)
	return nil
}

// RegisterResourceDriver adds an egress driver.
func (k *kernel) RegisterResourceDriver(driver types.ResourceDriver) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.started {
		return fmt.Errorf("cannot register drivers after kernel has started")
	}

	if driver == nil {
		return fmt.Errorf("driver cannot be nil")
	}

	// Track driver count by class
	driverClass := extractDriverClass(driver.Name())
	k.driverCounts[driverClass]++

	// Validate cardinality policy (before adding)
	if err := k.policyRegistry.ValidateDriverCount(driverClass, k.driverCounts[driverClass]); err != nil {
		k.driverCounts[driverClass]-- // Rollback
		return fmt.Errorf("policy violation: %w", err)
	}

	return k.resourceBus.RegisterDriver(driver)
}

// RegisterSecurityAuthn adds an authentication driver.
func (k *kernel) RegisterSecurityAuthn(driver types.SecurityAuthn) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.started {
		return fmt.Errorf("cannot register drivers after kernel has started")
	}

	if driver == nil {
		return fmt.Errorf("driver cannot be nil")
	}

	// Check for duplicate driver names
	for _, existing := range k.authnDrivers {
		if existing.Name() == driver.Name() {
			return fmt.Errorf("authn driver %q already registered", driver.Name())
		}
	}

	k.authnDrivers = append(k.authnDrivers, driver)
	return nil
}

// SetSecurityAuthz sets the authorization driver (only one).
func (k *kernel) SetSecurityAuthz(authz types.SecurityAuthz) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.started {
		return fmt.Errorf("cannot set authz after kernel has started")
	}

	if authz == nil {
		return fmt.Errorf("authz cannot be nil")
	}

	if k.authz != nil {
		return fmt.Errorf("authz already set")
	}

	k.authz = authz
	k.driverCounts["security.authz"] = 1
	return nil
}

// SetPackageManager sets the package manager (only one).
func (k *kernel) SetPackageManager(pkg types.PackageManager) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.started {
		return fmt.Errorf("cannot set package manager after kernel has started")
	}

	if pkg == nil {
		return fmt.Errorf("package manager cannot be nil")
	}

	if k.pkg != nil {
		return fmt.Errorf("package manager already set")
	}

	k.pkg = pkg
	k.driverCounts["pkgmgr"] = 1
	return nil
}

// SetRuntimeExec sets the execution engine (only one).
func (k *kernel) SetRuntimeExec(exec types.RuntimeExec) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.started {
		return fmt.Errorf("cannot set runtime exec after kernel has started")
	}

	if exec == nil {
		return fmt.Errorf("runtime exec cannot be nil")
	}

	if k.runtimeExec != nil {
		return fmt.Errorf("runtime exec already set")
	}

	k.runtimeExec = exec
	k.driverCounts["runtime.exec"] = 1
	return nil
}

// SetTelemetry sets the telemetry driver (only one).
func (k *kernel) SetTelemetry(telemetry types.RuntimeTelemetry) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.started {
		return fmt.Errorf("cannot set telemetry after kernel has started")
	}

	if telemetry == nil {
		return fmt.Errorf("telemetry cannot be nil")
	}

	if k.telemetry != nil {
		return fmt.Errorf("telemetry already set")
	}

	k.telemetry = telemetry
	return nil
}

// RegisterAuditDriver adds an audit driver.
func (k *kernel) RegisterAuditDriver(driver types.AuditDriver) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.started {
		return fmt.Errorf("cannot register drivers after kernel has started")
	}

	if driver == nil {
		return fmt.Errorf("audit driver cannot be nil")
	}

	// Check for duplicate driver names
	for _, existing := range k.auditDrivers {
		if existing.Name() == driver.Name() {
			return fmt.Errorf("audit driver %q already registered", driver.Name())
		}
	}

	k.auditDrivers = append(k.auditDrivers, driver)

	// Register audit driver with resource bus so it can emit audit events
	k.resourceBus.RegisterAuditDriver(driver)

	return nil
}

// Start initializes all drivers and begins processing.
func (k *kernel) Start(ctx context.Context) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.started {
		return fmt.Errorf("kernel already started")
	}

	// Validate required components
	if k.runtimeExec == nil {
		return fmt.Errorf("runtime exec not set")
	}
	if k.authz == nil {
		return fmt.Errorf("security authz not set")
	}
	if k.pkg == nil {
		return fmt.Errorf("package manager not set")
	}
	if len(k.requestDrivers) == 0 {
		return fmt.Errorf("no request drivers registered")
	}

	// Validate driver class policies
	requiredClasses := k.policyRegistry.GetRequiredClasses()
	for _, class := range requiredClasses {
		count := k.driverCounts[class]
		if err := k.policyRegistry.ValidateDriverCount(class, count); err != nil {
			return fmt.Errorf("startup validation failed: %w", err)
		}
	}

	// Create cancellable context
	k.ctx, k.cancel = context.WithCancel(ctx)

	// Wire up runtime exec with resource bus
	k.runtimeExec.SetResourceBus(k.resourceBus)

	// Register host functions for WASM drivers
	if err := k.registerHostFunctions(); err != nil {
		return fmt.Errorf("failed to register host functions: %w", err)
	}

	// Create invocation handler for request drivers
	invoker := &invocationHandler{
		runtimeExec: k.runtimeExec,
		telemetry:   k.telemetry,
		authz:       k.authz,
	}

	// Start request drivers
	for _, driver := range k.requestDrivers {
		driver.SetInvoker(invoker)
		if err := driver.Start(k.ctx); err != nil {
			return fmt.Errorf("failed to start request driver %q: %w", driver.Name(), err)
		}
	}

	k.started = true
	return nil
}

// Stop gracefully shuts down all drivers.
func (k *kernel) Stop(ctx context.Context) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if !k.started {
		return fmt.Errorf("kernel not started")
	}

	// Cancel kernel context
	if k.cancel != nil {
		k.cancel()
	}

	// Stop all request drivers
	var errs []error
	for _, driver := range k.requestDrivers {
		if err := driver.Stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop request driver %q: %w", driver.Name(), err))
		}
	}

	k.started = false

	if len(errs) > 0 {
		return fmt.Errorf("errors stopping drivers: %v", errs)
	}

	return nil
}

// registerHostFunctions registers kernel services as host functions for WASM drivers.
func (k *kernel) registerHostFunctions() error {
	// Register resource.call - allows WASM drivers to make resource calls
	if err := k.runtimeExec.RegisterHostFunction("kernel", "resource_call", k.hostResourceCall); err != nil {
		return err
	}

	// Register authz.check - allows WASM drivers to check permissions
	if err := k.runtimeExec.RegisterHostFunction("kernel", "authz_check", k.hostAuthzCheck); err != nil {
		return err
	}

	// Register pkg.resolve - allows WASM drivers to resolve app names
	if err := k.runtimeExec.RegisterHostFunction("kernel", "pkg_resolve", k.hostPkgResolve); err != nil {
		return err
	}

	return nil
}

// hostResourceCall implements the kernel.resource_call host function.
// Input: JSON-encoded ResourceCall
// Output: JSON-encoded ResourceResult
func (k *kernel) hostResourceCall(ctx context.Context, params []byte) ([]byte, error) {
	var call types.ResourceCall
	if err := json.Unmarshal(params, &call); err != nil {
		return nil, fmt.Errorf("failed to unmarshal resource call: %w", err)
	}

	result, err := k.resourceBus.Call(ctx, &call)
	if err != nil {
		return nil, err
	}

	return json.Marshal(result)
}

// hostAuthzCheck implements the kernel.authz_check host function.
// Input: JSON with {uri: string, requiredPermissions: []string, permissions: PermissionContext}
// Output: JSON with {allowed: bool, error: string}
func (k *kernel) hostAuthzCheck(ctx context.Context, params []byte) ([]byte, error) {
	var input struct {
		URI                 string                   `json:"uri"`
		RequiredPermissions []string                 `json:"requiredPermissions"`
		Permissions         *types.PermissionContext `json:"permissions"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("failed to unmarshal authz check: %w", err)
	}

	err := k.authz.CheckAccess(input.URI, input.RequiredPermissions, input.Permissions)

	output := struct {
		Allowed bool   `json:"allowed"`
		Error   string `json:"error,omitempty"`
	}{
		Allowed: err == nil,
	}
	if err != nil {
		output.Error = err.Error()
	}

	return json.Marshal(output)
}

// hostPkgResolve implements the kernel.pkg_resolve host function.
// Input: JSON with {name: string}
// Output: JSON with {appID: string, error: string}
func (k *kernel) hostPkgResolve(ctx context.Context, params []byte) ([]byte, error) {
	var input struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("failed to unmarshal pkg resolve: %w", err)
	}

	appID, err := k.pkg.Resolve(ctx, input.Name)

	output := struct {
		AppID string `json:"appID,omitempty"`
		Error string `json:"error,omitempty"`
	}{
		AppID: appID,
	}
	if err != nil {
		output.Error = err.Error()
	}

	return json.Marshal(output)
}

// invocationHandler implements types.InvocationHandler.
type invocationHandler struct {
	runtimeExec types.RuntimeExec
	telemetry   types.RuntimeTelemetry
	authz       types.SecurityAuthz
}

// Invoke executes an app and returns the result.
func (h *invocationHandler) Invoke(ctx context.Context, req *types.InvocationRequest) (*types.InvocationResult, error) {
	if req == nil || req.Context == nil {
		return nil, types.ErrInvalidRequest
	}

	// Execute the app
	result, err := h.runtimeExec.Execute(ctx, req)
	if err != nil {
		// Record failed invocation
		if h.telemetry != nil {
			h.telemetry.RecordInvocation(req.AppID, 0, false)
		}
		return nil, err
	}

	// Record successful invocation
	if h.telemetry != nil {
		h.telemetry.RecordInvocation(req.AppID, result.Duration, result.ExitCode == 0)
		if result.MemoryUsed > 0 {
			h.telemetry.RecordMemoryUsage(req.AppID, result.MemoryUsed)
		}
	}

	return result, nil
}
