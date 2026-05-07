package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wazeos/wazeos/internal/types"
)

// Mock package manager
type mockPkgManager struct {
	installFn   func(ctx context.Context, zipData []byte) (*types.AppMetadata, error)
	uninstallFn func(ctx context.Context, appID string) error
	listFn      func(ctx context.Context) ([]*types.AppMetadata, error)
	getFn       func(ctx context.Context, appID string) (*types.AppMetadata, error)
}

func (m *mockPkgManager) Name() string {
	return "mock.pkg"
}

func (m *mockPkgManager) Install(ctx context.Context, zipData []byte) (*types.AppMetadata, error) {
	if m.installFn != nil {
		return m.installFn(ctx, zipData)
	}
	return &types.AppMetadata{
		Name:    "test-app",
		Version: "1.0.0",
		Author:  "test",
	}, nil
}

func (m *mockPkgManager) Uninstall(ctx context.Context, appID string) error {
	if m.uninstallFn != nil {
		return m.uninstallFn(ctx, appID)
	}
	return nil
}

func (m *mockPkgManager) List(ctx context.Context) ([]*types.AppMetadata, error) {
	if m.listFn != nil {
		return m.listFn(ctx)
	}
	return []*types.AppMetadata{
		{Name: "app1", Version: "1.0.0", Author: "author1"},
		{Name: "app2", Version: "2.0.0", Author: "author2"},
	}, nil
}

func (m *mockPkgManager) Get(ctx context.Context, appID string) (*types.AppMetadata, error) {
	if m.getFn != nil {
		return m.getFn(ctx, appID)
	}
	return &types.AppMetadata{
		Name:    "test-app",
		Version: "1.0.0",
		Author:  "test",
	}, nil
}

func (m *mockPkgManager) Resolve(ctx context.Context, appName string) (string, error) {
	return "", nil
}

func (m *mockPkgManager) GetWasmBinary(appID string) ([]byte, error) {
	return []byte("mock wasm binary"), nil
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

// Helper to create test ZIP
func createTestZip() []byte {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	// Add metadata.json
	metadata := map[string]interface{}{
		"name":    "test-app",
		"version": "1.0.0",
		"author":  "test",
	}
	metadataJSON, _ := json.Marshal(metadata)
	f, _ := w.Create("metadata.json")
	f.Write(metadataJSON)

	// Add app.wasm
	f, _ = w.Create("app.wasm")
	f.Write([]byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00})

	w.Close()
	return buf.Bytes()
}

func TestNewManagementAPI(t *testing.T) {
	pkg := &mockPkgManager{}
	authn := []types.SecurityAuthn{&mockAuthn{}}

	api := NewManagementAPI(":9090", pkg, authn, nil)
	assert.NotNil(t, api)
	assert.Equal(t, ":9090", api.addr)
	assert.Equal(t, pkg, api.pkg)
	assert.Equal(t, authn, api.authn)
}

func TestNewManagementAPI_DefaultAddr(t *testing.T) {
	api := NewManagementAPI("", &mockPkgManager{}, nil, nil)
	assert.Equal(t, ":8081", api.addr)
}

func TestManagementAPI_Start_Success(t *testing.T) {
	api := NewManagementAPI(":0", &mockPkgManager{}, nil, nil)

	ctx := context.Background()
	err := api.Start(ctx)
	assert.NoError(t, err)
	assert.True(t, api.started)
	assert.NotEmpty(t, api.Addr())

	api.Stop(ctx)
}

func TestManagementAPI_Start_NoPkgManager(t *testing.T) {
	api := NewManagementAPI(":0", nil, nil, nil)

	ctx := context.Background()
	err := api.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "package manager not set")
}

func TestManagementAPI_Start_AlreadyStarted(t *testing.T) {
	api := NewManagementAPI(":0", &mockPkgManager{}, nil, nil)

	ctx := context.Background()
	err := api.Start(ctx)
	require.NoError(t, err)

	err = api.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already started")

	api.Stop(ctx)
}

func TestManagementAPI_Stop_Success(t *testing.T) {
	api := NewManagementAPI(":0", &mockPkgManager{}, nil, nil)

	ctx := context.Background()
	err := api.Start(ctx)
	require.NoError(t, err)

	err = api.Stop(ctx)
	assert.NoError(t, err)
	assert.False(t, api.started)
}

func TestManagementAPI_Stop_NotStarted(t *testing.T) {
	api := NewManagementAPI(":0", &mockPkgManager{}, nil, nil)

	ctx := context.Background()
	err := api.Stop(ctx)
	assert.NoError(t, err)
}

