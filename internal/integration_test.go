package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/wazeos/wazeos/internal/drivers/io/bus"
	"github.com/wazeos/wazeos/internal/drivers/io/request"
	"github.com/wazeos/wazeos/internal/drivers/kernel/runtime"
	"github.com/wazeos/wazeos/internal/drivers/kernel/security"
	"github.com/wazeos/wazeos/internal/kernel"
	"github.com/wazeos/wazeos/internal/types"
)

// TestEndToEnd_WASMDriver_FileRead tests the complete flow:
// MCP HTTP request → WASM app → WASM file driver → file system
func TestEndToEnd_WASMDriver_FileRead(t *testing.T) {
	ctx := context.Background()

	// Create test file
	testFile := "/tmp/wazeos-integration-test.txt"
	testContent := "Hello from WazeOS integration test!"
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(testFile)

	// 1. Set up kernel components
	runtimeExec := runtime.NewRuntimeExec(30 * time.Second)
	authz := security.NewAuthz()

	// Create kernel
	k := kernel.New()

	// Register authz
	if err := k.SetSecurityAuthz(authz); err != nil {
		t.Fatalf("Failed to set authz: %v", err)
	}

	// Register runtime
	if err := k.SetRuntimeExec(runtimeExec); err != nil {
		t.Fatalf("Failed to set runtime: %v", err)
	}

	// 2. Load WASM file driver
	fileDriverPath := "../../drivers/io.resource.file/app.wasm"
	fileDriverWASM, err := os.ReadFile(fileDriverPath)
	if err != nil {
		t.Fatalf("Failed to read file driver WASM: %v", err)
	}

	// Compile the file driver
	compiled, err := runtimeExec.CompileWASM(ctx, fileDriverWASM)
	if err != nil {
		t.Fatalf("Failed to compile file driver: %v", err)
	}

	// Create WASM driver wrapper
	fileDriver := runtime.NewWasmResourceDriver(
		"io.resource.file",
		[]string{"file://*/*"},
		runtimeExec,
		compiled,
	)

	// Register file driver with kernel
	if err := k.RegisterResourceDriver(fileDriver); err != nil {
		t.Fatalf("Failed to register file driver: %v", err)
	}

	// 3. Load file-reader app
	appPath := "../../apps/file-reader/app.wasm"
	appWASM, err := os.ReadFile(appPath)
	if err != nil {
		t.Fatalf("Failed to read file-reader app: %v", err)
	}

	appID := "test/file-reader_1.0.0"
	if err := runtimeExec.LoadApp(ctx, appID, appWASM); err != nil {
		t.Fatalf("Failed to load file-reader app: %v", err)
	}

	// 4. Create resource bus accessor for host functions
	// We need to manually wire up the resource bus to runtime for testing
	// In production, kernel.Start() does this
	resourceBus := bus.NewMemoryIOBus(nil)
	if err := resourceBus.RegisterDriver(fileDriver); err != nil {
		t.Fatalf("Failed to register driver with resource bus: %v", err)
	}
	runtimeExec.SetResourceBus(resourceBus)

	// 5. Register host functions manually (normally done by kernel.Start())
	// Register kernel.resource_call host function
	if err := runtimeExec.RegisterHostFunction("kernel", "resource_call", func(ctx context.Context, params []byte) ([]byte, error) {
		var call types.ResourceCall
		if err := json.Unmarshal(params, &call); err != nil {
			return nil, fmt.Errorf("failed to unmarshal resource call: %w", err)
		}

		result, err := resourceBus.Call(ctx, &call)
		if err != nil && result == nil {
			result = &types.ResourceResult{
				StatusCode: 500,
				Headers:    make(map[string]string),
				Body:       []byte(fmt.Sprintf("resource call failed: %v", err)),
				Error:      err.Error(),
			}
		}

		// Convert to SDK-compatible format (Error as string)
		sdkResult := struct {
			StatusCode int               `json:"statusCode"`
			Headers    map[string]string `json:"headers"`
			Body       []byte            `json:"body"`
			Error      string            `json:"error,omitempty"`
		}{
			StatusCode: result.StatusCode,
			Headers:    result.Headers,
			Body:       result.Body,
		}
		if result.Error != "" {
			sdkResult.Error = result.Error
		}

		return json.Marshal(sdkResult)
	}); err != nil {
		t.Fatalf("Failed to register resource_call host function: %v", err)
	}

	// 6. Set up test user with permissions
	principal := "user:test"
	permissions := types.NewPermissionContext([]types.PermissionEntry{
		{
			URIPattern:  "file://*/*",
			Permissions: []string{"read", "write"},
		},
	})

	if err := authz.SetPermissions(ctx, principal, permissions); err != nil {
		t.Fatalf("Failed to set permissions: %v", err)
	}

	// 7. Create simple invocation handler for testing
	invoker := &testInvoker{runtimeExec: runtimeExec}

	// 8. Set up MCP HTTP transport
	// Create simple auth driver that accepts any user
	simpleAuth := &simpleAuthDriver{principal: principal}
	// Create simple package manager mock for integration test
	pkgMgr := &simplePackageManager{}
	mcpDriver := request.NewHTTPMCPDriver(":0", []types.SecurityAuthn{simpleAuth}, authz, pkgMgr)
	mcpDriver.SetInvoker(invoker)

	if err := mcpDriver.Start(ctx); err != nil {
		t.Fatalf("Failed to start MCP driver: %v", err)
	}
	defer mcpDriver.Stop(ctx)

	// Get the actual bound address
	addr := mcpDriver.Addr()
	t.Logf("MCP server listening on: %s", addr)

	// 9. Make MCP request to invoke file-reader app
	// Arguments are passed as key-value pairs, converted to CLI flags
	mcpReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "test-1",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": appID,
			"arguments": map[string]interface{}{
				"path":   testFile,
				"format": "text",
			},
		},
	}

	reqBody, err := json.Marshal(mcpReq)
	if err != nil {
		t.Fatalf("Failed to marshal MCP request: %v", err)
	}

	httpReq, err := http.NewRequest("POST", fmt.Sprintf("http://%s/mcp", addr), bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		t.Fatalf("Failed to send MCP request: %v", err)
	}
	defer resp.Body.Close()

	// 10. Verify response
	var mcpResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&mcpResp); err != nil {
		t.Fatalf("Failed to decode MCP response: %v", err)
	}

	t.Logf("MCP Response: %+v", mcpResp)

	// Check for error
	if errObj, ok := mcpResp["error"]; ok {
		t.Fatalf("MCP returned error: %v", errObj)
	}

	// Check result
	result, ok := mcpResp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result object, got: %T", mcpResp["result"])
	}

	// Check content
	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatalf("Expected content array, got: %T", result["content"])
	}

	contentBlock, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected content block object, got: %T", content[0])
	}

	text, ok := contentBlock["text"].(string)
	if !ok {
		t.Fatalf("Expected text field, got: %T", contentBlock["text"])
	}

	// Verify the file contents are in the response
	if !contains(text, testContent) {
		t.Errorf("Expected response to contain %q, got: %s", testContent, text)
	}

	t.Logf("✅ End-to-end test passed!")
	t.Logf("   MCP client → file-reader app → WASM file driver → filesystem")
	t.Logf("   File contents retrieved successfully")
}

