package pkg

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wazeos/wazeos/internal/types"
)

// Mock RuntimeExec for testing
type mockRuntimeExec struct {
	loadedApps   map[string][]byte
	loadError    error
	unloadError  error
}

func (m *mockRuntimeExec) Name() string {
	return "mock.runtime"
}

func (m *mockRuntimeExec) LoadApp(ctx context.Context, appID string, wasmBytes []byte) error {
	if m.loadError != nil {
		return m.loadError
	}
	if m.loadedApps == nil {
		m.loadedApps = make(map[string][]byte)
	}
	m.loadedApps[appID] = wasmBytes
	return nil
}

func (m *mockRuntimeExec) LoadDriver(ctx context.Context, appID string, wasmBytes []byte, metadata *types.AppMetadata) error {
	// For testing, just delegate to LoadApp
	return m.LoadApp(ctx, appID, wasmBytes)
}

func (m *mockRuntimeExec) UnloadApp(ctx context.Context, appID string) error {
	if m.unloadError != nil {
		return m.unloadError
	}
	delete(m.loadedApps, appID)
	return nil
}

func (m *mockRuntimeExec) Execute(ctx context.Context, req *types.InvocationRequest) (*types.InvocationResult, error) {
	return nil, nil
}

func (m *mockRuntimeExec) RegisterHostFunction(namespace, name string, fn types.HostFunction) error {
	return nil
}

func (m *mockRuntimeExec) SetResourceBus(bus types.ResourceBus) {}

func (m *mockRuntimeExec) GetMetadata(ctx context.Context, wasmBytes []byte) (*types.AppMetadata, error) {
	return &types.AppMetadata{
		Name:    "test-app",
		Version: "1.0.0",
		Author:  "test",
	}, nil
}

// Helper to create test ZIP packages
func createTestZip(metadata *types.AppMetadata, wasmData []byte) ([]byte, error) {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	// Add metadata.json
	if metadata != nil {
		metadataJSON, err := json.Marshal(metadata)
		if err != nil {
			return nil, err
		}

		f, err := w.Create("metadata.json")
		if err != nil {
			return nil, err
		}
		if _, err := f.Write(metadataJSON); err != nil {
			return nil, err
		}
	}

	// Add app.wasm
	if wasmData != nil {
		f, err := w.Create("app.wasm")
		if err != nil {
			return nil, err
		}
		if _, err := f.Write(wasmData); err != nil {
			return nil, err
		}
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func TestNewPackageManager(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := &mockRuntimeExec{}

	pm, err := NewPackageManager(tmpDir, runtime)
	assert.NoError(t, err)
	assert.NotNil(t, pm)
	assert.Equal(t, "kernel.pkg", pm.Name())

	// Verify directory was created
	_, err = os.Stat(tmpDir)
	assert.NoError(t, err)
}

func TestNewPackageManager_DefaultPath(t *testing.T) {
	runtime := &mockRuntimeExec{}

	pm, err := NewPackageManager("", runtime)
	assert.NoError(t, err)
	assert.NotNil(t, pm)
	// When empty string is passed, it uses $HOME/.wazeos/data
	home, _ := os.UserHomeDir()
	expectedPath := filepath.Join(home, ".wazeos", "data")
	assert.Equal(t, expectedPath, pm.dataPath)
	assert.Equal(t, filepath.Join(expectedPath, "apps"), pm.appsPath)
	assert.Equal(t, filepath.Join(expectedPath, "drivers"), pm.driversPath)

	// Clean up
	os.RemoveAll("./data")
}

func TestPackageManager_Install_Success(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := &mockRuntimeExec{}

	pm, err := NewPackageManager(tmpDir, runtime)
	require.NoError(t, err)

	metadata := &types.AppMetadata{
		Name:        "test-app",
		Version:     "1.0.0",
		Author:      "test-author",
		Description: "Test application",
		Entrypoint:  "_start",
	}

	wasmData := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00} // WASM magic

	zipData, err := createTestZip(metadata, wasmData)
	require.NoError(t, err)

	ctx := context.Background()
	installed, err := pm.Install(ctx, zipData)
	assert.NoError(t, err)
	assert.NotNil(t, installed)
	assert.Equal(t, "test-app", installed.Name)
	assert.Equal(t, "1.0.0", installed.Version)
	assert.Equal(t, "test-author", installed.Author)

	// Verify files were written using new structure: apps/{author}/{name}/{version}/
	appID := metadata.AppID()
	appDir := filepath.Join(tmpDir, "apps", metadata.Author, metadata.Name, metadata.Version)
	_, err = os.Stat(filepath.Join(appDir, "metadata.json"))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(appDir, "app.wasm"))
	assert.NoError(t, err)

	// Verify app was loaded into runtime
	assert.Contains(t, runtime.loadedApps, appID)
}

