package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/wazeos/wazeos/internal/types"
)

// ManagementAPI provides REST endpoints for system administration.
type ManagementAPI struct {
	mu          sync.RWMutex
	addr        string
	listener    net.Listener
	server      *http.Server
	pkg         types.PackageManager
	authn       []types.SecurityAuthn
	authz       types.SecurityAuthz
	resourceBus types.ResourceBus
	started     bool
	actualAddr  string
}

// APIResponse represents a standard API response.
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
}

// APIError represents an API error.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// NewManagementAPI creates a new management API server.
func NewManagementAPI(addr string, pkg types.PackageManager, authn []types.SecurityAuthn, authz types.SecurityAuthz) *ManagementAPI {
	if addr == "" {
		addr = ":8081"
	}

	return &ManagementAPI{
		addr:  addr,
		pkg:   pkg,
		authn: authn,
		authz: authz,
	}
}

// SetResourceBus provides access to the resource bus for making resource calls.
func (api *ManagementAPI) SetResourceBus(bus types.ResourceBus) {
	api.mu.Lock()
	defer api.mu.Unlock()
	api.resourceBus = bus
}

// Start begins listening for API requests.
func (api *ManagementAPI) Start(ctx context.Context) error {
	api.mu.Lock()
	defer api.mu.Unlock()

	if api.started {
		return fmt.Errorf("API already started")
	}

	if api.pkg == nil {
		return fmt.Errorf("package manager not set")
	}

	// Create listener
	listener, err := net.Listen("tcp", api.addr)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	api.listener = listener
	api.actualAddr = listener.Addr().String()

	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", api.handleHealth)
	mux.HandleFunc("/api/apps", api.handleApps)
	mux.HandleFunc("/api/apps/", api.handleAppDetail)
	mux.HandleFunc("/api/secrets", api.handleSecrets)
	mux.HandleFunc("/api/secrets/", api.handleSecretDetail)
	mux.HandleFunc("/mcp", api.handleMCP)

	api.server = &http.Server{
		Handler: api.corsMiddleware(api.authMiddleware(mux)),
	}

	// Start server in background
	go func() {
		if err := api.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Management API error: %v\n", err)
		}
	}()

	api.started = true
	return nil
}

// Stop gracefully shuts down the API server.
func (api *ManagementAPI) Stop(ctx context.Context) error {
	api.mu.Lock()
	defer api.mu.Unlock()

	if !api.started {
		return nil
	}

	if api.server != nil {
		if err := api.server.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown server: %w", err)
		}
	}

	if api.listener != nil {
		api.listener.Close()
	}

	api.started = false
	return nil
}

// Addr returns the actual bound address.
func (api *ManagementAPI) Addr() string {
	api.mu.RLock()
	defer api.mu.RUnlock()
	return api.actualAddr
}

// corsMiddleware adds CORS headers.
func (api *ManagementAPI) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// authMiddleware handles authentication and authorization.
func (api *ManagementAPI) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health endpoint
		if r.URL.Path == "/api/health" {
			next.ServeHTTP(w, r)
			return
		}

		// Authenticate
		authPayload := &types.AuthPayload{
			Headers: make(map[string]string),
		}
		for key, values := range r.Header {
			if len(values) > 0 {
				authPayload.Headers[key] = values[0]
			}
		}

		principal, err := api.authenticate(r.Context(), authPayload)
		if err != nil {
			if types.IsAbstain(err) {
				api.sendError(w, http.StatusUnauthorized, "auth_required", "Authentication required")
			} else {
				api.sendError(w, http.StatusUnauthorized, "auth_failed", "Authentication failed")
			}
			return
		}

		// Store principal in context
		ctx := context.WithValue(r.Context(), "principal", principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// authenticate tries all authn drivers.
func (api *ManagementAPI) authenticate(ctx context.Context, payload *types.AuthPayload) (string, error) {
	if len(api.authn) == 0 {
		return "", types.ErrAbstain
	}

	for _, driver := range api.authn {
		principal, err := driver.Authenticate(ctx, payload)
		if err == nil {
			return principal, nil
		}
		if !types.IsAbstain(err) {
			return "", err
		}
	}

	return "", types.ErrAbstain
}

// handleHealth returns system health status.
func (api *ManagementAPI) handleHealth(w http.ResponseWriter, r *http.Request) {
	api.sendSuccess(w, map[string]string{"status": "ok"})
}

// handleApps handles app listing and installation.
func (api *ManagementAPI) handleApps(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		api.listApps(w, r)
	case http.MethodPost:
		api.installApp(w, r)
	default:
		api.sendError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	}
}

