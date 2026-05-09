package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/wazeos/wazeos/core/internal/pkg"
	"github.com/wazeos/wazeos/core/kernel"
	"github.com/wazeos/wazeos/core/kernel/iobus"
)

// mcpCmd is the parent command for MCP-related operations
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP (Model Context Protocol) integration commands",
	Long: `Commands for integrating WazeOS with MCP clients like Claude Desktop.

Available subcommands:
  server   - Run the MCP server
  install  - Install WazeOS into Claude Desktop configuration
`,
}

// mcpServerCmd runs the MCP server
var mcpServerCmd = &cobra.Command{
	Use:   "server",
	Short: "Run MCP server to expose installed tools to Claude Desktop",
	Long: `Starts an MCP (Model Context Protocol) server that exposes all installed
WazeOS apps as MCP tools. This allows Claude Desktop and other MCP clients
to discover and invoke your WASM-based tools.

The server implements the MCP stdio protocol and reads/writes JSON-RPC 2.0
messages over stdin/stdout.

Example usage in Claude Desktop config (~/.config/claude/mcp_servers.json):
  {
    "wazeos": {
      "command": "wazeos",
      "args": ["mcp", "server"]
    }
  }
`,
	RunE: runMCPServer,
}

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.AddCommand(mcpServerCmd)

	// Add backward compatibility alias for old command
	mcpServerAliasCmd := &cobra.Command{
		Use:    "mcp-server",
		Short:  "Run MCP server (deprecated: use 'mcp server' instead)",
		Hidden: true, // Hide from help but keep functional
		RunE:   runMCPServer,
	}
	rootCmd.AddCommand(mcpServerAliasCmd)
}

// ============================================================================
// MCP Protocol Types
// ============================================================================

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCPInitializeParams represents MCP initialize request params
type MCPInitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      ClientInfo             `json:"clientInfo"`
}

// ClientInfo represents MCP client information
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MCPInitializeResult represents MCP initialize response
type MCPInitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
}

// Capabilities represents MCP server capabilities
type Capabilities struct {
	Tools map[string]interface{} `json:"tools,omitempty"`
}

// ServerInfo represents MCP server information
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MCPTool represents an MCP tool definition
type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// MCPToolsListResult represents the response to tools/list
type MCPToolsListResult struct {
	Tools []MCPTool `json:"tools"`
}

// MCPToolCallParams represents the params for tools/call
type MCPToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// MCPToolCallResult represents the response to tools/call
type MCPToolCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents an MCP content block
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ============================================================================
// MCP Server Implementation
// ============================================================================

// MCPServer holds the MCP server state
type MCPServer struct {
	apps      map[string]*InstalledApp
	bus       *iobus.IOBus
	ctx       context.Context
	scanner   *bufio.Scanner
	logFile   *os.File
}

// InstalledApp represents an installed WazeOS app
type InstalledApp struct {
	Manifest *pkg.Manifest
	Path     string
	WASMPath string
}

func runMCPServer(cmd *cobra.Command, args []string) error {
	// Open log file for debugging (MCP uses stdio for protocol)
	logFile, err := os.OpenFile(filepath.Join(os.TempDir(), "wazeos-mcp.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer logFile.Close()

	// Initialize the kernel and IO Bus
	if err := kernel.InitDrivers(); err != nil {
		return fmt.Errorf("failed to initialize drivers: %w", err)
	}

	bus := iobus.GetDefaultBus()

	// Load all installed apps
	apps, err := loadInstalledApps()
	if err != nil {
		return fmt.Errorf("failed to load installed apps: %w", err)
	}

	server := &MCPServer{
		apps:    apps,
		bus:     bus,
		ctx:     context.Background(),
		scanner: bufio.NewScanner(os.Stdin),
		logFile: logFile,
	}

	server.log("MCP server started with %d apps", len(apps))

	// Process JSON-RPC requests from stdin
	for server.scanner.Scan() {
		line := server.scanner.Text()
		if line == "" {
			continue
		}

		server.log("Received: %s", line)

		// Parse JSON-RPC request
		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			server.sendError(nil, -32700, "Parse error", nil)
			continue
		}

		// Handle request
		server.handleRequest(&req)
	}

	if err := server.scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}

// handleRequest routes JSON-RPC requests to appropriate handlers
func (s *MCPServer) handleRequest(req *JSONRPCRequest) {
	s.log("Handling method: %s", req.Method)

	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "tools/list":
		s.handleToolsList(req)
	case "tools/call":
		s.handleToolsCall(req)
	case "notifications/initialized":
		// Client acknowledges initialization - no response needed
		return
	default:
		s.sendError(req.ID, -32601, "Method not found", nil)
	}
}