func TestPackageManager_Install_EmptyZip(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := &mockRuntimeExec{}

	pm, err := NewPackageManager(tmpDir, runtime)
	require.NoError(t, err)

	ctx := context.Background()
	installed, err := pm.Install(ctx, []byte{})
	assert.Error(t, err)
	assert.Nil(t, installed)
	assert.True(t, types.IsInvalidRequest(err))
}

func TestPackageManager_Install_InvalidZip(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := &mockRuntimeExec{}

	pm, err := NewPackageManager(tmpDir, runtime)
	require.NoError(t, err)

	ctx := context.Background()
	installed, err := pm.Install(ctx, []byte("not a zip file"))
	assert.Error(t, err)
	assert.Nil(t, installed)
}

func TestPackageManager_Install_MissingMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := &mockRuntimeExec{}

	pm, err := NewPackageManager(tmpDir, runtime)
	require.NoError(t, err)

	wasmData := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}

	// Create ZIP without metadata.json
	// With the new design, metadata will be extracted from WASM via GetMetadata()
	zipData, err := createTestZip(nil, wasmData)
	require.NoError(t, err)

	ctx := context.Background()
	installed, err := pm.Install(ctx, zipData)
	// Should succeed because GetMetadata() provides fallback metadata
	assert.NoError(t, err)
	assert.NotNil(t, installed)
	assert.Equal(t, "test-app", installed.Name)
}

func TestPackageManager_Install_MissingWasm(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := &mockRuntimeExec{}

	pm, err := NewPackageManager(tmpDir, runtime)
	require.NoError(t, err)

	metadata := &types.AppMetadata{
		Name:    "test-app",
		Version: "1.0.0",
		Author:  "test-author",
	}

	// Create ZIP without WASM
	zipData, err := createTestZip(metadata, nil)
	require.NoError(t, err)

	ctx := context.Background()
	installed, err := pm.Install(ctx, zipData)
	assert.Error(t, err)
	assert.Nil(t, installed)
	assert.Contains(t, err.Error(), "app.wasm not found")
}

