package request

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wazeos/wazeos/internal/types"
)

// Mock invoker for testing
type mockInvoker struct {
	invokeFn func(ctx context.Context, req *types.InvocationRequest) (*types.InvocationResult, error)
}

func (m *mockInvoker) Invoke(ctx context.Context, req *types.InvocationRequest) (*types.InvocationResult, error) {
	if m.invokeFn != nil {
		return m.invokeFn(ctx, req)
	}
	return &types.InvocationResult{
		RequestID:  req.Context.RequestID,
		Stdout:     []byte("success"),
		Stderr:     []byte{},
		ExitCode:   0,
		Duration:   time.Millisecond,
		MemoryUsed: 1024,
	}, nil
}

// Mock authn driver
type mockAuthn struct {
	authenticateFn func(ctx context.Context, payload *types.AuthPayload) (string, error)
}

func (m *mockAuthn) Name() string {
	return "mock.authn"
}

func (m *mockAuthn) Authenticate(ctx context.Context, payload *types.AuthPayload) (string, error) {
	if m.authenticateFn != nil {
		return m.authenticateFn(ctx, payload)
	}
	return "user:test", nil
}

// Mock authz driver
type mockAuthz struct {
	getPermissionsFn func(ctx context.Context, principal string) (*types.PermissionContext, error)
}

func (m *mockAuthz) Name() string {
	return "mock.authz"
}

func (m *mockAuthz) GetPermissions(ctx context.Context, principal string) (*types.PermissionContext, error) {
	if m.getPermissionsFn != nil {
		return m.getPermissionsFn(ctx, principal)
	}
	return types.NewPermissionContext([]types.PermissionEntry{
		{URIPattern: "*", Permissions: []string{"read", "write", "execute"}},
	}), nil
}

func (m *mockAuthz) SetPermissions(ctx context.Context, principal string, permissions *types.PermissionContext) error {
	return nil
}

func (m *mockAuthz) CheckAccess(uri string, requiredPermissions []string, permissions *types.PermissionContext) error {
	return nil
}

// Mock package manager
type mockPackageManager struct {
	resolveFn func(ctx context.Context, name string) (string, error)
	getFn     func(ctx context.Context, appID string) (*types.AppMetadata, error)
	listFn    func(ctx context.Context) ([]*types.AppMetadata, error)
}

func (m *mockPackageManager) Name() string {
	return "mock.package.manager"
}

func (m *mockPackageManager) Resolve(ctx context.Context, name string) (string, error) {
	if m.resolveFn != nil {
		return m.resolveFn(ctx, name)
	}
	return name + "_1.0.0", nil
}

func (m *mockPackageManager) Get(ctx context.Context, appID string) (*types.AppMetadata, error) {
	if m.getFn != nil {
		return m.getFn(ctx, appID)
	}
	return &types.AppMetadata{
		Name:    "test-app",
		Version: "1.0.0",
		Author:  "test",
	}, nil
}

func (m *mockPackageManager) List(ctx context.Context) ([]*types.AppMetadata, error) {
	if m.listFn != nil {
		return m.listFn(ctx)
	}
	return []*types.AppMetadata{
		{Name: "test-app", Version: "1.0.0", Author: "test"},
	}, nil
}

func (m *mockPackageManager) Install(ctx context.Context, pkgData []byte) (*types.AppMetadata, error) {
	return nil, nil
}

func (m *mockPackageManager) Uninstall(ctx context.Context, appID string) error {
	return nil
}

func (m *mockPackageManager) GetWasmBinary(appID string) ([]byte, error) {
	return []byte("mock wasm binary"), nil
}

func TestNewHTTPMCPDriver(t *testing.T) {
	authn := []types.SecurityAuthn{&mockAuthn{}}
	authz := &mockAuthz{}
	pkgMgr := &mockPackageManager{}

	driver := NewHTTPMCPDriver(":9090", authn, authz, pkgMgr)
	assert.NotNil(t, driver)
	assert.Equal(t, ":9090", driver.addr)
	assert.NotNil(t, driver.handler)
	assert.Equal(t, authn, driver.handler.authn)
	assert.Equal(t, authz, driver.handler.authz)
	assert.Equal(t, pkgMgr, driver.handler.packageManager)
}

