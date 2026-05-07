package request

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/wazeos/wazeos/internal/types"
)

// MCPRequest represents an MCP JSON-RPC 2.0 request.
type MCPRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id"` // Can be string, number, or null
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params"`
}

// MCPResponse represents an MCP JSON-RPC 2.0 response.
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"` // Can be string, number, or null
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP error.
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCPToolCallResult represents the result of a tool call.
type MCPToolCallResult struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError"`
}

// MCPContent represents a content block in the response.
type MCPContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// HTTPMCPDriver implements MCP transport over HTTP.
type HTTPMCPDriver struct {
	mu         sync.RWMutex
	addr       string
	listener   net.Listener
	server     *http.Server
	handler    *MCPHandler
	started    bool
	actualAddr string // Actual bound address (for testing with :0)
}

// NewHTTPMCPDriver creates a new HTTP MCP transport driver.
func NewHTTPMCPDriver(addr string, authn []types.SecurityAuthn, authz types.SecurityAuthz, pkgMgr types.PackageManager) *HTTPMCPDriver {
	if addr == "" {
		addr = ":8080"
	}

	return &HTTPMCPDriver{
		addr:    addr,
		handler: NewMCPHandler(authn, authz, pkgMgr),
	}
}

// Name returns the driver name.
func (d *HTTPMCPDriver) Name() string {
	return "wazeos/http"
}

// Patterns returns URI patterns this driver handles.
func (d *HTTPMCPDriver) Patterns() []string {
	return []string{"http://*", "https://*"}
}

// SetInvoker provides the callback to dispatch invocations.
func (d *HTTPMCPDriver) SetInvoker(invoker types.InvocationHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handler.SetInvoker(invoker)
}

// Start begins listening for inbound requests.
func (d *HTTPMCPDriver) Start(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.started {
		return fmt.Errorf("driver already started")
	}

	if d.handler.invoker == nil {
		return fmt.Errorf("invoker not set")
	}

	// Create listener
	listener, err := net.Listen("tcp", d.addr)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	d.listener = listener
	d.actualAddr = listener.Addr().String()

	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", d.handleMCP)
	mux.HandleFunc("/health", d.handleHealth)

	d.server = &http.Server{
		Handler: mux,
	}

	// Start server in background
	go func() {
		if err := d.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			// Log error but don't crash
			fmt.Printf("HTTP server error: %v\n", err)
		}
	}()

	d.started = true
	return nil
}

// Stop gracefully shuts down the driver.
func (d *HTTPMCPDriver) Stop(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.started {
		return nil
	}

	if d.server != nil {
		if err := d.server.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown server: %w", err)
		}
	}

	if d.listener != nil {
		d.listener.Close()
	}

	d.started = false
	return nil
}

// Addr returns the actual bound address (useful for testing with port :0).
func (d *HTTPMCPDriver) Addr() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.actualAddr
}

// handleHealth handles health check requests.
func (d *HTTPMCPDriver) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleMCP handles MCP tool call requests.
func (d *HTTPMCPDriver) handleMCP(w http.ResponseWriter, r *http.Request) {
	// Only accept POST
	if r.Method != http.MethodPost {
		d.sendError(w, "method-not-allowed", -32601, "Method not allowed", nil)
		return
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		d.sendError(w, "", -32700, "Failed to read request body", nil)
		return
	}
	defer r.Body.Close()

	// Parse MCP request
	var mcpReq MCPRequest
	if err := json.Unmarshal(body, &mcpReq); err != nil {
		d.sendError(w, "", -32700, "Parse error", nil)
		return
	}

	// Prepare authentication payload
	authPayload := &types.AuthPayload{
		Headers: make(map[string]string),
		Body:    body,
	}
	for key, values := range r.Header {
		if len(values) > 0 {
			authPayload.Headers[key] = values[0]
		}
	}

	// Use handler to process request
	resp, err := d.handler.HandleRequest(r.Context(), &mcpReq, authPayload)
	if err != nil {
		// err is an MCPError
		if mcpErr, ok := err.(*MCPError); ok {
			d.sendError(w, mcpReq.ID, mcpErr.Code, mcpErr.Message, mcpErr.Data)
		} else {
			d.sendError(w, mcpReq.ID, -32000, "Internal error", err.Error())
		}
		return
	}

	// Send success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// sendError sends an MCP error response.
func (d *HTTPMCPDriver) sendError(w http.ResponseWriter, id interface{}, code int, message string, data interface{}) {
	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}

	w.Header().Set("Content-Type", "application/json")

	// Map error codes to HTTP status codes
	statusCode := http.StatusOK // JSON-RPC errors are 200 OK
	if code == -32001 || code == -32002 {
		statusCode = http.StatusUnauthorized
	} else if code == -32003 || code == -32005 {
		statusCode = http.StatusForbidden
	} else if code == -32004 {
		statusCode = http.StatusNotFound
	}

	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(resp)
}