func TestPackageManager_Install_InvalidMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := &mockRuntimeExec{}

	pm, err := NewPackageManager(tmpDir, runtime)
	require.NoError(t, err)

	tests := []struct {
		name     string
		metadata *types.AppMetadata
		errMsg   string
	}{
		{
			name: "missing name",
			metadata: &types.AppMetadata{
				Version: "1.0.0",
				Author:  "author",
			},
			errMsg: "app name is required",
		},
		{
			name: "missing version",
			metadata: &types.AppMetadata{
				Name:   "app",
				Author: "author",
			},
			errMsg: "app version is required",
		},
		{
			name: "missing author",
			metadata: &types.AppMetadata{
				Name:    "app",
				Version: "1.0.0",
			},
			errMsg: "app author is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wasmData := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
			zipData, err := createTestZip(tt.metadata, wasmData)
			require.NoError(t, err)

			ctx := context.Background()
			installed, err := pm.Install(ctx, zipData)
			assert.Error(t, err)
			assert.Nil(t, installed)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestPackageManager_Install_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := &mockRuntimeExec{}

	pm, err := NewPackageManager(tmpDir, runtime)
	require.NoError(t, err)

	metadata := &types.AppMetadata{
		Name:    "test-app",
		Version: "1.0.0",
		Author:  "author",
	}

	wasmData := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	zipData, err := createTestZip(metadata, wasmData)
	require.NoError(t, err)

	ctx := context.Background()

	// Install first time
	_, err = pm.Install(ctx, zipData)
	require.NoError(t, err)

	// Try to install again
	installed, err := pm.Install(ctx, zipData)
	assert.Error(t, err)
	assert.Nil(t, installed)
	assert.True(t, types.IsAlreadyExists(err))
}

func TestPackageManager_Install_MissingDependency(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := &mockRuntimeExec{}

	pm, err := NewPackageManager(tmpDir, runtime)
	require.NoError(t, err)

	metadata := &types.AppMetadata{
		Name:    "test-app",
		Version: "1.0.0",
		Author:  "author",
		DependenciesV2: &types.PackageDependencies{
			Apps: map[string]string{"other-app": "1.0.0"},
		},
	}

	wasmData := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	zipData, err := createTestZip(metadata, wasmData)
	require.NoError(t, err)

	ctx := context.Background()
	installed, err := pm.Install(ctx, zipData)
	assert.Error(t, err)
	assert.Nil(t, installed)
	assert.True(t, types.IsDependencyNotFound(err))
}

func TestPackageManager_Install_WithDependency(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := &mockRuntimeExec{}

	pm, err := NewPackageManager(tmpDir, runtime)
	require.NoError(t, err)

	ctx := context.Background()

	// Install dependency first
	depMetadata := &types.AppMetadata{
		Name:    "dep-app",
		Version: "1.0.0",
		Author:  "author",
	}

	wasmData := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	depZip, err := createTestZip(depMetadata, wasmData)
	require.NoError(t, err)

	_, err = pm.Install(ctx, depZip)
	require.NoError(t, err)

	// Install app with dependency
	appMetadata := &types.AppMetadata{
		Name:    "test-app",
		Version: "1.0.0",
		Author:  "author",
		DependenciesV2: &types.PackageDependencies{
			Apps: map[string]string{depMetadata.Name: depMetadata.Version},
		},
	}

	appZip, err := createTestZip(appMetadata, wasmData)
	require.NoError(t, err)

	installed, err := pm.Install(ctx, appZip)
	assert.NoError(t, err)
	assert.NotNil(t, installed)
}

func TestPackageManager_Install_RuntimeLoadFailure(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := &mockRuntimeExec{
		loadError: assert.AnError,
	}

	pm, err := NewPackageManager(tmpDir, runtime)
	require.NoError(t, err)

	metadata := &types.AppMetadata{
		Name:    "test-app",
		Version: "1.0.0",
		Author:  "author",
	}

	wasmData := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	zipData, err := createTestZip(metadata, wasmData)
	require.NoError(t, err)

	ctx := context.Background()
	installed, err := pm.Install(ctx, zipData)
	assert.Error(t, err)
	assert.Nil(t, installed)
	assert.Contains(t, err.Error(), "failed to load app into runtime")

	// Verify rollback - app should not be in memory
	pm.mu.RLock()
	_, exists := pm.apps[metadata.AppID()]
	pm.mu.RUnlock()
	assert.False(t, exists)
}

func TestPackageManager_Uninstall_Success(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := &mockRuntimeExec{}

	pm, err := NewPackageManager(tmpDir, runtime)
	require.NoError(t, err)

	metadata := &types.AppMetadata{
		Name:    "test-app",
		Version: "1.0.0",
		Author:  "author",
	}

	wasmData := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	zipData, err := createTestZip(metadata, wasmData)
	require.NoError(t, err)

	ctx := context.Background()

	// Install
	installed, err := pm.Install(ctx, zipData)
	require.NoError(t, err)

	appID := installed.AppID()

	// Uninstall
	err = pm.Uninstall(ctx, appID)
	assert.NoError(t, err)

	// Verify it's gone
	_, err = pm.Get(ctx, appID)
	assert.Error(t, err)
	assert.True(t, types.IsNotFound(err))

	// Verify directory was removed using new structure
	appDir := filepath.Join(tmpDir, "apps", installed.Author, installed.Name, installed.Version)
	_, err = os.Stat(appDir)
	assert.True(t, os.IsNotExist(err))

	// Verify runtime was unloaded
	assert.NotContains(t, runtime.loadedApps, appID)
}

func TestPackageManager_Uninstall_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := &mockRuntimeExec{}

	pm, err := NewPackageManager(tmpDir, runtime)
	require.NoError(t, err)

	ctx := context.Background()
	err = pm.Uninstall(ctx, "nonexistent/app_1.0.0")
	assert.Error(t, err)
	assert.True(t, types.IsNotFound(err))
}