// handleInitialize handles the MCP initialize request
func (s *MCPServer) handleInitialize(req *JSONRPCRequest) {
	result := MCPInitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: Capabilities{
			Tools: map[string]interface{}{},
		},
		ServerInfo: ServerInfo{
			Name:    "wazeos",
			Version: "2.0.0",
		},
	}

	s.sendResult(req.ID, result)
}

// handleToolsList handles the tools/list request
func (s *MCPServer) handleToolsList(req *JSONRPCRequest) {
	tools := make([]MCPTool, 0, len(s.apps))

	for _, app := range s.apps {
		if app.Manifest.Tool == nil {
			continue
		}

		tools = append(tools, MCPTool{
			Name:        app.Manifest.Tool.Name,
			Description: app.Manifest.Tool.Description,
			InputSchema: app.Manifest.Tool.InputSchema,
		})
	}

	result := MCPToolsListResult{
		Tools: tools,
	}

	s.sendResult(req.ID, result)
}

// handleToolsCall handles the tools/call request
func (s *MCPServer) handleToolsCall(req *JSONRPCRequest) {
	// Parse params
	var params MCPToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, -32602, "Invalid params", err.Error())
		return
	}

	s.log("Calling tool: %s with args: %v", params.Name, params.Arguments)

	// Find the app
	app, ok := s.apps[params.Name]
	if !ok {
		s.sendError(req.ID, -32602, "Tool not found", params.Name)
		return
	}

	// Execute the WASM app
	result, err := s.executeWASMApp(app, params.Arguments)
	if err != nil {
		s.log("Tool execution error: %v", err)
		s.sendResult(req.ID, MCPToolCallResult{
			Content: []ContentBlock{
				{
					Type: "text",
					Text: fmt.Sprintf("Error: %v", err),
				},
			},
			IsError: true,
		})
		return
	}

	// Return result as text content
	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	s.sendResult(req.ID, MCPToolCallResult{
		Content: []ContentBlock{
			{
				Type: "text",
				Text: string(resultJSON),
			},
		},
	})
}

