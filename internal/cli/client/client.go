package client

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"

	"github.com/wazeos/wazeos/internal/cli/secrets"
	"github.com/wazeos/wazeos/internal/drivers/kernel/pkg"
	"github.com/wazeos/wazeos/internal/types"
)

// Client provides access to WazeOS data (packages, secrets, etc.)
type Client interface {
	// Package operations
	InstallPackage(ctx context.Context, zipData []byte) (*types.AppMetadata, error)
	ListPackages(ctx context.Context) ([]*types.AppMetadata, error)
	GetPackage(ctx context.Context, appID string) (*types.AppMetadata, error)
	ResolvePackage(ctx context.Context, nameOrID string) (string, error)
	UninstallPackage(ctx context.Context, appID string) error
	ValidatePackage(ctx context.Context, zipData []byte) (*types.AppMetadata, error)

	// Secrets operations
	SetSecret(ctx context.Context, key string, value interface{}) error
	GetSecret(ctx context.Context, key string) (interface{}, error)
	ListSecrets(ctx context.Context) ([]string, error)
	DeleteSecret(ctx context.Context, key string) error
	MatchSecrets(ctx context.Context, prefix string) (map[string]interface{}, error)

	// Lifecycle
	Close() error
}

// DirectClient provides direct filesystem access to WazeOS data
type DirectClient struct {
	dataPath     string
	pkgManager   types.PackageManager
	secretsStore *secrets.FileStore
	lockFile     *flock.Flock
	locked       bool
}

// NewDirectClient creates a new direct access client
func NewDirectClient(dataPath string) (*DirectClient, error) {
	if dataPath == "" {
		// Use default: $HOME/.wazeos/data
		home, err := os.UserHomeDir()
		if err != nil {
			dataPath = "./data" // Fallback
		} else {
			dataPath = filepath.Join(home, ".wazeos", "data")
		}
	}

	// Make path absolute
	absPath, err := filepath.Abs(dataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve data path: %w", err)
	}

	// Ensure data directory exists
	if err := os.MkdirAll(absPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	client := &DirectClient{
		dataPath: absPath,
		lockFile: flock.New(filepath.Join(absPath, ".wazeos.lock")),
	}

	// Try to acquire lock with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	locked, err := client.lockFile.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}
	if !locked {
		return nil, fmt.Errorf("another wazeos instance is running (lock file: %s)", client.lockFile.Path())
	}
	client.locked = true

	// Create package manager
	pkgMgr, err := pkg.NewPackageManager(absPath, nil)
	if err != nil {
		client.lockFile.Unlock()
		return nil, fmt.Errorf("failed to create package manager: %w", err)
	}
	client.pkgManager = pkgMgr

	// Create secrets store
	secretsStore, err := secrets.NewFileStore(absPath)
	if err != nil {
		client.lockFile.Unlock()
		return nil, fmt.Errorf("failed to create secrets store: %w", err)
	}
	client.secretsStore = secretsStore

	return client, nil
}

// InstallPackage installs a package from ZIP data
func (c *DirectClient) InstallPackage(ctx context.Context, zipData []byte) (*types.AppMetadata, error) {
	return c.pkgManager.Install(ctx, zipData)
}

// ListPackages lists all installed packages
func (c *DirectClient) ListPackages(ctx context.Context) ([]*types.AppMetadata, error) {
	return c.pkgManager.List(ctx)
}

// GetPackage gets a specific package by ID
func (c *DirectClient) GetPackage(ctx context.Context, appID string) (*types.AppMetadata, error) {
	return c.pkgManager.Get(ctx, appID)
}

// ResolvePackage resolves a package name or partial ID to a full app ID
// If version not specified (e.g., "wazeos/myapp"), returns a matching version
// If full ID specified (e.g., "wazeos/myapp:1.0.0"), validates it exists
func (c *DirectClient) ResolvePackage(ctx context.Context, nameOrID string) (string, error) {
	return c.pkgManager.Resolve(ctx, nameOrID)
}

// UninstallPackage uninstalls a package
func (c *DirectClient) UninstallPackage(ctx context.Context, appID string) error {
	return c.pkgManager.Uninstall(ctx, appID)
}

// ValidatePackage validates a package without installing it
func (c *DirectClient) ValidatePackage(ctx context.Context, zipData []byte) (*types.AppMetadata, error) {
	// Use a temporary package manager that doesn't write to disk
	// For now, we'll just try to install and immediately uninstall
	// A better approach would be to add a Validate method to PackageManager
	metadata, err := c.pkgManager.Install(ctx, zipData)
	if err != nil {
		return nil, err
	}

	// Get the full app ID using the metadata method
	appID := metadata.AppID()

	// Uninstall immediately
	if err := c.pkgManager.Uninstall(ctx, appID); err != nil {
		// Log warning but don't fail
		fmt.Printf("Warning: failed to cleanup after validation: %v\n", err)
	}

	return metadata, nil
}

// SetSecret stores a secret value
func (c *DirectClient) SetSecret(ctx context.Context, key string, value interface{}) error {
	return c.secretsStore.Set(ctx, key, value)
}

// GetSecret retrieves a secret value
func (c *DirectClient) GetSecret(ctx context.Context, key string) (interface{}, error) {
	return c.secretsStore.Get(ctx, key)
}

// ListSecrets returns all secret keys
func (c *DirectClient) ListSecrets(ctx context.Context) ([]string, error) {
	return c.secretsStore.List(ctx)
}

// DeleteSecret removes a secret
func (c *DirectClient) DeleteSecret(ctx context.Context, key string) error {
	return c.secretsStore.Delete(ctx, key)
}

// MatchSecrets returns secrets matching a prefix
func (c *DirectClient) MatchSecrets(ctx context.Context, prefix string) (map[string]interface{}, error) {
	return c.secretsStore.Match(ctx, prefix)
}

// Close releases resources and removes the lock
func (c *DirectClient) Close() error {
	if c.locked {
		return c.lockFile.Unlock()
	}
	return nil
}