func TestPackageManager_Uninstall_HasDependents(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := &mockRuntimeExec{}

	pm, err := NewPackageManager(tmpDir, runtime)
	require.NoError(t, err)

	ctx := context.Background()

	// Install base app
	baseMetadata := &types.AppMetadata{
		Name:    "base-app",
		Version: "1.0.0",
		Author:  "author",
	}

	wasmData := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	baseZip, err := createTestZip(baseMetadata, wasmData)
	require.NoError(t, err)

	base, err := pm.Install(ctx, baseZip)
	require.NoError(t, err)

	// Install dependent app
	depMetadata := &types.AppMetadata{
		Name:    "dependent-app",
		Version: "1.0.0",
		Author:  "author",
		DependenciesV2: &types.PackageDependencies{
			Apps: map[string]string{base.Name: base.Version},
		},
	}

	depZip, err := createTestZip(depMetadata, wasmData)
	require.NoError(t, err)

	_, err = pm.Install(ctx, depZip)
	require.NoError(t, err)

	// Try to uninstall base app (should fail)
	err = pm.Uninstall(ctx, base.AppID())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "depend on")
}

func TestPackageManager_List(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := &mockRuntimeExec{}

	pm, err := NewPackageManager(tmpDir, runtime)
	require.NoError(t, err)

	ctx := context.Background()

	// Initially empty
	apps, err := pm.List(ctx)
	assert.NoError(t, err)
	assert.Empty(t, apps)

	// Install some apps
	wasmData := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}

	for i := 1; i <= 3; i++ {
		metadata := &types.AppMetadata{
			Name:    fmt.Sprintf("app%d", i),
			Version: "1.0.0",
			Author:  "author",
		}

		zipData, err := createTestZip(metadata, wasmData)
		require.NoError(t, err)

		_, err = pm.Install(ctx, zipData)
		require.NoError(t, err)
	}

	// List should return all apps
	apps, err = pm.List(ctx)
	assert.NoError(t, err)
	assert.Len(t, apps, 3)
}

func TestPackageManager_Get_Success(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := &mockRuntimeExec{}

	pm, err := NewPackageManager(tmpDir, runtime)
	require.NoError(t, err)

	metadata := &types.AppMetadata{
		Name:    "test-app",
		Version: "1.0.0",
		Author:  "author",
	}

	wasmData := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	zipData, err := createTestZip(metadata, wasmData)
	require.NoError(t, err)

	ctx := context.Background()
	installed, err := pm.Install(ctx, zipData)
	require.NoError(t, err)

	// Get the app
	retrieved, err := pm.Get(ctx, installed.AppID())
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, "test-app", retrieved.Name)
	assert.Equal(t, "1.0.0", retrieved.Version)
}

func TestPackageManager_Get_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := &mockRuntimeExec{}

	pm, err := NewPackageManager(tmpDir, runtime)
	require.NoError(t, err)

	ctx := context.Background()
	retrieved, err := pm.Get(ctx, "nonexistent/app_1.0.0")
	assert.Error(t, err)
	assert.Nil(t, retrieved)
	assert.True(t, types.IsNotFound(err))
}

func TestPackageManager_Resolve_ByFullID(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := &mockRuntimeExec{}

	pm, err := NewPackageManager(tmpDir, runtime)
	require.NoError(t, err)

	metadata := &types.AppMetadata{
		Name:    "test-app",
		Version: "1.0.0",
		Author:  "author",
	}

	wasmData := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	zipData, err := createTestZip(metadata, wasmData)
	require.NoError(t, err)

	ctx := context.Background()
	installed, err := pm.Install(ctx, zipData)
	require.NoError(t, err)

	// Resolve by full ID
	resolved, err := pm.Resolve(ctx, installed.AppID())
	assert.NoError(t, err)
	assert.Equal(t, installed.AppID(), resolved)
}