func TestNewHTTPMCPDriver_DefaultAddr(t *testing.T) {
	driver := NewHTTPMCPDriver("", nil, nil, &mockPackageManager{})
	assert.NotNil(t, driver)
	assert.Equal(t, ":8080", driver.addr)
}

func TestHTTPMCPDriver_Name(t *testing.T) {
	driver := NewHTTPMCPDriver("", nil, nil, &mockPackageManager{})
	assert.Equal(t, "wazeos/http", driver.Name())
}

func TestHTTPMCPDriver_Patterns(t *testing.T) {
	driver := NewHTTPMCPDriver("", nil, nil, &mockPackageManager{})
	patterns := driver.Patterns()
	assert.Contains(t, patterns, "http://*")
	assert.Contains(t, patterns, "https://*")
}

func TestHTTPMCPDriver_SetInvoker(t *testing.T) {
	driver := NewHTTPMCPDriver("", nil, nil, &mockPackageManager{})
	invoker := &mockInvoker{}

	driver.SetInvoker(invoker)

	driver.mu.RLock()
	assert.Equal(t, invoker, driver.handler.invoker)
	driver.mu.RUnlock()
}

func TestHTTPMCPDriver_Start_Success(t *testing.T) {
	driver := NewHTTPMCPDriver(":0", nil, nil, &mockPackageManager{}) // Port 0 = random port
	driver.SetInvoker(&mockInvoker{})

	ctx := context.Background()
	err := driver.Start(ctx)
	assert.NoError(t, err)
	assert.True(t, driver.started)

	// Cleanup
	driver.Stop(ctx)
}

func TestHTTPMCPDriver_Start_NoInvoker(t *testing.T) {
	driver := NewHTTPMCPDriver(":0", nil, nil, &mockPackageManager{})

	ctx := context.Background()
	err := driver.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invoker not set")
}

func TestHTTPMCPDriver_Start_AlreadyStarted(t *testing.T) {
	driver := NewHTTPMCPDriver(":0", nil, nil, &mockPackageManager{})
	driver.SetInvoker(&mockInvoker{})

	ctx := context.Background()
	err := driver.Start(ctx)
	require.NoError(t, err)

	// Try to start again
	err = driver.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already started")

	// Cleanup
	driver.Stop(ctx)
}

func TestHTTPMCPDriver_Stop_Success(t *testing.T) {
	driver := NewHTTPMCPDriver(":0", nil, nil, &mockPackageManager{})
	driver.SetInvoker(&mockInvoker{})

	ctx := context.Background()
	err := driver.Start(ctx)
	require.NoError(t, err)

	err = driver.Stop(ctx)
	assert.NoError(t, err)
	assert.False(t, driver.started)
}

func TestHTTPMCPDriver_Stop_NotStarted(t *testing.T) {
	driver := NewHTTPMCPDriver(":0", nil, nil, &mockPackageManager{})

	ctx := context.Background()
	err := driver.Stop(ctx)
	assert.NoError(t, err)
}

func TestHTTPMCPDriver_HealthEndpoint(t *testing.T) {
	driver := NewHTTPMCPDriver(":0", nil, nil, &mockPackageManager{})
	driver.SetInvoker(&mockInvoker{})

	ctx := context.Background()
	err := driver.Start(ctx)
	require.NoError(t, err)
	defer driver.Stop(ctx)

	// Wait for server to start
	time.Sleep(50 * time.Millisecond)

	// Get the actual address
	addr := driver.Addr()

	// Make health check request
	resp, err := http.Get(fmt.Sprintf("http://%s/health", addr))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "ok", result["status"])
}

