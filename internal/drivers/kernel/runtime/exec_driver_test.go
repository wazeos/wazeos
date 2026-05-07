package runtime

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wazeos/wazeos/internal/types"
)

// mockPackageManager for testing
type mockPackageManager struct {
	apps         map[string]*types.AppMetadata
	wasmBinaries map[string][]byte
}

func newMockPackageManager() *mockPackageManager {
	return &mockPackageManager{
		apps:         make(map[string]*types.AppMetadata),
		wasmBinaries: make(map[string][]byte),
	}
}

func (m *mockPackageManager) Name() string {
	return "mock-pkg-manager"
}

func (m *mockPackageManager) Install(ctx context.Context, zipData []byte) (*types.AppMetadata, error) {
	return nil, nil
}

func (m *mockPackageManager) Uninstall(ctx context.Context, appID string) error {
	return nil
}

func (m *mockPackageManager) List(ctx context.Context) ([]*types.AppMetadata, error) {
	apps := make([]*types.AppMetadata, 0, len(m.apps))
	for _, app := range m.apps {
		apps = append(apps, app)
	}
	return apps, nil
}

func (m *mockPackageManager) Get(ctx context.Context, appID string) (*types.AppMetadata, error) {
	app, ok := m.apps[appID]
	if !ok {
		return nil, types.ErrNotFound
	}
	return app, nil
}

func (m *mockPackageManager) Resolve(ctx context.Context, appNameOrID string) (string, error) {
	// Simple mock: return as-is if full ID, otherwise search by name
	if app, ok := m.apps[appNameOrID]; ok {
		return app.AppID(), nil
	}
	// Search by name
	for _, app := range m.apps {
		if app.Name == appNameOrID {
			return app.AppID(), nil
		}
	}
	return "", types.ErrNotFound
}

func (m *mockPackageManager) GetWasmBinary(appID string) ([]byte, error) {
	binary, ok := m.wasmBinaries[appID]
	if !ok {
		return nil, types.ErrNotFound
	}
	return binary, nil
}

func TestExecDriver_HandleCall_SchemaValidation_Valid(t *testing.T) {
	ctx := context.Background()

	// Create JSON schema that requires "name" field (string) and "age" field (number)
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "number"}
		},
		"required": ["name", "age"]
	}`)

	// Create app metadata with schema
	metadata := &types.AppMetadata{
		Name:        "test-app",
		Author:      "test",
		Version:     "1.0.0",
		Type:        "app",
		Description: "Test app with schema",
		InputSchema: &schema,
	}

	// Create mock package manager
	pkgMgr := newMockPackageManager()
	appID := metadata.AppID()
	pkgMgr.apps[appID] = metadata
	pkgMgr.wasmBinaries[appID] = helloWorldWasm

	// Create exec driver
	driver := NewExecDriver(pkgMgr, nil, nil)

	// Create valid call with matching schema
	validArgs := map[string]interface{}{
		"name": "Alice",
		"age":  30,
	}
	argsJSON, _ := json.Marshal(validArgs)

	call := &types.ResourceCall{
		URI:         "fn://test-app",
		Body:        argsJSON,
		Context:     types.NewExecutionContext("req-1", "trace-1", "user:test", types.NewPermissionContext(nil)),
		Permissions: []string{"invoke"},
	}

	// Execute - should succeed
	result, err := driver.HandleCall(ctx, call)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 200, result.StatusCode)
}

func TestExecDriver_HandleCall_SchemaValidation_Invalid(t *testing.T) {
	ctx := context.Background()

	// Create JSON schema that requires "name" field (string) and "age" field (number)
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "number"}
		},
		"required": ["name", "age"]
	}`)

	// Create app metadata with schema
	metadata := &types.AppMetadata{
		Name:        "test-app",
		Author:      "test",
		Version:     "1.0.0",
		Type:        "app",
		Description: "Test app with schema",
		InputSchema: &schema,
	}

	// Create mock package manager
	pkgMgr := newMockPackageManager()
	appID := metadata.AppID()
	pkgMgr.apps[appID] = metadata
	pkgMgr.wasmBinaries[appID] = helloWorldWasm

	// Create exec driver
	driver := NewExecDriver(pkgMgr, nil, nil)

	// Create invalid call - missing required "age" field
	invalidArgs := map[string]interface{}{
		"name": "Alice",
		// Missing "age" field
	}
	argsJSON, _ := json.Marshal(invalidArgs)

	call := &types.ResourceCall{
		URI:         "fn://test-app",
		Body:        argsJSON,
		Context:     types.NewExecutionContext("req-1", "trace-1", "user:test", types.NewPermissionContext(nil)),
		Permissions: []string{"invoke"},
	}

	// Execute - should fail validation
	result, err := driver.HandleCall(ctx, call)
	require.NoError(t, err) // No Go error, but HTTP error
	require.NotNil(t, result)
	assert.Equal(t, 400, result.StatusCode)
	assert.Contains(t, string(result.Body), "input validation failed")
	assert.Contains(t, string(result.Body), "age")
}

