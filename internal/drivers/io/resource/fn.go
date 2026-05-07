package resource

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/wazeos/wazeos/internal/drivers/base"
	"github.com/wazeos/wazeos/internal/types"
)

// FnDriver implements types.ResourceDriver for fn:// URIs (inter-app calls).
type FnDriver struct {
	*base.BaseDriver
	pkgManager    types.PackageManager
	maxDepth      int
	currentDepths map[string]int // track call depth per trace
}

// NewFnDriver creates a new fn:// driver.
func NewFnDriver(pkgManager types.PackageManager, maxDepth int) *FnDriver {
	if maxDepth == 0 {
		maxDepth = 10 // Default max call depth
	}

	config := base.DefaultConfig("wazeos/fn", "fn://*/*")
	baseDriver := base.NewBaseDriver(config)

	return &FnDriver{
		BaseDriver:    baseDriver,
		pkgManager:    pkgManager,
		maxDepth:      maxDepth,
		currentDepths: make(map[string]int),
	}
}

// HandleCall processes fn:// calls to execute other apps.
func (f *FnDriver) HandleCall(ctx context.Context, call *types.ResourceCall) (*types.ResourceResult, error) {
	if call == nil {
		return nil, types.ErrInvalidRequest
	}

	invoker := f.GetInvoker()
	if invoker == nil {
		return base.InternalError("fn driver not initialized: invoker not set"), types.ErrInternal
	}

	// Parse fn:// URI: fn://app-name/arg1/arg2/arg3
	parsed, err := url.Parse(call.URI)
	if err != nil {
		return base.BadRequest("invalid fn:// URI: %v", err), types.ErrInvalidRequest
	}

	appName := parsed.Host
	if appName == "" {
		return base.BadRequest("fn:// URI must specify app name: fn://app-name/args"), types.ErrInvalidRequest
	}

	// Parse arguments from path
	args := []string{}
	if parsed.Path != "" && parsed.Path != "/" {
		pathSegments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		args = pathSegments
	}

	// Resolve app name to full app ID
	appID, err := f.pkgManager.Resolve(ctx, appName)
	if err != nil {
		return base.NotFound("app not found: %s", appName), types.ErrNotFound
	}

	// Check call depth to prevent infinite recursion
	depth := f.getCallDepth(call.Context.TraceID, call.Context.RequestID)
	if depth >= f.maxDepth {
		return base.ErrorResponse(508, "maximum call depth %d exceeded", f.maxDepth), types.ErrMaxDepthExceeded
	}

	// Detect cycles by checking parent chain
	if f.hasCycle(call.Context, appID) {
		return base.ErrorResponse(508, "cycle detected: app %s already in call chain", appID), types.ErrCycleDetected
	}

	// Get target app permissions (for MVP, we'll just use caller's permissions intersected)
	// In full implementation, this would come from permission store
	targetPermissions := call.Context.PermissionContext

	// Create child execution context with intersected permissions
	childRequestID := fmt.Sprintf("%s.%s", call.Context.RequestID, appName)
	childCtx := call.Context.ChildContext(childRequestID, appID, targetPermissions)

	// Track call depth
	f.incrementCallDepth(call.Context.TraceID, childRequestID)
	defer f.decrementCallDepth(call.Context.TraceID, childRequestID)

	// Create invocation request
	invocationReq := &types.InvocationRequest{
		Context: childCtx,
		AppID:   appID,
		Args:    args,
	}

	// Execute the app
	result, err := invoker.Invoke(ctx, invocationReq)
	if err != nil {
		return base.InternalError("invocation failed: %v", err), err
	}

	// Map invocation result to resource result
	statusCode := 200
	if result.ExitCode != 0 {
		statusCode = 500
	}

	// Combine stdout and stderr
	output := result.Stdout
	if len(result.Stderr) > 0 {
		output = append(output, []byte("\n[stderr]\n")...)
		output = append(output, result.Stderr...)
	}

	return types.SuccessResultWithHeaders(
		statusCode,
		output,
		map[string]string{"X-Exit-Code": fmt.Sprintf("%d", result.ExitCode)},
	), nil
}

// getCallDepth returns the current call depth for a trace/request.
func (f *FnDriver) getCallDepth(traceID, requestID string) int {
	// Count dots in request ID to determine depth
	// e.g., "req-1" = depth 0, "req-1.app1" = depth 1, "req-1.app1.app2" = depth 2
	return strings.Count(requestID, ".")
}

// incrementCallDepth tracks call depth (for cycle detection).
func (f *FnDriver) incrementCallDepth(traceID, requestID string) {
	f.currentDepths[requestID] = f.getCallDepth(traceID, requestID)
}

// decrementCallDepth removes call depth tracking.
func (f *FnDriver) decrementCallDepth(traceID, requestID string) {
	delete(f.currentDepths, requestID)
}

// hasCycle checks if the app is already in the current call chain.
// This prevents infinite recursion cycles like A->B->A or A->B->C->A.
func (f *FnDriver) hasCycle(execCtx *types.ExecutionContext, targetAppID string) bool {
	// Check if targetAppID is already in the call chain
	for _, appID := range execCtx.CallChain {
		if appID == targetAppID {
			return true
		}
	}
	return false
}