func TestHTTPMCPDriver_MCPEndpoint_Success(t *testing.T) {
	authn := []types.SecurityAuthn{&mockAuthn{}}
	authz := &mockAuthz{}
	invoker := &mockInvoker{
		invokeFn: func(ctx context.Context, req *types.InvocationRequest) (*types.InvocationResult, error) {
			assert.Equal(t, "test-app", req.AppID)
			return &types.InvocationResult{
				RequestID:  req.Context.RequestID,
				Stdout:     []byte("Hello, World!"),
				Stderr:     []byte{},
				ExitCode:   0,
				Duration:   time.Millisecond,
				MemoryUsed: 1024,
			}, nil
		},
	}

	driver := NewHTTPMCPDriver(":0", authn, authz, &mockPackageManager{})
	driver.SetInvoker(invoker)

	ctx := context.Background()
	err := driver.Start(ctx)
	require.NoError(t, err)
	defer driver.Stop(ctx)

	// Wait for server to start
	time.Sleep(50 * time.Millisecond)

	// Create MCP request
	mcpReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      "test-123",
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "test-app",
			"arguments": map[string]interface{}{
				"foo": "bar",
			},
		},
	}

	reqBody, _ := json.Marshal(mcpReq)
	resp, err := http.Post(
		fmt.Sprintf("http://%s/mcp", driver.Addr()),
		"application/json",
		bytes.NewReader(reqBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var mcpResp MCPResponse
	err = json.NewDecoder(resp.Body).Decode(&mcpResp)
	require.NoError(t, err)

	assert.Equal(t, "2.0", mcpResp.JSONRPC)
	assert.Equal(t, "test-123", mcpResp.ID)
	assert.Nil(t, mcpResp.Error)
	assert.NotNil(t, mcpResp.Result)

	// Check result
	resultMap, ok := mcpResp.Result.(map[string]interface{})
	require.True(t, ok)
	assert.False(t, resultMap["isError"].(bool))

	content := resultMap["content"].([]interface{})
	require.Len(t, content, 1)

	contentItem := content[0].(map[string]interface{})
	assert.Equal(t, "text", contentItem["type"])
	assert.Equal(t, "Hello, World!", contentItem["text"])
}

func TestHTTPMCPDriver_MCPEndpoint_MethodNotAllowed(t *testing.T) {
	driver := NewHTTPMCPDriver(":0", nil, nil, &mockPackageManager{})
	driver.SetInvoker(&mockInvoker{})

	ctx := context.Background()
	err := driver.Start(ctx)
	require.NoError(t, err)
	defer driver.Stop(ctx)

	time.Sleep(50 * time.Millisecond)

	// Try GET instead of POST
	resp, err := http.Get(fmt.Sprintf("http://%s/mcp", driver.Addr()))
	require.NoError(t, err)
	defer resp.Body.Close()

	var mcpResp MCPResponse
	json.NewDecoder(resp.Body).Decode(&mcpResp)

	assert.NotNil(t, mcpResp.Error)
	assert.Equal(t, -32601, mcpResp.Error.Code)
	assert.Contains(t, mcpResp.Error.Message, "Method not allowed")
}

func TestHTTPMCPDriver_MCPEndpoint_InvalidJSON(t *testing.T) {
	driver := NewHTTPMCPDriver(":0", nil, nil, &mockPackageManager{})
	driver.SetInvoker(&mockInvoker{})

	ctx := context.Background()
	err := driver.Start(ctx)
	require.NoError(t, err)
	defer driver.Stop(ctx)

	time.Sleep(50 * time.Millisecond)

	resp, err := http.Post(
		fmt.Sprintf("http://%s/mcp", driver.Addr()),
		"application/json",
		bytes.NewReader([]byte("invalid json")),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	var mcpResp MCPResponse
	json.NewDecoder(resp.Body).Decode(&mcpResp)

	assert.NotNil(t, mcpResp.Error)
	assert.Equal(t, -32700, mcpResp.Error.Code)
	assert.Contains(t, mcpResp.Error.Message, "Parse error")
}

func TestHTTPMCPDriver_MCPEndpoint_InvalidJSONRPCVersion(t *testing.T) {
	driver := NewHTTPMCPDriver(":0", nil, nil, &mockPackageManager{})
	driver.SetInvoker(&mockInvoker{})

	ctx := context.Background()
	err := driver.Start(ctx)
	require.NoError(t, err)
	defer driver.Stop(ctx)

	time.Sleep(50 * time.Millisecond)

	mcpReq := map[string]interface{}{
		"jsonrpc": "1.0",
		"id":      "test",
		"method":  "tools/call",
	}

	reqBody, _ := json.Marshal(mcpReq)
	resp, err := http.Post(
		fmt.Sprintf("http://%s/mcp", driver.Addr()),
		"application/json",
		bytes.NewReader(reqBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	var mcpResp MCPResponse
	json.NewDecoder(resp.Body).Decode(&mcpResp)

	assert.NotNil(t, mcpResp.Error)
	assert.Equal(t, -32600, mcpResp.Error.Code)
	assert.Contains(t, mcpResp.Error.Message, "jsonrpc must be '2.0'")
}

func TestHTTPMCPDriver_MCPEndpoint_UnknownMethod(t *testing.T) {
	authn := []types.SecurityAuthn{&mockAuthn{}}
	driver := NewHTTPMCPDriver(":0", authn, nil, &mockPackageManager{})
	driver.SetInvoker(&mockInvoker{})

	ctx := context.Background()
	err := driver.Start(ctx)
	require.NoError(t, err)
	defer driver.Stop(ctx)

	time.Sleep(50 * time.Millisecond)

	mcpReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      "test",
		Method:  "unknown/method",
	}

	reqBody, _ := json.Marshal(mcpReq)
	resp, err := http.Post(
		fmt.Sprintf("http://%s/mcp", driver.Addr()),
		"application/json",
		bytes.NewReader(reqBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	var mcpResp MCPResponse
	json.NewDecoder(resp.Body).Decode(&mcpResp)

	assert.NotNil(t, mcpResp.Error)
	assert.Equal(t, -32601, mcpResp.Error.Code)
	assert.Contains(t, mcpResp.Error.Message, "Method not found")
}

func TestHTTPMCPDriver_MCPEndpoint_AuthenticationFailure(t *testing.T) {
	authn := []types.SecurityAuthn{
		&mockAuthn{
			authenticateFn: func(ctx context.Context, payload *types.AuthPayload) (string, error) {
				return "", types.ErrAbstain
			},
		},
	}

	driver := NewHTTPMCPDriver(":0", authn, nil, &mockPackageManager{})
	driver.SetInvoker(&mockInvoker{})

	ctx := context.Background()
	err := driver.Start(ctx)
	require.NoError(t, err)
	defer driver.Stop(ctx)

	time.Sleep(50 * time.Millisecond)

	mcpReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      "test",
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "test-app",
		},
	}

	reqBody, _ := json.Marshal(mcpReq)
	resp, err := http.Post(
		fmt.Sprintf("http://%s/mcp", driver.Addr()),
		"application/json",
		bytes.NewReader(reqBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	var mcpResp MCPResponse
	json.NewDecoder(resp.Body).Decode(&mcpResp)

	assert.NotNil(t, mcpResp.Error)
	assert.Equal(t, -32001, mcpResp.Error.Code)
	assert.Contains(t, mcpResp.Error.Message, "Authentication required")
}

func TestHTTPMCPDriver_MCPEndpoint_MissingName(t *testing.T) {
	authn := []types.SecurityAuthn{&mockAuthn{}}
	authz := &mockAuthz{}

	driver := NewHTTPMCPDriver(":0", authn, authz, &mockPackageManager{})
	driver.SetInvoker(&mockInvoker{})

	ctx := context.Background()
	err := driver.Start(ctx)
	require.NoError(t, err)
	defer driver.Stop(ctx)

	time.Sleep(50 * time.Millisecond)

	mcpReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      "test",
		Method:  "tools/call",
		Params:  map[string]interface{}{},
	}

	reqBody, _ := json.Marshal(mcpReq)
	resp, err := http.Post(
		fmt.Sprintf("http://%s/mcp", driver.Addr()),
		"application/json",
		bytes.NewReader(reqBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	var mcpResp MCPResponse
	json.NewDecoder(resp.Body).Decode(&mcpResp)

	assert.NotNil(t, mcpResp.Error)
	assert.Equal(t, -32602, mcpResp.Error.Code)
	assert.Contains(t, mcpResp.Error.Message, "'name' required")
}

func TestHTTPMCPDriver_MCPEndpoint_AppNotFound(t *testing.T) {
	authn := []types.SecurityAuthn{&mockAuthn{}}
	authz := &mockAuthz{}
	invoker := &mockInvoker{
		invokeFn: func(ctx context.Context, req *types.InvocationRequest) (*types.InvocationResult, error) {
			return nil, types.ErrNotFound
		},
	}

	driver := NewHTTPMCPDriver(":0", authn, authz, &mockPackageManager{})
	driver.SetInvoker(invoker)

	ctx := context.Background()
	err := driver.Start(ctx)
	require.NoError(t, err)
	defer driver.Stop(ctx)

	time.Sleep(50 * time.Millisecond)

	mcpReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      "test",
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "nonexistent-app",
		},
	}

	reqBody, _ := json.Marshal(mcpReq)
	resp, err := http.Post(
		fmt.Sprintf("http://%s/mcp", driver.Addr()),
		"application/json",
		bytes.NewReader(reqBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	var mcpResp MCPResponse
	json.NewDecoder(resp.Body).Decode(&mcpResp)

	assert.NotNil(t, mcpResp.Error)
	assert.Equal(t, -32004, mcpResp.Error.Code)
	assert.Contains(t, mcpResp.Error.Message, "App not found")
}

func TestHTTPMCPDriver_MCPEndpoint_PermissionDenied(t *testing.T) {
	authn := []types.SecurityAuthn{&mockAuthn{}}
	authz := &mockAuthz{}
	invoker := &mockInvoker{
		invokeFn: func(ctx context.Context, req *types.InvocationRequest) (*types.InvocationResult, error) {
			return nil, types.ErrPermissionDenied
		},
	}

	driver := NewHTTPMCPDriver(":0", authn, authz, &mockPackageManager{})
	driver.SetInvoker(invoker)

	ctx := context.Background()
	err := driver.Start(ctx)
	require.NoError(t, err)
	defer driver.Stop(ctx)

	time.Sleep(50 * time.Millisecond)

	mcpReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      "test",
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "test-app",
		},
	}

	reqBody, _ := json.Marshal(mcpReq)
	resp, err := http.Post(
		fmt.Sprintf("http://%s/mcp", driver.Addr()),
		"application/json",
		bytes.NewReader(reqBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	var mcpResp MCPResponse
	json.NewDecoder(resp.Body).Decode(&mcpResp)

	assert.NotNil(t, mcpResp.Error)
	assert.Equal(t, -32005, mcpResp.Error.Code)
	assert.Contains(t, mcpResp.Error.Message, "Permission denied")
}

func TestHTTPMCPDriver_MCPEndpoint_WithStderr(t *testing.T) {
	authn := []types.SecurityAuthn{&mockAuthn{}}
	authz := &mockAuthz{}
	invoker := &mockInvoker{
		invokeFn: func(ctx context.Context, req *types.InvocationRequest) (*types.InvocationResult, error) {
			return &types.InvocationResult{
				RequestID:  req.Context.RequestID,
				Stdout:     []byte("output"),
				Stderr:     []byte("error message"),
				ExitCode:   1,
				Duration:   time.Millisecond,
				MemoryUsed: 1024,
			}, nil
		},
	}

	driver := NewHTTPMCPDriver(":0", authn, authz, &mockPackageManager{})
	driver.SetInvoker(invoker)

	ctx := context.Background()
	err := driver.Start(ctx)
	require.NoError(t, err)
	defer driver.Stop(ctx)

	time.Sleep(50 * time.Millisecond)

	mcpReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      "test",
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "test-app",
		},
	}

	reqBody, _ := json.Marshal(mcpReq)
	resp, err := http.Post(
		fmt.Sprintf("http://%s/mcp", driver.Addr()),
		"application/json",
		bytes.NewReader(reqBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	var mcpResp MCPResponse
	json.NewDecoder(resp.Body).Decode(&mcpResp)

	assert.Nil(t, mcpResp.Error)
	resultMap := mcpResp.Result.(map[string]interface{})
	assert.True(t, resultMap["isError"].(bool))

	content := resultMap["content"].([]interface{})
	require.Len(t, content, 2)

	// Check stdout
	content0 := content[0].(map[string]interface{})
	assert.Equal(t, "output", content0["text"])

	// Check stderr
	content1 := content[1].(map[string]interface{})
	assert.Contains(t, content1["text"], "stderr: error message")
}

func TestHTTPMCPDriver_MCPEndpoint_NoOutput(t *testing.T) {
	authn := []types.SecurityAuthn{&mockAuthn{}}
	authz := &mockAuthz{}
	invoker := &mockInvoker{
		invokeFn: func(ctx context.Context, req *types.InvocationRequest) (*types.InvocationResult, error) {
			return &types.InvocationResult{
				RequestID:  req.Context.RequestID,
				Stdout:     []byte{},
				Stderr:     []byte{},
				ExitCode:   0,
				Duration:   time.Millisecond,
				MemoryUsed: 1024,
			}, nil
		},
	}

	driver := NewHTTPMCPDriver(":0", authn, authz, &mockPackageManager{})
	driver.SetInvoker(invoker)

	ctx := context.Background()
	err := driver.Start(ctx)
	require.NoError(t, err)
	defer driver.Stop(ctx)

	time.Sleep(50 * time.Millisecond)

	mcpReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      "test",
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "test-app",
		},
	}

	reqBody, _ := json.Marshal(mcpReq)
	resp, err := http.Post(
		fmt.Sprintf("http://%s/mcp", driver.Addr()),
		"application/json",
		bytes.NewReader(reqBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	var mcpResp MCPResponse
	json.NewDecoder(resp.Body).Decode(&mcpResp)

	assert.Nil(t, mcpResp.Error)
	resultMap := mcpResp.Result.(map[string]interface{})

	content := resultMap["content"].([]interface{})
	require.Len(t, content, 1)

	content0 := content[0].(map[string]interface{})
	assert.Contains(t, content0["text"], "Execution completed with exit code 0")
}

func TestHTTPMCPDriver_Authenticate_NoDrivers(t *testing.T) {
	driver := NewHTTPMCPDriver(":0", nil, nil, &mockPackageManager{})

	ctx := context.Background()
	payload := &types.AuthPayload{
		Headers: map[string]string{},
	}

	principal, err := driver.handler.authenticate(ctx, payload)
	assert.Equal(t, "", principal)
	assert.True(t, types.IsAbstain(err))
}

func TestHTTPMCPDriver_Authenticate_MultipleDrivers(t *testing.T) {
	authn1 := &mockAuthn{
		authenticateFn: func(ctx context.Context, payload *types.AuthPayload) (string, error) {
			return "", types.ErrAbstain
		},
	}
	authn2 := &mockAuthn{
		authenticateFn: func(ctx context.Context, payload *types.AuthPayload) (string, error) {
			return "user:alice", nil
		},
	}

	driver := NewHTTPMCPDriver(":0", []types.SecurityAuthn{authn1, authn2}, nil, &mockPackageManager{})

	ctx := context.Background()
	payload := &types.AuthPayload{
		Headers: map[string]string{},
	}

	principal, err := driver.handler.authenticate(ctx, payload)
	assert.NoError(t, err)
	assert.Equal(t, "user:alice", principal)
}
