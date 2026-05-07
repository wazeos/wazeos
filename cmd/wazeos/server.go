package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/wazeos/wazeos/internal/api"
	"github.com/wazeos/wazeos/internal/drivers/io/bus"
	"github.com/wazeos/wazeos/internal/drivers/io/request"
	"github.com/wazeos/wazeos/internal/drivers/kernel/pkg"
	"github.com/wazeos/wazeos/internal/drivers/kernel/runtime"
	"github.com/wazeos/wazeos/internal/security"
	"github.com/wazeos/wazeos/internal/types"
)

var (
	serverAddr string
	serverMode string
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the WazeOS server",
	Long: `Start the WazeOS server in HTTP or stdio mode.

HTTP mode runs a REST API server for managing packages, secrets, and MCP tools.
stdio mode runs an MCP server that communicates over stdin/stdout.

Examples:
  # Start HTTP server (default)
  wazeos server --addr=:8081

  # Start stdio MCP server
  wazeos server --mode=stdio

  # Start with custom data directory
  wazeos server --data-path=/var/lib/wazeos --addr=:9090`,
	Run: runServer,
}

func init() {
	rootCmd.AddCommand(serverCmd)

	serverCmd.Flags().StringVar(&serverAddr, "addr", ":8081", "address to listen on (HTTP mode)")
	serverCmd.Flags().StringVar(&serverMode, "mode", "http", "transport mode: http or stdio")

	// Bind flags to viper
	viper.BindPFlag("server.addr", serverCmd.Flags().Lookup("addr"))
	viper.BindPFlag("server.mode", serverCmd.Flags().Lookup("mode"))
}

func runServer(cmd *cobra.Command, args []string) {
	// Get configuration
	packagesPath := viper.GetString("data_path")
	if packagesPath == "" {
		// Fallback to default
		home, err := os.UserHomeDir()
		if err != nil {
			packagesPath = "./data"
		} else {
			packagesPath = filepath.Join(home, ".wazeos", "data")
		}
	}

	// Make path absolute for clarity
	absDataPath, err := filepath.Abs(packagesPath)
	if err != nil {
		absDataPath = packagesPath
	}

	if !quiet {
		fmt.Fprintln(os.Stderr, "Starting WazeOS...")
		fmt.Fprintf(os.Stderr, "Data path: %s\n", absDataPath)
		fmt.Fprintln(os.Stderr, "")
	}

	ctx := context.Background()

	// Create secrets store driver
	secretsStore := NewInMemorySecretsStore()

	// Create io.bus.memory - stateful native Go I/O routing layer
	resourceBus := bus.NewMemoryIOBus(secretsStore)

	// Create package manager (needed for ExecDriver)
	pkgMgr, err := pkg.NewPackageManager(packagesPath, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create package manager: %v\n", err)
		os.Exit(1)
	}

	// Register ExecDriver for fn:// calls (MCP tool invocation)
	execDriver := runtime.NewExecDriver(pkgMgr, resourceBus, nil)
	if err := resourceBus.RegisterDriver(execDriver); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to register exec driver: %v\n", err)
		os.Exit(1)
	}

	// Create RuntimeExec for WASM drivers
	runtimeExec := runtime.NewRuntimeExec(0) // Use default timeout

	// Load and register installed WASM resource drivers
	wasmDrivers, err := runtime.LoadInstalledResourceDrivers(ctx, pkgMgr, runtimeExec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load installed drivers: %v\n", err)
		os.Exit(1)
	}
	for _, driver := range wasmDrivers {
		if err := resourceBus.RegisterDriver(driver); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to register driver %q: %v\n", driver.Name(), err)
			os.Exit(1)
		}
		if !quiet {
			fmt.Fprintf(os.Stderr, "✓ Registered driver: %s (patterns: %v)\n", driver.Name(), driver.Patterns())
		}
	}

	// Wrap with authz injection layer
	authzLayer := security.NewAuthzInjectionLayer(resourceBus)
	authzLayer.RegisterScheme("secret", "security.secrets")
	authzLayer.RegisterScheme("fn", "runtime.exec")

	// Create simple auth driver for testing
	authDrivers := []types.SecurityAuthn{&AllowAllAuth{}}

	// Create invocation handler
	invoker := NewSimpleInvocationHandler(authzLayer)

	// Get mode from viper (flag or config)
	mode := viper.GetString("server.mode")
	if serverMode != "" {
		mode = serverMode
	}

	// Switch based on transport mode
	switch mode {
	case "http":
		addr := viper.GetString("server.addr")
		if serverAddr != "" && serverAddr != ":8081" {
			addr = serverAddr
		}
		runHTTPMode(ctx, addr, pkgMgr, authDrivers, authzLayer)
	case "stdio":
		runStdioMode(ctx, pkgMgr, authDrivers, invoker)
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s (supported: http, stdio)\n", mode)
		os.Exit(1)
	}
}

