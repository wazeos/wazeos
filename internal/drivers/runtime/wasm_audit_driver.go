package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/wazeos/wazeos/internal/types"
)

// WasmAuditDriver wraps a WASM audit driver module.
type WasmAuditDriver struct {
	name           string
	runtimeExec    *RuntimeExec
	compiledModule wazero.CompiledModule
	timeout        time.Duration
	config         map[string]string // Configuration (e.g., Kafka bootstrap servers, topic)
}

// NewWasmAuditDriver creates a new WASM audit driver wrapper.
func NewWasmAuditDriver(name string, rt *RuntimeExec, compiled wazero.CompiledModule, config map[string]string) *WasmAuditDriver {
	timeout := 10 * time.Second // Default timeout for audit operations
	return &WasmAuditDriver{
		name:           name,
		runtimeExec:        rt,
		compiledModule: compiled,
		timeout:        timeout,
		config:         config,
	}
}

// Name returns the driver class.
func (w *WasmAuditDriver) Name() string {
	return w.name
}

// RecordResourceCall logs a resource call event.
func (w *WasmAuditDriver) RecordResourceCall(ctx context.Context, event *types.ResourceCallAuditEvent) error {
	return w.recordEvent(ctx, "resource_call", event)
}

// RecordAuthzCheck logs an authorization check event.
func (w *WasmAuditDriver) RecordAuthzCheck(ctx context.Context, event *types.AuthzCheckAuditEvent) error {
	return w.recordEvent(ctx, "authz_check", event)
}

// RecordAppInvoke logs an app invocation event.
func (w *WasmAuditDriver) RecordAppInvoke(ctx context.Context, event *types.AppInvokeAuditEvent) error {
	return w.recordEvent(ctx, "app_invoke", event)
}

// recordEvent sends an audit event to the WASM driver.
func (w *WasmAuditDriver) recordEvent(ctx context.Context, eventType string, event interface{}) error {
	// Create request with event type and data
	request := map[string]interface{}{
		"eventType": eventType,
		"event":     event,
	}

	// Add configuration (Kafka bootstrap, topic, etc.)
	for k, v := range w.config {
		request[k] = v
	}

	// Marshal to JSON
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal audit request: %w", err)
	}

	// Create timeout context
	execCtx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	// Prepare stdin/stdout buffers
	stdin := bytes.NewReader(requestJSON)
	var stdout, stderr bytes.Buffer

	// Create module config
	moduleConfig := wazero.NewModuleConfig().
		WithStdin(stdin).
		WithStdout(&stdout).
		WithStderr(&stderr).
		WithName(w.name)

	// Instantiate and run the module
	module, err := w.runtimeExec.runtime.InstantiateModule(execCtx, w.compiledModule, moduleConfig)
	if err != nil {
		// Log error but don't fail - audit should not block operations
		fmt.Fprintf(&stderr, "Failed to instantiate audit driver: %v\n", err)
		return nil // Return nil to not block operations
	}
	defer module.Close(execCtx)

	// Parse result from stdout (should be a simple success/error response)
	var result struct {
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		// Log error but don't fail
		fmt.Fprintf(&stderr, "Failed to parse audit driver response: %v\n", err)
		return nil
	}

	if result.Error != "" {
		// Log error but don't fail - audit should not block operations
		fmt.Fprintf(&stderr, "Audit driver returned error: %s\n", result.Error)
	}

	return nil
}

// Query is not typically supported by WASM audit drivers (write-only).
func (w *WasmAuditDriver) Query(ctx context.Context, filter types.AuditFilter) ([]*types.AuditEvent, error) {
	return nil, types.ErrNotSupported
}
