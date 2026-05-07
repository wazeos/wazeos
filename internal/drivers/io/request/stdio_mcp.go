package request

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/wazeos/wazeos/internal/types"
)

// StdioMCPDriver implements MCP transport over stdin/stdout.
type StdioMCPDriver struct {
	mu      sync.RWMutex
	handler *MCPHandler
	reader  io.Reader // stdin (or injected for testing)
	writer  io.Writer // stdout (or injected for testing)
	started bool
	done    chan struct{}
}

// NewStdioMCPDriver creates a new stdio MCP transport driver.
func NewStdioMCPDriver(
	authn []types.SecurityAuthn,
	authz types.SecurityAuthz,
	pkgMgr types.PackageManager,
) *StdioMCPDriver {
	return &StdioMCPDriver{
		handler: NewMCPHandler(authn, authz, pkgMgr),
		reader:  os.Stdin,
		writer:  os.Stdout,
		done:    make(chan struct{}),
	}
}

// Name returns the driver name.
func (d *StdioMCPDriver) Name() string {
	return "wazeos/stdio"
}

// Patterns returns URI patterns this driver handles.
func (d *StdioMCPDriver) Patterns() []string {
	return []string{"stdio://*"}
}

// SetInvoker provides the callback to dispatch invocations.
func (d *StdioMCPDriver) SetInvoker(invoker types.InvocationHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handler.SetInvoker(invoker)
}

// Start begins listening for inbound requests from stdin.
func (d *StdioMCPDriver) Start(ctx context.Context) error {
	d.mu.Lock()
	if d.started {
		d.mu.Unlock()
		return fmt.Errorf("driver already started")
	}

	if d.handler.invoker == nil {
		d.mu.Unlock()
		return fmt.Errorf("invoker not set")
	}

	d.started = true
	d.mu.Unlock()

	// Line-based JSON-RPC I/O loop
	scanner := bufio.NewScanner(d.reader)

	for {
		select {
		case <-ctx.Done():
			close(d.done)
			return nil
		default:
			if !scanner.Scan() {
				// EOF or error
				if err := scanner.Err(); err != nil {
					fmt.Fprintf(os.Stderr, "Scanner error: %v\n", err)
				}
				close(d.done)
				return scanner.Err()
			}

			line := scanner.Text()
			if line == "" {
				continue // Skip empty lines
			}

			d.handleLine(ctx, line)
		}
	}
}

// Stop gracefully shuts down the driver.
func (d *StdioMCPDriver) Stop(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.started {
		return nil
	}

	// Wait for processing to complete or context to cancel
	select {
	case <-d.done:
	case <-ctx.Done():
		return ctx.Err()
	}

	d.started = false
	return nil
}

// handleLine processes a single line of JSON-RPC input.
func (d *StdioMCPDriver) handleLine(ctx context.Context, line string) {
	// Debug logging
	fmt.Fprintf(os.Stderr, "[STDIO-MCP] Received: %s\n", line)

	// Parse MCP request
	var mcpReq MCPRequest
	if err := json.Unmarshal([]byte(line), &mcpReq); err != nil {
		fmt.Fprintf(os.Stderr, "[STDIO-MCP] Parse error: %v\n", err)
		d.sendError("", -32700, "Parse error", err.Error())
		return
	}

	fmt.Fprintf(os.Stderr, "[STDIO-MCP] Method: %s, ID: %s\n", mcpReq.Method, mcpReq.ID)

	// Prepare authentication payload
	// For stdio, we support authentication via environment variables
	authPayload := &types.AuthPayload{
		Headers: make(map[string]string),
		Body:    []byte(line),
	}

	// Check for principal in environment
	if principal := os.Getenv("WAZEOS_PRINCIPAL"); principal != "" {
		authPayload.Headers["X-Principal"] = principal
	}

	// Use handler to process request
	resp, err := d.handler.HandleRequest(ctx, &mcpReq, authPayload)
	if err != nil {
		// err is an MCPError
		if mcpErr, ok := err.(*MCPError); ok {
			d.sendError(mcpReq.ID, mcpErr.Code, mcpErr.Message, mcpErr.Data)
		} else {
			d.sendError(mcpReq.ID, -32000, "Internal error", err.Error())
		}
		return
	}

	// Send success response (skip for notifications which return nil)
	if resp != nil {
		d.sendResponse(resp)
	}
}

// sendResponse sends an MCP response to stdout.
func (d *StdioMCPDriver) sendResponse(resp *MCPResponse) {
	respJSON, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[STDIO-MCP] Failed to marshal response: %v\n", err)
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	fmt.Fprintf(os.Stderr, "[STDIO-MCP] Sending: %s\n", string(respJSON))
	fmt.Fprintf(d.writer, "%s\n", string(respJSON))
}

// sendError sends an MCP error response to stdout.
func (d *StdioMCPDriver) sendError(id interface{}, code int, message string, data interface{}) {
	resp := &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}

	d.sendResponse(resp)
}

// SendNotification sends an MCP notification to the client (no response expected).
// This is used to notify clients about server-side changes like tool list updates.
func (d *StdioMCPDriver) SendNotification(method string, params map[string]interface{}) error {
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
	}

	if params != nil {
		notification["params"] = params
	}

	notifJSON, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	fmt.Fprintf(os.Stderr, "[STDIO-MCP] Sending notification: %s\n", string(notifJSON))
	fmt.Fprintf(d.writer, "%s\n", string(notifJSON))

	return nil
}

// NotifyToolsChanged sends a notification that the tools list has changed.
// This should be called when apps are installed or uninstalled.
func (d *StdioMCPDriver) NotifyToolsChanged() error {
	return d.SendNotification("notifications/tools/list_changed", nil)
}

// OnPackageChanged implements the PackageChangeListener interface.
// Called when packages are installed or uninstalled.
func (d *StdioMCPDriver) OnPackageChanged() {
	fmt.Fprintf(os.Stderr, "[STDIO-MCP] Package changed, notifying client\n")
	if err := d.NotifyToolsChanged(); err != nil {
		fmt.Fprintf(os.Stderr, "[STDIO-MCP] Failed to send tool change notification: %v\n", err)
	}
}