func TestPackageManager_Resolve_ByName(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := &mockRuntimeExec{}

	pm, err := NewPackageManager(tmpDir, runtime)
	require.NoError(t, err)

	metadata := &types.AppMetadata{
		Name:    "test-app",
		Version: "1.0.0",
		Author:  "author",
	}

	wasmData := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	zipData, err := createTestZip(metadata, wasmData)
	require.NoError(t, err)

	ctx := context.Background()
	installed, err := pm.Install(ctx, zipData)
	require.NoError(t, err)

	// Resolve by author/name
	resolved, err := pm.Resolve(ctx, "author/test-app")
	assert.NoError(t, err)
	assert.Equal(t, installed.AppID(), resolved)
}

func TestPackageManager_Resolve_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := &mockRuntimeExec{}

	pm, err := NewPackageManager(tmpDir, runtime)
	require.NoError(t, err)

	ctx := context.Background()
	resolved, err := pm.Resolve(ctx, "author/nonexistent-app")
	assert.Error(t, err)
	assert.Empty(t, resolved)
	assert.True(t, types.IsNotFound(err))
}

func TestPackageManager_LoadExistingApps(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := &mockRuntimeExec{}

	// Create first package manager and install app
	pm1, err := NewPackageManager(tmpDir, runtime)
	require.NoError(t, err)

	metadata := &types.AppMetadata{
		Name:    "test-app",
		Version: "1.0.0",
		Author:  "author",
	}

	wasmData := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	zipData, err := createTestZip(metadata, wasmData)
	require.NoError(t, err)

	ctx := context.Background()
	installed, err := pm1.Install(ctx, zipData)
	require.NoError(t, err)

	// Create new package manager (should load existing apps)
	runtime2 := &mockRuntimeExec{}
	pm2, err := NewPackageManager(tmpDir, runtime2)
	require.NoError(t, err)

	// Verify app was loaded
	retrieved, err := pm2.Get(ctx, installed.AppID())
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, "test-app", retrieved.Name)

	// Verify runtime was loaded
	assert.Contains(t, runtime2.loadedApps, installed.AppID())
}

func TestPackageManager_GetWasmBinary(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := &mockRuntimeExec{}

	pm, err := NewPackageManager(tmpDir, runtime)
	require.NoError(t, err)

	metadata := &types.AppMetadata{
		Name:    "test-app",
		Version: "1.0.0",
		Author:  "author",
	}

	wasmData := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	zipData, err := createTestZip(metadata, wasmData)
	require.NoError(t, err)

	ctx := context.Background()
	installed, err := pm.Install(ctx, zipData)
	require.NoError(t, err)

	// Get WASM binary
	binary, err := pm.GetWasmBinary(installed.AppID())
	assert.NoError(t, err)
	assert.Equal(t, wasmData, binary)
}

func TestPackageManager_GetWasmBinary_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := &mockRuntimeExec{}

	pm, err := NewPackageManager(tmpDir, runtime)
	require.NoError(t, err)

	binary, err := pm.GetWasmBinary("nonexistent/app_1.0.0")
	assert.Error(t, err)
	assert.Nil(t, binary)
	assert.True(t, types.IsNotFound(err))
}