// handleAppDetail handles app details and deletion.
func (api *ManagementAPI) handleAppDetail(w http.ResponseWriter, r *http.Request) {
	// Extract appID from path /api/apps/{appID}
	appID := strings.TrimPrefix(r.URL.Path, "/api/apps/")
	if appID == "" {
		api.sendError(w, http.StatusBadRequest, "invalid_app_id", "App ID required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		api.getApp(w, r, appID)
	case http.MethodDelete:
		api.uninstallApp(w, r, appID)
	default:
		api.sendError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	}
}

// listApps returns all installed apps.
func (api *ManagementAPI) listApps(w http.ResponseWriter, r *http.Request) {
	apps, err := api.pkg.List(r.Context())
	if err != nil {
		api.sendError(w, http.StatusInternalServerError, "list_failed", "Failed to list apps")
		return
	}

	api.sendSuccess(w, map[string]interface{}{
		"apps": apps,
	})
}

// getApp returns details for a specific app.
func (api *ManagementAPI) getApp(w http.ResponseWriter, r *http.Request, appID string) {
	app, err := api.pkg.Get(r.Context(), appID)
	if err != nil {
		if types.IsNotFound(err) {
			api.sendError(w, http.StatusNotFound, "app_not_found", fmt.Sprintf("App not found: %s", appID))
		} else {
			api.sendError(w, http.StatusInternalServerError, "get_failed", "Failed to get app")
		}
		return
	}

	api.sendSuccess(w, map[string]interface{}{
		"app": app,
	})
}

// installApp installs a new app from uploaded ZIP.
func (api *ManagementAPI) installApp(w http.ResponseWriter, r *http.Request) {
	// Check content type
	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "multipart/form-data") && contentType != "application/zip" {
		api.sendError(w, http.StatusBadRequest, "invalid_content_type", "Content-Type must be multipart/form-data or application/zip")
		return
	}

	var zipData []byte
	var err error

	if contentType == "application/zip" {
		// Direct ZIP upload
		zipData, err = io.ReadAll(r.Body)
		if err != nil {
			api.sendError(w, http.StatusBadRequest, "read_failed", "Failed to read request body")
			return
		}
	} else {
		// Multipart form upload
		err := r.ParseMultipartForm(32 << 20) // 32MB max
		if err != nil {
			api.sendError(w, http.StatusBadRequest, "parse_failed", "Failed to parse multipart form")
			return
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			api.sendError(w, http.StatusBadRequest, "file_required", "File parameter required")
			return
		}
		defer file.Close()

		zipData, err = io.ReadAll(file)
		if err != nil {
			api.sendError(w, http.StatusBadRequest, "read_failed", "Failed to read uploaded file")
			return
		}
	}

	// Install app
	metadata, err := api.pkg.Install(r.Context(), zipData)
	if err != nil {
		if types.IsInvalidRequest(err) {
			api.sendError(w, http.StatusBadRequest, "invalid_package", err.Error())
		} else if types.IsAlreadyExists(err) {
			api.sendError(w, http.StatusConflict, "already_exists", err.Error())
		} else if types.IsDependencyNotFound(err) {
			api.sendError(w, http.StatusBadRequest, "dependency_not_found", err.Error())
		} else {
			api.sendError(w, http.StatusInternalServerError, "install_failed", "Failed to install app")
		}
		return
	}

	api.sendSuccess(w, map[string]interface{}{
		"app": metadata,
	})
}

