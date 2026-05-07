package request

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/wazeos/wazeos/internal/conversion"
	"github.com/wazeos/wazeos/internal/types"
	"github.com/wazeos/wazeos/internal/validation"
)

// Error implements the error interface for MCPError.
func (e *MCPError) Error() string {
	if e.Data != nil {
		return fmt.Sprintf("MCP error %d: %s (data: %v)", e.Code, e.Message, e.Data)
	}
	return fmt.Sprintf("MCP error %d: %s", e.Code, e.Message)
}

// MCPHandler provides transport-agnostic MCP JSON-RPC request handling.
// Both HTTP and stdio transports use this shared handler for core MCP logic.
type MCPHandler struct {
	invoker        types.InvocationHandler
	packageManager types.PackageManager
	authn          []types.SecurityAuthn
	authz          types.SecurityAuthz
}

// NewMCPHandler creates a new MCP handler.
func NewMCPHandler(
	authn []types.SecurityAuthn,
	authz types.SecurityAuthz,
	pkgMgr types.PackageManager,
) *MCPHandler {
	return &MCPHandler{
		authn:          authn,
		authz:          authz,
		packageManager: pkgMgr,
	}
}

// SetInvoker sets the invocation handler.
func (h *MCPHandler) SetInvoker(invoker types.InvocationHandler) {
	h.invoker = invoker
}

// HandleRequest processes an MCP JSON-RPC request and returns a response.
// This is the main entry point for both HTTP and stdio transports.
func (h *MCPHandler) HandleRequest(
	ctx context.Context,
	mcpReq *MCPRequest,
	authPayload *types.AuthPayload,
) (*MCPResponse, error) {
	// Validate JSON-RPC version
	if mcpReq.JSONRPC != "2.0" {
		return nil, &MCPError{
			Code:    -32600,
			Message: "Invalid Request: jsonrpc must be '2.0'",
		}
	}

	// Handle initialize method without authentication (it's part of the handshake)
	if mcpReq.Method == "initialize" {
		return h.handleInitialize(ctx, mcpReq)
	}

	// Handle initialized notification (no response needed)
	if mcpReq.Method == "notifications/initialized" || mcpReq.Method == "initialized" {
		// This is a notification, no response needed
		return nil, nil
	}

	// Authenticate request (required for all other methods)
	principal, err := h.authenticate(ctx, authPayload)
	if err != nil {
		if types.IsAbstain(err) {
			return nil, &MCPError{
				Code:    -32001,
				Message: "Authentication required",
			}
		}
		return nil, &MCPError{
			Code:    -32002,
			Message: "Authentication failed",
			Data:    err.Error(),
		}
	}

	// Get permissions
	var permissions *types.PermissionContext
	if h.authz != nil {
		permissions, err = h.authz.GetPermissions(ctx, principal)
		if err != nil {
			return nil, &MCPError{
				Code:    -32003,
				Message: "Authorization failed",
				Data:    err.Error(),
			}
		}
	} else {
		// No authz driver - grant no permissions
		permissions = types.NewPermissionContext(nil)
	}

	// Route based on method
	switch mcpReq.Method {
	case "tools/call":
		return h.handleToolsCall(ctx, mcpReq, principal, permissions)
	case "tools/list":
		return h.handleToolsList(ctx, mcpReq, principal, permissions)
	default:
		return nil, &MCPError{
			Code:    -32601,
			Message: fmt.Sprintf("Method not found: %s", mcpReq.Method),
		}
	}
}

// handleInitialize handles the initialize method for MCP protocol handshake.
func (h *MCPHandler) handleInitialize(
	ctx context.Context,
	mcpReq *MCPRequest,
) (*MCPResponse, error) {
	// Extract protocol version from params
	protocolVersion, ok := mcpReq.Params["protocolVersion"].(string)
	if !ok {
		protocolVersion = "2024-11-05" // Default to current version
	}

	// Return initialize response
	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      mcpReq.ID,
		Result: map[string]interface{}{
			"protocolVersion": protocolVersion,
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "wazeos",
				"version": "1.0.0",
			},
		},
	}, nil
}