// testInvoker is a simple invocation handler for testing
type testInvoker struct {
	runtimeExec types.RuntimeExec
}

func (t *testInvoker) Invoke(ctx context.Context, req *types.InvocationRequest) (*types.InvocationResult, error) {
	return t.runtimeExec.Execute(ctx, req)
}

// simpleAuthDriver is a test auth driver that always returns a fixed principal
type simpleAuthDriver struct {
	principal string
}

func (s *simpleAuthDriver) Name() string {
	return "test.authn"
}

func (s *simpleAuthDriver) Authenticate(ctx context.Context, payload *types.AuthPayload) (string, error) {
	return s.principal, nil
}

// simplePackageManager is a test package manager that returns basic metadata
type simplePackageManager struct{}

func (s *simplePackageManager) Name() string {
	return "test.pkg.manager"
}

func (s *simplePackageManager) Resolve(ctx context.Context, name string) (string, error) {
	return name, nil // Just return the name as-is for testing
}

func (s *simplePackageManager) Get(ctx context.Context, appID string) (*types.AppMetadata, error) {
	return &types.AppMetadata{
		Name:    appID,
		Version: "1.0.0",
		Author:  "test",
	}, nil
}

func (s *simplePackageManager) List(ctx context.Context) ([]*types.AppMetadata, error) {
	return []*types.AppMetadata{}, nil
}

func (s *simplePackageManager) Install(ctx context.Context, pkgData []byte) (*types.AppMetadata, error) {
	return nil, nil
}

func (s *simplePackageManager) Uninstall(ctx context.Context, appID string) error {
	return nil
}

func (s *simplePackageManager) GetWasmBinary(appID string) ([]byte, error) {
	return []byte("mock wasm binary"), nil
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