func TestPackageManager_SeparateAppsAndDrivers(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := &mockRuntimeExec{}

	pm, err := NewPackageManager(tmpDir, runtime)
	require.NoError(t, err)

	ctx := context.Background()
	wasmData := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}

	// Install an app
	appMetadata := &types.AppMetadata{
		Name:    "my-app",
		Version: "1.0.0",
		Author:  "testauthor",
		Type:    "app",
	}
	appZip, err := createTestZip(appMetadata, wasmData)
	require.NoError(t, err)
	installedApp, err := pm.Install(ctx, appZip)
	require.NoError(t, err)

	// Install a driver
	driverMetadata := &types.AppMetadata{
		Name:        "my-driver",
		Version:     "2.0.0",
		Author:      "driverauthor",
		Type:        "driver",
		DriverClass: "io.resource.custom",
	}
	driverZip, err := createTestZip(driverMetadata, wasmData)
	require.NoError(t, err)
	installedDriver, err := pm.Install(ctx, driverZip)
	require.NoError(t, err)

	// Verify app is in apps directory with structure: apps/{author}/{name}/{version}/
	appDir := filepath.Join(tmpDir, "apps", "testauthor", "my-app", "1.0.0")
	_, err = os.Stat(filepath.Join(appDir, "metadata.json"))
	assert.NoError(t, err, "App should be in apps directory")
	_, err = os.Stat(filepath.Join(appDir, "app.wasm"))
	assert.NoError(t, err)

	// Verify driver is in drivers directory with structure: drivers/{author}/{name}/{version}/
	driverDir := filepath.Join(tmpDir, "drivers", "driverauthor", "my-driver", "2.0.0")
	_, err = os.Stat(filepath.Join(driverDir, "metadata.json"))
	assert.NoError(t, err, "Driver should be in drivers directory")
	_, err = os.Stat(filepath.Join(driverDir, "app.wasm"))
	assert.NoError(t, err)

	// Verify both are loaded in memory
	apps, err := pm.List(ctx)
	assert.NoError(t, err)
	assert.Len(t, apps, 2)

	// Verify they can be retrieved
	retrievedApp, err := pm.Get(ctx, installedApp.AppID())
	assert.NoError(t, err)
	assert.Equal(t, "my-app", retrievedApp.Name)

	retrievedDriver, err := pm.Get(ctx, installedDriver.AppID())
	assert.NoError(t, err)
	assert.Equal(t, "my-driver", retrievedDriver.Name)
	assert.True(t, retrievedDriver.IsDriver())
}

func TestPackageManager_LoadExistingAppsAndDrivers(t *testing.T) {
	tmpDir := t.TempDir()
	runtime := &mockRuntimeExec{}

	// Create first package manager and install both app and driver
	pm1, err := NewPackageManager(tmpDir, runtime)
	require.NoError(t, err)

	ctx := context.Background()
	wasmData := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}

	// Install app
	appMetadata := &types.AppMetadata{
		Name:    "test-app",
		Version: "1.0.0",
		Author:  "author",
		Type:    "app",
	}
	appZip, err := createTestZip(appMetadata, wasmData)
	require.NoError(t, err)
	installedApp, err := pm1.Install(ctx, appZip)
	require.NoError(t, err)

	// Install driver
	driverMetadata := &types.AppMetadata{
		Name:        "test-driver",
		Version:     "1.0.0",
		Author:      "author",
		Type:        "driver",
		DriverClass: "io.resource.test",
	}
	driverZip, err := createTestZip(driverMetadata, wasmData)
	require.NoError(t, err)
	installedDriver, err := pm1.Install(ctx, driverZip)
	require.NoError(t, err)

	// Create new package manager (should load both apps and drivers)
	runtime2 := &mockRuntimeExec{}
	pm2, err := NewPackageManager(tmpDir, runtime2)
	require.NoError(t, err)

	// Verify both were loaded
	apps, err := pm2.List(ctx)
	assert.NoError(t, err)
	assert.Len(t, apps, 2)

	// Verify app was loaded
	retrievedApp, err := pm2.Get(ctx, installedApp.AppID())
	assert.NoError(t, err)
	assert.Equal(t, "test-app", retrievedApp.Name)
	assert.Contains(t, runtime2.loadedApps, installedApp.AppID())

	// Verify driver was loaded
	retrievedDriver, err := pm2.Get(ctx, installedDriver.AppID())
	assert.NoError(t, err)
	assert.Equal(t, "test-driver", retrievedDriver.Name)
	assert.True(t, retrievedDriver.IsDriver())
	assert.Contains(t, runtime2.loadedApps, installedDriver.AppID())
}