func runHTTPMode(ctx context.Context, addr string, pkgMgr types.PackageManager, authDrivers []types.SecurityAuthn, authzLayer types.ResourceBus) {
	// Create management API
	managementAPI := api.NewManagementAPI(addr, pkgMgr, authDrivers, nil)

	// Set resource bus (with authz layer)
	managementAPI.SetResourceBus(authzLayer)

	// Start management API
	if err := managementAPI.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start management API: %v\n", err)
		os.Exit(1)
	}

	actualAddr := managementAPI.Addr()
	if !quiet {
		fmt.Printf("✓ Management API listening on http://%s\n", actualAddr)
		fmt.Printf("  • Secrets API: http://%s/api/secrets\n", actualAddr)
		fmt.Printf("  • Packages API: http://%s/api/v1/packages\n", actualAddr)
		fmt.Println()
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	if !quiet {
		fmt.Println("WazeOS running (HTTP mode). Press Ctrl+C to stop.")
	}
	<-sigChan

	if !quiet {
		fmt.Println("\nShutting down...")
	}
	if err := managementAPI.Stop(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Error stopping management API: %v\n", err)
	}

	if !quiet {
		fmt.Println("✓ WazeOS stopped")
	}
}

func runStdioMode(ctx context.Context, pkgMgr types.PackageManager, authDrivers []types.SecurityAuthn, execDriver types.InvocationHandler) {
	// Create stdio MCP driver
	stdioDriver := request.NewStdioMCPDriver(authDrivers, nil, pkgMgr)
	stdioDriver.SetInvoker(execDriver)

	// Register stdio driver as a package change listener
	// This allows the driver to notify MCP clients when tools are installed/uninstalled
	if concretePkgMgr, ok := pkgMgr.(*pkg.PackageManager); ok {
		concretePkgMgr.AddChangeListener(stdioDriver)
	}

	// Set up context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Handle interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "\nShutting down...")
		cancel()
	}()

	fmt.Fprintln(os.Stderr, "✓ MCP stdio server ready")
	fmt.Fprintln(os.Stderr, "Listening on stdin/stdout...")
	fmt.Fprintln(os.Stderr, "")

	// Start stdio driver (blocks until EOF or interrupt)
	if err := stdioDriver.Start(ctx); err != nil {
		if err != context.Canceled {
			fmt.Fprintf(os.Stderr, "stdio driver error: %v\n", err)
			os.Exit(1)
		}
	}

	// Stop driver
	if err := stdioDriver.Stop(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Error stopping stdio driver: %v\n", err)
	}

	fmt.Fprintln(os.Stderr, "✓ MCP stdio server stopped")
}

// InMemorySecretsStore is a simple in-memory secrets store for testing
type InMemorySecretsStore struct {
	secrets map[string]map[string]interface{}
}

func NewInMemorySecretsStore() *InMemorySecretsStore {
	return &InMemorySecretsStore{
		secrets: make(map[string]map[string]interface{}),
	}
}

func (s *InMemorySecretsStore) Name() string {
	return "wazeos/secrets"
}

func (s *InMemorySecretsStore) Patterns() []string {
	return []string{"secret://**"}
}

