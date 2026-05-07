package request

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wazeos/wazeos/internal/types"
)

func TestNewStdioMCPDriver(t *testing.T) {
	authn := []types.SecurityAuthn{&mockAuthn{}}
	authz := &mockAuthz{}
	pkgMgr := &mockPackageManager{}

	driver := NewStdioMCPDriver(authn, authz, pkgMgr)
	assert.NotNil(t, driver)
	assert.NotNil(t, driver.handler)
	assert.Equal(t, authn, driver.handler.authn)
	assert.Equal(t, authz, driver.handler.authz)
	assert.Equal(t, pkgMgr, driver.handler.packageManager)
}

func TestStdioMCPDriver_Name(t *testing.T) {
	driver := NewStdioMCPDriver(nil, nil, &mockPackageManager{})
	assert.Equal(t, "io.request.stdio", driver.Name())
}

func TestStdioMCPDriver_Patterns(t *testing.T) {
	driver := NewStdioMCPDriver(nil, nil, &mockPackageManager{})
	patterns := driver.Patterns()
	assert.Contains(t, patterns, "stdio://*")
}

func TestStdioMCPDriver_SetInvoker(t *testing.T) {
	driver := NewStdioMCPDriver(nil, nil, &mockPackageManager{})
	invoker := &mockInvoker{}

	driver.SetInvoker(invoker)

	driver.mu.RLock()
	assert.Equal(t, invoker, driver.handler.invoker)
	driver.mu.RUnlock()
}

func TestStdioMCPDriver_ToolsCall_Success(t *testing.T) {
	// Setup
	authn := []types.SecurityAuthn{&mockAuthn{}}
	pkgMgr := &mockPackageManager{}
	invoker := &mockInvoker{}

	driver := NewStdioMCPDriver(authn, nil, pkgMgr)
	driver.SetInvoker(invoker)

	// Create pipes for stdin/stdout
	input := `{"jsonrpc":"2.0","id":"test-1","method":"tools/call","params":{"name":"test-app","arguments":{}}}`
	reader := strings.NewReader(input + "\n")
	writer := &bytes.Buffer{}

	driver.reader = reader
	driver.writer = writer

	// Run driver in goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		driver.Start(ctx)
	}()

	// Wait for response
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Parse response
	var resp MCPResponse
	err := json.Unmarshal(writer.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, "test-1", resp.ID)
	assert.Nil(t, resp.Error)

	// Check result
	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	assert.False(t, result["isError"].(bool))

	content, ok := result["content"].([]interface{})
	require.True(t, ok)
	require.Len(t, content, 1)

	contentBlock, ok := content[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "text", contentBlock["type"])
	assert.Equal(t, "success", contentBlock["text"])
}

func TestStdioMCPDriver_ToolsList_Success(t *testing.T) {
	// Setup
	authn := []types.SecurityAuthn{&mockAuthn{}}
	pkgMgr := &mockPackageManager{}
	invoker := &mockInvoker{}

	driver := NewStdioMCPDriver(authn, nil, pkgMgr)
	driver.SetInvoker(invoker)

	// Create pipes for stdin/stdout
	input := `{"jsonrpc":"2.0","id":"test-1","method":"tools/list","params":{}}`
	reader := strings.NewReader(input + "\n")
	writer := &bytes.Buffer{}

	driver.reader = reader
	driver.writer = writer

	// Run driver in goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		driver.Start(ctx)
	}()

	// Wait for response
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Parse response
	var resp MCPResponse
	err := json.Unmarshal(writer.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, "test-1", resp.ID)
	assert.Nil(t, resp.Error)

	// Check result
	result, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)

	tools, ok := result["tools"].([]interface{})
	require.True(t, ok)
	assert.Len(t, tools, 1)

	tool, ok := tools[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "test-app", tool["name"])
}