// uninstallApp removes an installed app.
func (api *ManagementAPI) uninstallApp(w http.ResponseWriter, r *http.Request, appID string) {
	err := api.pkg.Uninstall(r.Context(), appID)
	if err != nil {
		if types.IsNotFound(err) {
			api.sendError(w, http.StatusNotFound, "app_not_found", fmt.Sprintf("App not found: %s", appID))
		} else {
			api.sendError(w, http.StatusInternalServerError, "uninstall_failed", err.Error())
		}
		return
	}

	api.sendSuccess(w, map[string]interface{}{
		"message": fmt.Sprintf("App %s uninstalled successfully", appID),
	})
}

// sendSuccess sends a successful API response.
func (api *ManagementAPI) sendSuccess(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(APIResponse{
		Success: true,
		Data:    data,
	})
}

// sendError sends an error API response.
func (api *ManagementAPI) sendError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(APIResponse{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
		},
	})
}

// handleSecrets handles secrets listing and creation.
func (api *ManagementAPI) handleSecrets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		api.listSecrets(w, r)
	case http.MethodPost:
		api.createSecret(w, r)
	default:
		api.sendError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	}
}

// handleSecretDetail handles secret deletion.
func (api *ManagementAPI) handleSecretDetail(w http.ResponseWriter, r *http.Request) {
	// Extract key from path /api/secrets/{key}
	key := strings.TrimPrefix(r.URL.Path, "/api/secrets/")
	if key == "" {
		api.sendError(w, http.StatusBadRequest, "invalid_key", "Secret key required")
		return
	}

	switch r.Method {
	case http.MethodDelete:
		api.deleteSecret(w, r, key)
	default:
		api.sendError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	}
}

// listSecrets returns all secret keys (without values).
func (api *ManagementAPI) listSecrets(w http.ResponseWriter, r *http.Request) {
	if api.resourceBus == nil {
		api.sendError(w, http.StatusServiceUnavailable, "resource_bus_unavailable", "Resource bus not configured")
		return
	}

	// Call secrets driver LIST method
	call := &types.ResourceCall{
		URI:         "secret:///",
		Permissions: []string{"list"},
	}

	result, err := api.resourceBus.Call(r.Context(), call)
	if err != nil {
		api.sendError(w, http.StatusInternalServerError, "list_failed", fmt.Sprintf("Failed to list secrets: %v", err))
		return
	}

	if result.StatusCode != 200 {
		api.sendError(w, result.StatusCode, "list_failed", string(result.Body))
		return
	}

	// Parse result
	var response map[string]interface{}
	if err := json.Unmarshal(result.Body, &response); err != nil {
		api.sendError(w, http.StatusInternalServerError, "parse_failed", "Failed to parse response")
		return
	}

	api.sendSuccess(w, response)
}

// createSecret stores a new credential.
func (api *ManagementAPI) createSecret(w http.ResponseWriter, r *http.Request) {
	if api.resourceBus == nil {
		api.sendError(w, http.StatusServiceUnavailable, "resource_bus_unavailable", "Resource bus not configured")
		return
	}

	// Parse request body
	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
		URL   string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.sendError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON request")
		return
	}

	if req.Key == "" {
		api.sendError(w, http.StatusBadRequest, "key_required", "Secret key is required")
		return
	}

	if req.Value == "" {
		api.sendError(w, http.StatusBadRequest, "value_required", "Secret value is required")
		return
	}

	// URL is optional - credentials are tied to drivers via hierarchical naming

	// Build request body for secrets driver
	secretBody := map[string]string{
		"value": req.Value,
	}
	// Only include URL if provided (for backward compatibility)
	if req.URL != "" {
		secretBody["url"] = req.URL
	}
	bodyJSON, _ := json.Marshal(secretBody)

	// Call secrets driver WRITE method
	call := &types.ResourceCall{
		URI:         fmt.Sprintf("secret:///%s", req.Key),
		Permissions: []string{"write"},
		Body:        bodyJSON,
	}

	result, err := api.resourceBus.Call(r.Context(), call)
	if err != nil {
		api.sendError(w, http.StatusInternalServerError, "create_failed", fmt.Sprintf("Failed to create secret: %v", err))
		return
	}

	if result.StatusCode != 200 {
		api.sendError(w, result.StatusCode, "create_failed", string(result.Body))
		return
	}

	// Parse result
	var response map[string]interface{}
	if err := json.Unmarshal(result.Body, &response); err != nil {
		api.sendError(w, http.StatusInternalServerError, "parse_failed", "Failed to parse response")
		return
	}

	api.sendSuccess(w, response)
}