func (s *InMemorySecretsStore) HandleCall(ctx context.Context, call *types.ResourceCall) (*types.ResourceResult, error) {
	// Determine operation from permissions
	hasWrite := false
	hasRead := false
	hasDelete := false
	hasList := false

	for _, perm := range call.Permissions {
		switch strings.ToLower(perm) {
		case "write", "set", "put":
			hasWrite = true
		case "read", "get":
			hasRead = true
		case "delete":
			hasDelete = true
		case "list":
			hasList = true
		}
	}

	// Handle based on permissions (priority: write > delete > list > read)
	if hasWrite {
		var req map[string]interface{}
		if err := json.Unmarshal(call.Body, &req); err != nil {
			return &types.ResourceResult{StatusCode: 400, Body: []byte(`{"error":"invalid JSON"}`)}, nil
		}
		key := strings.TrimPrefix(call.URI, "secret:///")
		s.secrets[key] = req
		resp := map[string]interface{}{"key": key, "stored": true}
		respJSON, _ := json.Marshal(resp)
		return &types.ResourceResult{StatusCode: 200, Body: respJSON}, nil
	}

	if hasDelete {
		key := strings.TrimPrefix(call.URI, "secret:///")
		delete(s.secrets, key)
		resp := map[string]interface{}{"key": key, "deleted": true}
		respJSON, _ := json.Marshal(resp)
		return &types.ResourceResult{StatusCode: 200, Body: respJSON}, nil
	}

	if hasList {
		keys := make([]string, 0, len(s.secrets))
		for k := range s.secrets {
			keys = append(keys, k)
		}
		resp := map[string]interface{}{"keys": keys, "count": len(keys)}
		respJSON, _ := json.Marshal(resp)
		return &types.ResourceResult{StatusCode: 200, Body: respJSON}, nil
	}

	if hasRead {
		key := strings.TrimPrefix(call.URI, "secret:///")
		if secret, ok := s.secrets[key]; ok {
			resp := map[string]interface{}{"key": key, "value": secret["value"]}
			respJSON, _ := json.Marshal(resp)
			return &types.ResourceResult{StatusCode: 200, Body: respJSON}, nil
		}
		return &types.ResourceResult{StatusCode: 404, Body: []byte(`{"error":"not found"}`)}, nil
	}

	// Check for match permission (special case for pattern matching)
	for _, perm := range call.Permissions {
		if strings.ToLower(perm) == "match" {
			parsed, _ := url.Parse(call.URI)
			prefix := parsed.Query().Get("prefix")
			matches := make([]map[string]interface{}, 0)
			for k, v := range s.secrets {
				if prefix == "" || strings.HasPrefix(k, prefix) {
					matches = append(matches, map[string]interface{}{
						"key":   k,
						"value": v["value"],
					})
				}
			}
			resp := map[string]interface{}{"matches": matches, "count": len(matches)}
			respJSON, _ := json.Marshal(resp)
			return &types.ResourceResult{StatusCode: 200, Body: respJSON}, nil
		}
	}

	return &types.ResourceResult{StatusCode: 403, Body: []byte(`{"error":"no valid operation permission provided"}`)}, nil
}

// AllowAllAuth is a passthrough authentication driver for testing
type AllowAllAuth struct{}

func (a *AllowAllAuth) Name() string {
	return "wazeos/allow-all"
}

func (a *AllowAllAuth) Authenticate(ctx context.Context, payload *types.AuthPayload) (string, error) {
	return "test-user", nil
}

func (a *AllowAllAuth) ValidateToken(ctx context.Context, token string) (string, error) {
	return "test-user", nil
}

// SimpleInvocationHandler wraps a ResourceBus to provide InvocationHandler functionality
type SimpleInvocationHandler struct {
	resourceBus types.ResourceBus
}

func NewSimpleInvocationHandler(resourceBus types.ResourceBus) *SimpleInvocationHandler {
	return &SimpleInvocationHandler{resourceBus: resourceBus}
}

func (h *SimpleInvocationHandler) Invoke(ctx context.Context, req *types.InvocationRequest) (*types.InvocationResult, error) {
	// Convert invocation request to resource call using fn:// URI
	uri := fmt.Sprintf("fn://%s", req.AppID)

	// Convert args to JSON body
	argsJSON, err := json.Marshal(map[string]interface{}{
		"args": req.Args,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal args: %w", err)
	}

	resourceCall := &types.ResourceCall{
		URI:         uri,
		Body:        argsJSON,
		Context:     req.Context,
		Permissions: []string{"invoke"},
	}

	result, err := h.resourceBus.Call(ctx, resourceCall)
	if err != nil {
		return nil, err
	}

	// Parse result
	var invResult types.InvocationResult
	if err := json.Unmarshal(result.Body, &invResult); err != nil {
		// If unmarshal fails, create a simple result from the raw response
		invResult = types.InvocationResult{
			RequestID: req.Context.RequestID,
			Stdout:    result.Body,
			Stderr:    []byte{},
			ExitCode:  0,
		}
		if result.StatusCode >= 400 {
			invResult.ExitCode = 1
			invResult.Stderr = result.Body
			invResult.Stdout = []byte{}
		}
	}

	return &invResult, nil
}