func TestManagementAPI_HealthEndpoint(t *testing.T) {
	api := NewManagementAPI(":0", &mockPkgManager{}, nil, nil)

	ctx := context.Background()
	err := api.Start(ctx)
	require.NoError(t, err)
	defer api.Stop(ctx)

	time.Sleep(50 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://%s/api/health", api.Addr()))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result APIResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.True(t, result.Success)

	data := result.Data.(map[string]interface{})
	assert.Equal(t, "ok", data["status"])
}

func TestManagementAPI_ListApps_Success(t *testing.T) {
	authn := []types.SecurityAuthn{&mockAuthn{}}
	pkg := &mockPkgManager{}

	api := NewManagementAPI(":0", pkg, authn, nil)

	ctx := context.Background()
	err := api.Start(ctx)
	require.NoError(t, err)
	defer api.Stop(ctx)

	time.Sleep(50 * time.Millisecond)

	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s/api/apps", api.Addr()), nil)
	req.Header.Set("Authorization", "Bearer token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result APIResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.True(t, result.Success)

	data := result.Data.(map[string]interface{})
	apps := data["apps"].([]interface{})
	assert.Len(t, apps, 2)
}

func TestManagementAPI_ListApps_Unauthorized(t *testing.T) {
	authn := []types.SecurityAuthn{
		&mockAuthn{
			authenticateFn: func(ctx context.Context, payload *types.AuthPayload) (string, error) {
				return "", types.ErrAbstain
			},
		},
	}
	pkg := &mockPkgManager{}

	api := NewManagementAPI(":0", pkg, authn, nil)

	ctx := context.Background()
	err := api.Start(ctx)
	require.NoError(t, err)
	defer api.Stop(ctx)

	time.Sleep(50 * time.Millisecond)

	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s/api/apps", api.Addr()), nil)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	var result APIResponse
	json.NewDecoder(resp.Body).Decode(&result)
	assert.False(t, result.Success)
	assert.NotNil(t, result.Error)
	assert.Equal(t, "auth_required", result.Error.Code)
}

func TestManagementAPI_GetApp_Success(t *testing.T) {
	authn := []types.SecurityAuthn{&mockAuthn{}}
	pkg := &mockPkgManager{}

	api := NewManagementAPI(":0", pkg, authn, nil)

	ctx := context.Background()
	err := api.Start(ctx)
	require.NoError(t, err)
	defer api.Stop(ctx)

	time.Sleep(50 * time.Millisecond)

	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s/api/apps/test/test-app_1.0.0", api.Addr()), nil)
	req.Header.Set("Authorization", "Bearer token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	bodyBytes, _ := io.ReadAll(resp.Body)

	var fullResult struct {
		Success bool `json:"success"`
		Data    struct {
			App types.AppMetadata `json:"app"`
		} `json:"data"`
	}
	err = json.Unmarshal(bodyBytes, &fullResult)
	require.NoError(t, err)
	assert.True(t, fullResult.Success)
	assert.Equal(t, "test-app", fullResult.Data.App.Name)
}

func TestManagementAPI_GetApp_NotFound(t *testing.T) {
	authn := []types.SecurityAuthn{&mockAuthn{}}
	pkg := &mockPkgManager{
		getFn: func(ctx context.Context, appID string) (*types.AppMetadata, error) {
			return nil, types.ErrNotFound
		},
	}

	api := NewManagementAPI(":0", pkg, authn, nil)

	ctx := context.Background()
	err := api.Start(ctx)
	require.NoError(t, err)
	defer api.Stop(ctx)

	time.Sleep(50 * time.Millisecond)

	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s/api/apps/nonexistent", api.Addr()), nil)
	req.Header.Set("Authorization", "Bearer token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	var result APIResponse
	json.NewDecoder(resp.Body).Decode(&result)
	assert.False(t, result.Success)
	assert.Equal(t, "app_not_found", result.Error.Code)
}

func TestManagementAPI_InstallApp_ZipUpload(t *testing.T) {
	authn := []types.SecurityAuthn{&mockAuthn{}}
	pkg := &mockPkgManager{
		installFn: func(ctx context.Context, zipData []byte) (*types.AppMetadata, error) {
			assert.Greater(t, len(zipData), 0)
			return &types.AppMetadata{
				Name:    "installed-app",
				Version: "1.0.0",
				Author:  "test",
			}, nil
		},
	}

	api := NewManagementAPI(":0", pkg, authn, nil)

	ctx := context.Background()
	err := api.Start(ctx)
	require.NoError(t, err)
	defer api.Stop(ctx)

	time.Sleep(50 * time.Millisecond)

	zipData := createTestZip()

	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/api/apps", api.Addr()), bytes.NewReader(zipData))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/zip")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	bodyBytes, _ := io.ReadAll(resp.Body)

	var fullResult struct {
		Success bool `json:"success"`
		Data    struct {
			App types.AppMetadata `json:"app"`
		} `json:"data"`
	}
	err = json.Unmarshal(bodyBytes, &fullResult)
	require.NoError(t, err)
	assert.True(t, fullResult.Success)
	assert.Equal(t, "installed-app", fullResult.Data.App.Name)
}

func TestManagementAPI_InstallApp_MultipartUpload(t *testing.T) {
	authn := []types.SecurityAuthn{&mockAuthn{}}
	pkg := &mockPkgManager{
		installFn: func(ctx context.Context, zipData []byte) (*types.AppMetadata, error) {
			assert.Greater(t, len(zipData), 0)
			return &types.AppMetadata{
				Name:    "installed-app",
				Version: "1.0.0",
				Author:  "test",
			}, nil
		},
	}

	api := NewManagementAPI(":0", pkg, authn, nil)

	ctx := context.Background()
	err := api.Start(ctx)
	require.NoError(t, err)
	defer api.Stop(ctx)

	time.Sleep(50 * time.Millisecond)

	zipData := createTestZip()

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "app.zip")
	io.Copy(part, bytes.NewReader(zipData))
	writer.Close()

	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/api/apps", api.Addr()), body)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result APIResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestManagementAPI_InstallApp_InvalidPackage(t *testing.T) {
	authn := []types.SecurityAuthn{&mockAuthn{}}
	pkg := &mockPkgManager{
		installFn: func(ctx context.Context, zipData []byte) (*types.AppMetadata, error) {
			return nil, fmt.Errorf("invalid package: %w", types.ErrInvalidRequest)
		},
	}

	api := NewManagementAPI(":0", pkg, authn, nil)

	ctx := context.Background()
	err := api.Start(ctx)
	require.NoError(t, err)
	defer api.Stop(ctx)

	time.Sleep(50 * time.Millisecond)

	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/api/apps", api.Addr()), bytes.NewReader([]byte("invalid")))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/zip")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var result APIResponse
	json.NewDecoder(resp.Body).Decode(&result)
	assert.False(t, result.Success)
	assert.Equal(t, "invalid_package", result.Error.Code)
}

func TestManagementAPI_InstallApp_AlreadyExists(t *testing.T) {
	authn := []types.SecurityAuthn{&mockAuthn{}}
	pkg := &mockPkgManager{
		installFn: func(ctx context.Context, zipData []byte) (*types.AppMetadata, error) {
			return nil, fmt.Errorf("app already exists: %w", types.ErrAlreadyExists)
		},
	}

	api := NewManagementAPI(":0", pkg, authn, nil)

	ctx := context.Background()
	err := api.Start(ctx)
	require.NoError(t, err)
	defer api.Stop(ctx)

	time.Sleep(50 * time.Millisecond)

	zipData := createTestZip()

	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/api/apps", api.Addr()), bytes.NewReader(zipData))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/zip")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode)

	var result APIResponse
	json.NewDecoder(resp.Body).Decode(&result)
	assert.False(t, result.Success)
	assert.Equal(t, "already_exists", result.Error.Code)
}