// deleteSecret removes a credential.
func (api *ManagementAPI) deleteSecret(w http.ResponseWriter, r *http.Request, key string) {
	if api.resourceBus == nil {
		api.sendError(w, http.StatusServiceUnavailable, "resource_bus_unavailable", "Resource bus not configured")
		return
	}

	// Call secrets driver DELETE method
	call := &types.ResourceCall{
		URI:         fmt.Sprintf("secret:///%s", key),
		Permissions: []string{"delete"},
	}

	result, err := api.resourceBus.Call(r.Context(), call)
	if err != nil {
		api.sendError(w, http.StatusInternalServerError, "delete_failed", fmt.Sprintf("Failed to delete secret: %v", err))
		return
	}

	if result.StatusCode == 404 {
		api.sendError(w, http.StatusNotFound, "secret_not_found", fmt.Sprintf("Secret not found: %s", key))
		return
	}

	if result.StatusCode != 200 {
		api.sendError(w, result.StatusCode, "delete_failed", string(result.Body))
		return
	}

	// Parse result
	var response map[string]interface{}
	if err := json.Unmarshal(result.Body, &response); err != nil {
		api.sendError(w, http.StatusInternalServerError, "parse_failed", "Failed to parse response")
		return
	}

	api.sendSuccess(w, response)
}


// MCPRequest represents a JSON-RPC 2.0 request for MCP protocol
type MCPRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// MCPResponse represents a JSON-RPC 2.0 response
type MCPResponse struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id"`
	Result  map[string]interface{} `json:"result,omitempty"`
	Error   *MCPError              `json:"error,omitempty"`
}

// MCPError represents a JSON-RPC 2.0 error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// handleMCP handles MCP (Model Context Protocol) requests for tool invocation
func (api *ManagementAPI) handleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.sendMCPError(w, nil, -32600, "Invalid Request", "Only POST method is allowed")
		return
	}

	if api.resourceBus == nil {
		api.sendMCPError(w, nil, -32603, "Internal Error", "Resource bus not configured")
		return
	}

	// Parse JSON-RPC request
	var req MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.sendMCPError(w, nil, -32700, "Parse Error", "Invalid JSON")
		return
	}

	// Validate JSON-RPC version
	if req.JSONRPC != "2.0" {
		api.sendMCPError(w, req.ID, -32600, "Invalid Request", "JSON-RPC version must be 2.0")
		return
	}

	// Handle different MCP methods
	switch req.Method {
	case "tools/call":
		api.handleToolCall(w, r, &req)
	case "tools/list":
		api.handleToolsList(w, r, &req)
	default:
		api.sendMCPError(w, req.ID, -32601, "Method Not Found", fmt.Sprintf("Method not supported: %s", req.Method))
	}
}