func TestStdioMCPDriver_InvalidJSON(t *testing.T) {
	// Setup
	authn := []types.SecurityAuthn{&mockAuthn{}}
	driver := NewStdioMCPDriver(authn, nil, &mockPackageManager{})
	driver.SetInvoker(&mockInvoker{})

	// Invalid JSON
	input := `{invalid json}`
	reader := strings.NewReader(input + "\n")
	writer := &bytes.Buffer{}

	driver.reader = reader
	driver.writer = writer

	// Run driver
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		driver.Start(ctx)
	}()

	// Wait for response
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Parse response
	var resp MCPResponse
	err := json.Unmarshal(writer.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, -32700, resp.Error.Code)
	assert.Equal(t, "Parse error", resp.Error.Message)
}

func TestStdioMCPDriver_InvalidJSONRPCVersion(t *testing.T) {
	// Setup
	authn := []types.SecurityAuthn{&mockAuthn{}}
	driver := NewStdioMCPDriver(authn, nil, &mockPackageManager{})
	driver.SetInvoker(&mockInvoker{})

	// Wrong JSON-RPC version
	input := `{"jsonrpc":"1.0","id":"test-1","method":"tools/list","params":{}}`
	reader := strings.NewReader(input + "\n")
	writer := &bytes.Buffer{}

	driver.reader = reader
	driver.writer = writer

	// Run driver
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		driver.Start(ctx)
	}()

	// Wait for response
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Parse response
	var resp MCPResponse
	err := json.Unmarshal(writer.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, -32600, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "jsonrpc must be '2.0'")
}

func TestStdioMCPDriver_UnknownMethod(t *testing.T) {
	// Setup
	authn := []types.SecurityAuthn{&mockAuthn{}}
	driver := NewStdioMCPDriver(authn, nil, &mockPackageManager{})
	driver.SetInvoker(&mockInvoker{})

	// Unknown method
	input := `{"jsonrpc":"2.0","id":"test-1","method":"tools/unknown","params":{}}`
	reader := strings.NewReader(input + "\n")
	writer := &bytes.Buffer{}

	driver.reader = reader
	driver.writer = writer

	// Run driver
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		driver.Start(ctx)
	}()

	// Wait for response
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Parse response
	var resp MCPResponse
	err := json.Unmarshal(writer.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, -32601, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "Method not found")
}

func TestStdioMCPDriver_EmptyLines(t *testing.T) {
	// Setup
	authn := []types.SecurityAuthn{&mockAuthn{}}
	driver := NewStdioMCPDriver(authn, nil, &mockPackageManager{})
	driver.SetInvoker(&mockInvoker{})

	// Input with empty lines
	input := "\n\n" + `{"jsonrpc":"2.0","id":"test-1","method":"tools/list","params":{}}` + "\n"
	reader := strings.NewReader(input)
	writer := &bytes.Buffer{}

	driver.reader = reader
	driver.writer = writer

	// Run driver
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		driver.Start(ctx)
	}()

	// Wait for response
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Should get one response (empty lines ignored)
	var resp MCPResponse
	err := json.Unmarshal(writer.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, "test-1", resp.ID)
	assert.Nil(t, resp.Error)
}

func TestStdioMCPDriver_AuthenticationRequired(t *testing.T) {
	// Setup with no auth drivers
	driver := NewStdioMCPDriver(nil, nil, &mockPackageManager{})
	driver.SetInvoker(&mockInvoker{})

	// Request
	input := `{"jsonrpc":"2.0","id":"test-1","method":"tools/list","params":{}}`
	reader := strings.NewReader(input + "\n")
	writer := &bytes.Buffer{}

	driver.reader = reader
	driver.writer = writer

	// Run driver
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		driver.Start(ctx)
	}()

	// Wait for response
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Parse response
	var resp MCPResponse
	err := json.Unmarshal(writer.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, -32001, resp.Error.Code)
	assert.Equal(t, "Authentication required", resp.Error.Message)
}

func TestStdioMCPDriver_Stop_NotStarted(t *testing.T) {
	driver := NewStdioMCPDriver(nil, nil, &mockPackageManager{})

	err := driver.Stop(context.Background())
	assert.NoError(t, err)
}