func TestExecDriver_HandleCall_SchemaValidation_WrongType(t *testing.T) {
	ctx := context.Background()

	// Create JSON schema that requires "age" to be a number
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "number"}
		},
		"required": ["name", "age"]
	}`)

	// Create app metadata with schema
	metadata := &types.AppMetadata{
		Name:        "test-app",
		Author:      "test",
		Version:     "1.0.0",
		Type:        "app",
		Description: "Test app with schema",
		InputSchema: &schema,
	}

	// Create mock package manager
	pkgMgr := newMockPackageManager()
	appID := metadata.AppID()
	pkgMgr.apps[appID] = metadata
	pkgMgr.wasmBinaries[appID] = helloWorldWasm

	// Create exec driver
	driver := NewExecDriver(pkgMgr, nil, nil)

	// Create invalid call - "age" is string instead of number
	invalidArgs := map[string]interface{}{
		"name": "Alice",
		"age":  "thirty", // Wrong type - should be number
	}
	argsJSON, _ := json.Marshal(invalidArgs)

	call := &types.ResourceCall{
		URI:         "fn://test-app",
		Body:        argsJSON,
		Context:     types.NewExecutionContext("req-1", "trace-1", "user:test", types.NewPermissionContext(nil)),
		Permissions: []string{"invoke"},
	}

	// Execute - should fail validation
	result, err := driver.HandleCall(ctx, call)
	require.NoError(t, err) // No Go error, but HTTP error
	require.NotNil(t, result)
	assert.Equal(t, 400, result.StatusCode)
	assert.Contains(t, string(result.Body), "input validation failed")
}

func TestExecDriver_HandleCall_NoSchema(t *testing.T) {
	ctx := context.Background()

	// Create app metadata WITHOUT schema
	metadata := &types.AppMetadata{
		Name:        "test-app",
		Author:      "test",
		Version:     "1.0.0",
		Type:        "app",
		Description: "Test app without schema",
		InputSchema: nil, // No schema
	}

	// Create mock package manager
	pkgMgr := newMockPackageManager()
	appID := metadata.AppID()
	pkgMgr.apps[appID] = metadata
	pkgMgr.wasmBinaries[appID] = helloWorldWasm

	// Create exec driver
	driver := NewExecDriver(pkgMgr, nil, nil)

	// Create call with any args (no validation should happen)
	anyArgs := map[string]interface{}{
		"random": "data",
		"foo":    123,
	}
	argsJSON, _ := json.Marshal(anyArgs)

	call := &types.ResourceCall{
		URI:         "fn://test-app",
		Body:        argsJSON,
		Context:     types.NewExecutionContext("req-1", "trace-1", "user:test", types.NewPermissionContext(nil)),
		Permissions: []string{"invoke"},
	}

	// Execute - should succeed (no validation)
	result, err := driver.HandleCall(ctx, call)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 200, result.StatusCode)
}