// handleToolCall executes a tool via fn:// URI scheme
func (api *ManagementAPI) handleToolCall(w http.ResponseWriter, r *http.Request, req *MCPRequest) {
	// Extract tool name and arguments
	toolName, ok := req.Params["name"].(string)
	if !ok || toolName == "" {
		api.sendMCPError(w, req.ID, -32602, "Invalid Params", "Tool name is required")
		return
	}

	arguments, ok := req.Params["arguments"].(map[string]interface{})
	if !ok {
		arguments = make(map[string]interface{})
	}

	fmt.Printf("[MCP] Tool call: %s\n", toolName)
	fmt.Printf("[MCP] Arguments: %d parameters\n", len(arguments))

	// Convert arguments to JSON body
	argsJSON, err := json.Marshal(arguments)
	if err != nil {
		api.sendMCPError(w, req.ID, -32603, "Internal Error", "Failed to serialize arguments")
		return
	}

	// Create fn:// ResourceCall to route through kernel.iobus
	call := &types.ResourceCall{
		URI:         fmt.Sprintf("fn://%s", toolName),
		Permissions: []string{"invoke"},
		Body:        argsJSON,
		Headers:     make(map[string]string),
	}

	fmt.Printf("[MCP] Routing to: %s\n", call.URI)

	// Execute through resource bus (flows through authz → kernel.iobus → kernel.runtime.exec)
	result, err := api.resourceBus.Call(r.Context(), call)
	if err != nil {
		api.sendMCPError(w, req.ID, -32603, "Internal Error", fmt.Sprintf("Tool execution failed: %v", err))
		return
	}

	// Handle non-200 status codes
	if result.StatusCode != 200 {
		var errorMsg string
		var errorData map[string]interface{}
		if err := json.Unmarshal(result.Body, &errorData); err == nil {
			if msg, ok := errorData["error"].(string); ok {
				errorMsg = msg
			}
		}
		if errorMsg == "" {
			errorMsg = string(result.Body)
		}

		// Map HTTP status codes to JSON-RPC error codes
		code := -32603 // Internal error
		if result.StatusCode == 404 {
			code = -32602 // Invalid params (app not found)
		}

		api.sendMCPError(w, req.ID, code, "Tool Execution Failed", errorMsg)
		return
	}

	// Parse result
	var resultData map[string]interface{}
	if err := json.Unmarshal(result.Body, &resultData); err != nil {
		api.sendMCPError(w, req.ID, -32603, "Internal Error", "Failed to parse tool result")
		return
	}

	fmt.Printf("[MCP] ✓ Tool executed successfully\n")

	// Send JSON-RPC success response
	api.sendMCPSuccess(w, req.ID, resultData)
}

// handleToolsList returns list of available tools
func (api *ManagementAPI) handleToolsList(w http.ResponseWriter, r *http.Request, req *MCPRequest) {
	// Get list of installed apps
	apps, err := api.pkg.List(r.Context())
	if err != nil {
		api.sendMCPError(w, req.ID, -32603, "Internal Error", "Failed to list tools")
		return
	}

	// Convert apps to MCP tools format
	tools := make([]map[string]interface{}, 0, len(apps))
	for _, app := range apps {
		tool := map[string]interface{}{
			"name":        app.Name,
			"description": app.Description,
		}

		// TODO: Add inputSchema from app metadata when schema support is implemented
		// if app.Metadata.InputSchema != nil {
		//     tool["inputSchema"] = app.Metadata.InputSchema
		// }

		tools = append(tools, tool)
	}

	fmt.Printf("[MCP] Listed %d tools\n", len(tools))

	// Send JSON-RPC success response
	api.sendMCPSuccess(w, req.ID, map[string]interface{}{
		"tools": tools,
	})
}

// sendMCPSuccess sends a JSON-RPC success response
func (api *ManagementAPI) sendMCPSuccess(w http.ResponseWriter, id interface{}, result map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

// sendMCPError sends a JSON-RPC error response
func (api *ManagementAPI) sendMCPError(w http.ResponseWriter, id interface{}, code int, message, data string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // JSON-RPC errors still return 200 OK
	json.NewEncoder(w).Encode(MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	})
}