// handleToolsCall handles the tools/call method.
func (h *MCPHandler) handleToolsCall(
	ctx context.Context,
	mcpReq *MCPRequest,
	principal string,
	permissions *types.PermissionContext,
) (*MCPResponse, error) {
	// Extract tool name
	name, ok := mcpReq.Params["name"].(string)
	if !ok || name == "" {
		return nil, &MCPError{
			Code:    -32602,
			Message: "Invalid params: 'name' required",
		}
	}

	// Get app metadata and schema for validation
	var args []string
	if h.packageManager != nil {
		// Resolve app name to appID
		appID, err := h.packageManager.Resolve(ctx, name)
		if err != nil {
			return nil, &MCPError{
				Code:    -32004,
				Message: fmt.Sprintf("App not found: %s", name),
			}
		}

		// Get metadata (includes schema)
		metadata, err := h.packageManager.Get(ctx, appID)
		if err != nil {
			return nil, &MCPError{
				Code:    -32004,
				Message: fmt.Sprintf("Failed to get app metadata: %s", name),
				Data:    err.Error(),
			}
		}

		// Extract and validate arguments if present
		if argsMap, ok := mcpReq.Params["arguments"].(map[string]interface{}); ok {
			// If schema is present, validate arguments
			if metadata.InputSchema != nil {
				// Parse schema
				var schemaMap map[string]interface{}
				if err := json.Unmarshal([]byte(*metadata.InputSchema), &schemaMap); err != nil {
					return nil, &MCPError{
						Code:    -32602,
						Message: "Invalid schema format",
						Data:    err.Error(),
					}
				}

				// Validate arguments against schema
				validator, err := validation.NewSchemaValidator([]byte(*metadata.InputSchema))
				if err != nil {
					return nil, &MCPError{
						Code:    -32602,
						Message: "Failed to create validator",
						Data:    err.Error(),
					}
				}

				if err := validator.Validate(argsMap); err != nil {
					return nil, &MCPError{
						Code:    -32602,
						Message: fmt.Sprintf("Invalid arguments: %v", err),
					}
				}

				// Extract CLI config from schema
				cliConfig := conversion.ExtractCLIConfig(schemaMap)

				// Convert to CLI arguments
				args, err = conversion.ConvertToCLIArgs(argsMap, cliConfig)
				if err != nil {
					return nil, &MCPError{
						Code:    -32602,
						Message: "Failed to convert arguments",
						Data:    err.Error(),
					}
				}
			} else {
				// No schema - use default conversion (backward compatibility)
				cliConfig := conversion.DefaultCLIConfig()
				args, err = conversion.ConvertToCLIArgs(argsMap, cliConfig)
				if err != nil {
					return nil, &MCPError{
						Code:    -32602,
						Message: "Failed to convert arguments",
						Data:    err.Error(),
					}
				}
			}
		}
	} else {
		// No package manager - fall back to naive conversion (backward compatibility)
		if argsMap, ok := mcpReq.Params["arguments"].(map[string]interface{}); ok {
			for key, value := range argsMap {
				argJSON, _ := json.Marshal(value)
				args = append(args, fmt.Sprintf("%s=%s", key, string(argJSON)))
			}
		}
	}

	// Create execution context
	traceID := uuid.New().String()
	// Convert ID to string (JSON-RPC allows string, number, or null)
	requestID := fmt.Sprintf("%v", mcpReq.ID)
	execCtx := types.NewExecutionContext(requestID, traceID, principal, permissions)

	// Create invocation request
	invReq := &types.InvocationRequest{
		Context: execCtx,
		AppID:   name,
		Args:    args,
	}

	// Invoke the app
	if h.invoker == nil {
		return nil, &MCPError{
			Code:    -32000,
			Message: "Internal error: invoker not set",
		}
	}

	result, err := h.invoker.Invoke(ctx, invReq)
	if err != nil {
		// Check error type for appropriate error code
		var code int
		var message string
		if types.IsNotFound(err) {
			code = -32004
			message = fmt.Sprintf("App not found: %s", name)
		} else if types.IsPermissionDenied(err) {
			code = -32005
			message = "Permission denied"
		} else if types.IsTimeout(err) {
			code = -32006
			message = "Execution timeout"
		} else {
			code = -32000
			message = "Internal error"
		}
		return nil, &MCPError{
			Code:    code,
			Message: message,
			Data:    err.Error(),
		}
	}

	// Format response
	toolResult := MCPToolCallResult{
		Content: []MCPContent{},
		IsError: result.ExitCode != 0,
	}

	// Add stdout as text content
	if len(result.Stdout) > 0 {
		toolResult.Content = append(toolResult.Content, MCPContent{
			Type: "text",
			Text: string(result.Stdout),
		})
	}

	// Add stderr if present
	if len(result.Stderr) > 0 {
		toolResult.Content = append(toolResult.Content, MCPContent{
			Type: "text",
			Text: fmt.Sprintf("stderr: %s", string(result.Stderr)),
		})
	}

	// If no output, provide default message
	if len(toolResult.Content) == 0 {
		toolResult.Content = append(toolResult.Content, MCPContent{
			Type: "text",
			Text: fmt.Sprintf("Execution completed with exit code %d", result.ExitCode),
		})
	}

	// Return success response
	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      mcpReq.ID,
		Result:  toolResult,
	}, nil
}

// handleToolsList handles the tools/list method.
// Returns a list of all available tools with their schemas.
func (h *MCPHandler) handleToolsList(
	ctx context.Context,
	mcpReq *MCPRequest,
	principal string,
	permissions *types.PermissionContext,
) (*MCPResponse, error) {
	// List all installed apps
	var apps []*types.AppMetadata
	var err error

	if h.packageManager != nil {
		apps, err = h.packageManager.List(ctx)
		if err != nil {
			return nil, &MCPError{
				Code:    -32000,
				Message: "Failed to list tools",
				Data:    err.Error(),
			}
		}
	}

	// Convert to MCP tools format
	tools := make([]map[string]interface{}, 0, len(apps))
	for _, app := range apps {
		tool := map[string]interface{}{
			"name":        app.AppID(),
			"description": app.Description,
		}

		// Include schema - MCP requires inputSchema to always be present
		if app.InputSchema != nil {
			var schema map[string]interface{}
			if err := json.Unmarshal([]byte(*app.InputSchema), &schema); err == nil {
				tool["inputSchema"] = schema
			} else {
				// Failed to parse schema, use empty object
				tool["inputSchema"] = map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{},
				}
			}
		} else {
			// No schema defined, use empty object
			tool["inputSchema"] = map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{},
			}
		}

		tools = append(tools, tool)
	}

	// Return response
	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      mcpReq.ID,
		Result: map[string]interface{}{
			"tools": tools,
		},
	}, nil
}

// authenticate tries all authn drivers until one succeeds.
func (h *MCPHandler) authenticate(ctx context.Context, payload *types.AuthPayload) (string, error) {
	if len(h.authn) == 0 {
		// No authn drivers - deny by default
		return "", types.ErrAbstain
	}

	for _, driver := range h.authn {
		principal, err := driver.Authenticate(ctx, payload)
		if err == nil {
			return principal, nil
		}
		if !types.IsAbstain(err) {
			// Driver recognized credentials but rejected them
			return "", err
		}
		// Driver abstained, try next one
	}

	// All drivers abstained
	return "", types.ErrAbstain
}