func TestManagementAPI_UninstallApp_Success(t *testing.T) {
	authn := []types.SecurityAuthn{&mockAuthn{}}
	pkg := &mockPkgManager{
		uninstallFn: func(ctx context.Context, appID string) error {
			assert.Equal(t, "test/test-app_1.0.0", appID)
			return nil
		},
	}

	api := NewManagementAPI(":0", pkg, authn, nil)

	ctx := context.Background()
	err := api.Start(ctx)
	require.NoError(t, err)
	defer api.Stop(ctx)

	time.Sleep(50 * time.Millisecond)

	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("http://%s/api/apps/test/test-app_1.0.0", api.Addr()), nil)
	req.Header.Set("Authorization", "Bearer token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result APIResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestManagementAPI_UninstallApp_NotFound(t *testing.T) {
	authn := []types.SecurityAuthn{&mockAuthn{}}
	pkg := &mockPkgManager{
		uninstallFn: func(ctx context.Context, appID string) error {
			return types.ErrNotFound
		},
	}

	api := NewManagementAPI(":0", pkg, authn, nil)

	ctx := context.Background()
	err := api.Start(ctx)
	require.NoError(t, err)
	defer api.Stop(ctx)

	time.Sleep(50 * time.Millisecond)

	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("http://%s/api/apps/nonexistent", api.Addr()), nil)
	req.Header.Set("Authorization", "Bearer token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	var result APIResponse
	json.NewDecoder(resp.Body).Decode(&result)
	assert.False(t, result.Success)
	assert.Equal(t, "app_not_found", result.Error.Code)
}

func TestManagementAPI_CORS(t *testing.T) {
	api := NewManagementAPI(":0", &mockPkgManager{}, nil, nil)

	ctx := context.Background()
	err := api.Start(ctx)
	require.NoError(t, err)
	defer api.Stop(ctx)

	time.Sleep(50 * time.Millisecond)

	req, _ := http.NewRequest(http.MethodOptions, fmt.Sprintf("http://%s/api/health", api.Addr()), nil)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
	assert.Contains(t, resp.Header.Get("Access-Control-Allow-Methods"), "GET")
	assert.Contains(t, resp.Header.Get("Access-Control-Allow-Methods"), "POST")
	assert.Contains(t, resp.Header.Get("Access-Control-Allow-Methods"), "DELETE")
}