// executeWASMApp loads and executes a WASM app
func (s *MCPServer) executeWASMApp(app *InstalledApp, args map[string]interface{}) (interface{}, error) {
	// Load WASM binary
	wasmBytes, err := os.ReadFile(app.WASMPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read WASM binary: %w", err)
	}

	// Create context with permissions from manifest
	permissions := make([]iobus.PermissionEntry, 0)

	// Add file permissions
	for _, pattern := range app.Manifest.Permissions.File {
		permissions = append(permissions, iobus.PermissionEntry{
			URIPattern:  "file://" + pattern,
			Permissions: []string{"call"},
		})
	}

	// Add HTTP permissions
	for _, pattern := range app.Manifest.Permissions.HTTP {
		permissions = append(permissions, iobus.PermissionEntry{
			URIPattern:  "http://" + pattern,
			Permissions: []string{"call"},
		})
	}

	// Add shell permissions
	for _, pattern := range app.Manifest.Permissions.Shell {
		permissions = append(permissions, iobus.PermissionEntry{
			URIPattern:  "shell://" + pattern,
			Permissions: []string{"call"},
		})
	}

	// Add permission to create WASM handles
	permissions = append(permissions, iobus.PermissionEntry{
		URIPattern:  "wasm://**",
		Permissions: []string{"call", "handle"},
	})

	wazeosCtx := iobus.NewContext(
		s.ctx,
		fmt.Sprintf("app:%s", app.Manifest.Package.Name),
		"invoke",
		"invoke",
		permissions,
		s.bus,
	)

	// Create WASM handle
	createReq := iobus.Request{
		URI:       "wasm://load",
		Operation: iobus.OpCreateHandle,
		Args: map[string]any{
			"binary": wasmBytes,
		},
	}

	createResp, err := s.bus.Call(wazeosCtx, createReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create WASM handle: %w", err)
	}

	if createResp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to create WASM handle: %s", createResp.Error)
	}

	handleID := string(createResp.Body)
	defer func() {
		// Clean up handle
		closeReq := iobus.Request{
			URI:       handleID,
			Operation: iobus.OpCloseHandle,
		}
		s.bus.Call(wazeosCtx, closeReq)
	}()

	// Prepare args JSON
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal args: %w", err)
	}

	// Call wazeos_tool_invoke
	callReq := iobus.Request{
		URI:       handleID,
		Operation: iobus.OpCall,
		Args: map[string]any{
			"function": "wazeos_tool_invoke",
			"args":     string(argsJSON),
		},
	}

	callResp, err := s.bus.Call(wazeosCtx, callReq)
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	// Parse the result
	var toolResult map[string]interface{}
	if err := json.Unmarshal(callResp.Body, &toolResult); err != nil {
		return nil, fmt.Errorf("failed to parse tool result: %w", err)
	}

	// Check if the tool returned an error
	if success, ok := toolResult["success"].(bool); ok && !success {
		if errMsg, ok := toolResult["error"].(string); ok {
			return nil, fmt.Errorf("tool error: %s", errMsg)
		}
		return nil, fmt.Errorf("tool execution failed")
	}

	// Return the result field
	if result, ok := toolResult["result"]; ok {
		return result, nil
	}

	return toolResult, nil
}

// ============================================================================
// Response Helpers
// ============================================================================

// sendResult sends a JSON-RPC success response
func (s *MCPServer) sendResult(id interface{}, result interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		s.log("Failed to marshal response: %v", err)
		return
	}

	s.log("Sending: %s", string(data))
	fmt.Println(string(data))
}

// sendError sends a JSON-RPC error response
func (s *MCPServer) sendError(id interface{}, code int, message string, data interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}

	jsonData, err := json.Marshal(resp)
	if err != nil {
		s.log("Failed to marshal error response: %v", err)
		return
	}

	s.log("Sending error: %s", string(jsonData))
	fmt.Println(string(jsonData))
}

// log writes to the debug log file
func (s *MCPServer) log(format string, args ...interface{}) {
	if s.logFile != nil {
		fmt.Fprintf(s.logFile, "[MCP] "+format+"\n", args...)
	}
}

// ============================================================================
// App Loading
// ============================================================================

// loadInstalledApps loads all installed apps from ~/.wazeos/apps/
func loadInstalledApps() (map[string]*InstalledApp, error) {
	appsDir := filepath.Join(os.Getenv("HOME"), ".wazeos", "apps")

	// Check if apps directory exists
	if _, err := os.Stat(appsDir); os.IsNotExist(err) {
		return make(map[string]*InstalledApp), nil
	}

	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read apps directory: %w", err)
	}

	apps := make(map[string]*InstalledApp)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		appPath := filepath.Join(appsDir, entry.Name())
		manifestPath := filepath.Join(appPath, "wazeos.toml")

		// Load manifest
		manifest, err := pkg.LoadManifest(manifestPath)
		if err != nil {
			continue // Skip invalid apps
		}

		// Only load apps (not drivers)
		if !manifest.IsApp() {
			continue
		}

		// Find WASM binary (uses package name, not Cargo naming)
		wasmPath := filepath.Join(appPath, manifest.Package.Name+".wasm")

		// Check if WASM file exists
		if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
			continue // Skip if no WASM binary
		}

		apps[manifest.Tool.Name] = &InstalledApp{
			Manifest: manifest,
			Path:     appPath,
			WASMPath: wasmPath,
		}
	}

	return apps, nil
}
