package resource

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/wazeos/wazeos/internal/types"
)

// FnDriver implements types.ResourceDriver for fn:// URIs (inter-app calls).
type FnDriver struct {
	invoker       types.InvocationHandler
	pkgManager    types.PackageManager
	maxDepth      int
	currentDepths map[string]int // track call depth per trace
}

// NewFnDriver creates a new fn:// driver.
func NewFnDriver(pkgManager types.PackageManager, maxDepth int) *FnDriver {
	if maxDepth == 0 {
		maxDepth = 10 // Default max call depth
	}

	return &FnDriver{
		pkgManager:    pkgManager,
		maxDepth:      maxDepth,
		currentDepths: make(map[string]int),
	}
}

// Name returns the driver class.
func (f *FnDriver) Name() string {
	return "io.resource.fn"
}

// Patterns returns URI patterns this driver handles.
func (f *FnDriver) Patterns() []string {
	return []string{"fn://*/*"}
}

// SetInvoker sets the invocation handler (called by kernel during setup).
func (f *FnDriver) SetInvoker(invoker types.InvocationHandler) {
	f.invoker = invoker
}

// HandleCall processes fn:// calls to execute other apps.
func (f *FnDriver) HandleCall(ctx context.Context, call *types.ResourceCall) (*types.ResourceResult, error) {
	if call == nil {
		return nil, types.ErrInvalidRequest
	}

	if f.invoker == nil {
		return &types.ResourceResult{
			StatusCode: 500,
			Headers:    make(map[string]string),
			Body:       []byte("fn driver not initialized: invoker not set"),
			Error:      types.ErrInternal.Error(),
		}, types.ErrInternal
	}

	// Parse fn:// URI: fn://app-name/arg1/arg2/arg3
	parsed, err := url.Parse(call.URI)
	if err != nil {
		return &types.ResourceResult{
			StatusCode: 400,
			Headers:    make(map[string]string),
			Body:       []byte(fmt.Sprintf("invalid fn:// URI: %v", err)),
			Error:      types.ErrInvalidRequest.Error(),
		}, types.ErrInvalidRequest
	}

	appName := parsed.Host
	if appName == "" {
		return &types.ResourceResult{
			StatusCode: 400,
			Headers:    make(map[string]string),
			Body:       []byte("fn:// URI must specify app name: fn://app-name/args"),
			Error:      types.ErrInvalidRequest.Error(),
		}, types.ErrInvalidRequest
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
		return &types.ResourceResult{
			StatusCode: 404,
			Headers:    make(map[string]string),
			Body:       []byte(fmt.Sprintf("app not found: %s", appName)),
			Error:      types.ErrNotFound.Error(),
		}, types.ErrNotFound
	}

	// Check call depth to prevent infinite recursion
	depth := f.getCallDepth(call.Context.TraceID, call.Context.RequestID)
	if depth >= f.maxDepth {
		return &types.ResourceResult{
			StatusCode: 508, // Loop Detected
			Headers:    make(map[string]string),
			Body:       []byte(fmt.Sprintf("maximum call depth %d exceeded", f.maxDepth)),
			Error:      types.ErrMaxDepthExceeded.Error(),
		}, types.ErrMaxDepthExceeded
	}

	// Detect cycles by checking parent chain
	if f.hasCycle(call.Context, appID) {
		return &types.ResourceResult{
			StatusCode: 508,
			Headers:    make(map[string]string),
			Body:       []byte(fmt.Sprintf("cycle detected: app %s already in call chain", appID)),
			Error:      types.ErrCycleDetected.Error(),
		}, types.ErrCycleDetected
	}

	// Get target app permissions (for MVP, we'll just use caller's permissions intersected)
	// In full implementation, this would come from permission store
	targetPermissions := call.Context.PermissionContext

	// Create child execution context with intersected permissions
	childRequestID := fmt.Sprintf("%s.%s", call.Context.RequestID, appName)
	childCtx := call.Context.ChildContext(childRequestID, targetPermissions)

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
	result, err := f.invoker.Invoke(ctx, invocationReq)
	if err != nil {
		return &types.ResourceResult{
			StatusCode: 500,
			Headers:    make(map[string]string),
			Body:       []byte(fmt.Sprintf("invocation failed: %v", err)),
			Error:      err.Error(),
		}, err
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

	return &types.ResourceResult{
		StatusCode: statusCode,
		Headers:    map[string]string{"X-Exit-Code": fmt.Sprintf("%d", result.ExitCode)},
		Body:       output,
		Error:      "",
	}, nil
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
func (f *FnDriver) hasCycle(execCtx *types.ExecutionContext, targetAppID string) bool {
	// For MVP, simple cycle detection: check if we're calling the same app
	// In full implementation, would track full call chain in context
	// For now, we'll just prevent immediate recursion (A calls A)
	if execCtx.ParentRequestID != nil {
		// Extract app name from parent request ID
		// Format is "req-1.app1.app2" - last segment is app name
		parts := strings.Split(*execCtx.ParentRequestID, ".")
		if len(parts) > 1 {
			parentApp := parts[len(parts)-1]
			if strings.Contains(targetAppID, parentApp) {
				return true
			}
		}
	}
	return false
}
