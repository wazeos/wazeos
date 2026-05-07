package pkg

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/wazeos/wazeos/internal/types"
	"github.com/wazeos/wazeos/internal/validation"
)

// PackageChangeListener is notified when packages are installed or uninstalled.
type PackageChangeListener interface {
	OnPackageChanged()
}

// PackageManager implements types.PackageManager for app and driver lifecycle management.
// Both apps and drivers are stored together and can depend on each other.
type PackageManager struct {
	mu          sync.RWMutex
	dataPath    string                          // Base data directory (e.g., ./data)
	appsPath    string                          // Apps directory (e.g., ./data/apps)
	driversPath string                          // Drivers directory (e.g., ./data/drivers)
	apps        map[string]*types.AppMetadata   // appID -> metadata (includes both apps and drivers)
	wasmData    map[string][]byte               // appID -> wasm binary (includes both apps and drivers)
	runtime     types.RuntimeExec               // Runtime for loading apps
	watcher     *fsnotify.Watcher               // File system watcher for hot-reload
	stopChan    chan struct{}                   // Signal to stop watching
	watching    bool                            // Whether watching is active
	listeners   []PackageChangeListener         // Observers to notify on package changes
}

// NewPackageManager creates a new package manager.
func NewPackageManager(dataPath string, runtime types.RuntimeExec) (*PackageManager, error) {
	if dataPath == "" {
		// Use default: $HOME/.wazeos/data
		home, err := os.UserHomeDir()
		if err != nil {
			dataPath = "./data" // Fallback
		} else {
			dataPath = filepath.Join(home, ".wazeos", "data")
		}
	}

	appsPath := filepath.Join(dataPath, "apps")
	driversPath := filepath.Join(dataPath, "drivers")

	// Create directories if they don't exist
	if err := os.MkdirAll(appsPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create apps directory: %w", err)
	}
	if err := os.MkdirAll(driversPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create drivers directory: %w", err)
	}

	pm := &PackageManager{
		dataPath:    dataPath,
		appsPath:    appsPath,
		driversPath: driversPath,
		apps:        make(map[string]*types.AppMetadata),
		wasmData:    make(map[string][]byte),
		runtime:     runtime,
		stopChan:    make(chan struct{}),
		watching:    false,
	}

	// Load existing apps and drivers from disk
	if err := pm.loadExistingApps(); err != nil {
		return nil, fmt.Errorf("failed to load existing packages: %w", err)
	}

	// Start hot-reload watcher
	if err := pm.StartWatching(); err != nil {
		// Log warning but don't fail - hot-reload is optional
		fmt.Fprintf(os.Stderr, "Warning: failed to start hot-reload watcher: %v\n", err)
	}

	return pm, nil
}

// Name returns the driver class.
func (pm *PackageManager) Name() string {
	return "kernel.pkg"
}

// AddChangeListener registers a listener to be notified of package changes.
func (pm *PackageManager) AddChangeListener(listener PackageChangeListener) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.listeners = append(pm.listeners, listener)
}

// notifyListeners notifies all registered listeners of a package change.
func (pm *PackageManager) notifyListeners() {
	pm.mu.RLock()
	listeners := pm.listeners
	pm.mu.RUnlock()

	// Notify asynchronously to avoid blocking
	go func() {
		for _, listener := range listeners {
			listener.OnPackageChanged()
		}
	}()
}

// Install installs an app from a zip file.
func (pm *PackageManager) Install(ctx context.Context, zipData []byte) (*types.AppMetadata, error) {
	if len(zipData) == 0 {
		return nil, fmt.Errorf("empty zip data: %w", types.ErrInvalidRequest)
	}

	// Open zip archive
	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("failed to open zip: %w", err)
	}

	// Extract files from ZIP
	var metadata *types.AppMetadata
	var wasmBinary []byte

	for _, file := range zipReader.File {
		switch file.Name {
		case "metadata.json":
			// Optional: support legacy metadata.json format
			rc, err := file.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to open metadata.json: %w", err)
			}
			defer rc.Close()

			data, err := io.ReadAll(rc)
			if err != nil {
				return nil, fmt.Errorf("failed to read metadata.json: %w", err)
			}

			metadata = &types.AppMetadata{}
			if err := json.Unmarshal(data, metadata); err != nil {
				return nil, fmt.Errorf("failed to parse metadata.json: %w", err)
			}

		case "app.wasm":
			rc, err := file.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to open app.wasm: %w", err)
			}
			defer rc.Close()

			wasmBinary, err = io.ReadAll(rc)
			if err != nil {
				return nil, fmt.Errorf("failed to read app.wasm: %w", err)
			}
		}
	}

	// Validate required files
	if wasmBinary == nil {
		return nil, fmt.Errorf("app.wasm not found in package: %w", types.ErrInvalidRequest)
	}

	// Get metadata: either from metadata.json (legacy) or from WASM itself
	if metadata == nil {
		// Call wazeos_metadata() function in WASM to get self-describing metadata
		var err error
		metadata, err = pm.runtime.GetMetadata(ctx, wasmBinary)
		if err != nil {
			return nil, fmt.Errorf("failed to load metadata from WASM: %w", err)
		}
	}

	// Validate metadata fields (GetMetadata already validates these, but check again for legacy)
	if metadata.Name == "" {
		return nil, fmt.Errorf("app name is required: %w", types.ErrInvalidRequest)
	}
	if metadata.Version == "" {
		return nil, fmt.Errorf("app version is required: %w", types.ErrInvalidRequest)
	}
	if metadata.Author == "" {
		return nil, fmt.Errorf("app author is required: %w", types.ErrInvalidRequest)
	}

	// Validate input schema if present
	if metadata.InputSchema != nil {
		if err := validation.ValidateJSONSchema([]byte(*metadata.InputSchema)); err != nil {
			return nil, fmt.Errorf("invalid input schema: %w", err)
		}
	}

	appID := metadata.AppID()

	// Check if app already exists
	pm.mu.RLock()
	_, exists := pm.apps[appID]
	pm.mu.RUnlock()

	if exists {
		return nil, fmt.Errorf("app %s already installed: %w", appID, types.ErrAlreadyExists)
	}

	// Install prerequisites recursively if any are missing
	if err := pm.installPrerequisites(ctx, metadata.GetAllPrerequisites(), make(map[string]bool)); err != nil {
		return nil, fmt.Errorf("failed to install prerequisites: %w", err)
	}

	// Check dependencies
	for _, depID := range metadata.GetAllDependencies() {
		pm.mu.RLock()
		_, depExists := pm.apps[depID]
		pm.mu.RUnlock()

		if !depExists {
			return nil, fmt.Errorf("dependency %s not found: %w", depID, types.ErrDependencyNotFound)
		}
	}

	// Create app/driver directory with structure: {author}/{name}/{version}/
	basePath := pm.appsPath
	if metadata.IsDriver() {
		basePath = pm.driversPath
	}
	appDir := filepath.Join(basePath, metadata.Author, metadata.Name, metadata.Version)
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create package directory: %w", err)
	}

	// Write metadata to disk
	metadataPath := filepath.Join(appDir, "metadata.json")
	metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if err := os.WriteFile(metadataPath, metadataJSON, 0644); err != nil {
		return nil, fmt.Errorf("failed to write metadata: %w", err)
	}

	// Write WASM binary to disk
	wasmPath := filepath.Join(appDir, "app.wasm")
	if err := os.WriteFile(wasmPath, wasmBinary, 0644); err != nil {
		return nil, fmt.Errorf("failed to write wasm: %w", err)
	}

	// Store in memory
	pm.mu.Lock()
	pm.apps[appID] = metadata
	pm.wasmData[appID] = wasmBinary
	pm.mu.Unlock()

	// Load into runtime
	if pm.runtime != nil {
		var err error
		if metadata.IsDriver() {
			// Load as driver with metadata for permission checking
			err = pm.runtime.LoadDriver(ctx, appID, wasmBinary, metadata)
		} else {
			// Load as regular app
			err = pm.runtime.LoadApp(ctx, appID, wasmBinary)
		}

		if err != nil {
			// Rollback installation
			pm.mu.Lock()
			delete(pm.apps, appID)
			delete(pm.wasmData, appID)
			pm.mu.Unlock()
			os.RemoveAll(appDir)

			if metadata.IsDriver() {
				return nil, fmt.Errorf("failed to load driver into runtime: %w", err)
			}
			return nil, fmt.Errorf("failed to load app into runtime: %w", err)
		}
	}

	// Notify listeners of package change
	pm.notifyListeners()

	return metadata, nil
}

// installPrerequisites recursively installs missing prerequisites.
// Prerequisites can be either apps or drivers - the system automatically detects the type.
// The installing map tracks packages currently being installed to detect circular dependencies.
//
// Example metadata showing an app depending on both another app and a driver:
//   {
//     "name": "myapp",
//     "prerequisites": [
//       "wazeos/logger:1.0.0",      // An app for logging
//       "wazeos/s3-driver:2.0.0"    // A driver for S3 access
//     ]
//   }
func (pm *PackageManager) installPrerequisites(ctx context.Context, prerequisites []string, installing map[string]bool) error {
	for _, prereqID := range prerequisites {
		// Check if prerequisite is already installed
		pm.mu.RLock()
		_, exists := pm.apps[prereqID]
		pm.mu.RUnlock()

		if exists {
			// Already installed, skip
			continue
		}

		// Check for circular dependency
		if installing[prereqID] {
			return fmt.Errorf("circular prerequisite dependency detected: %s", prereqID)
		}

		// Mark as installing
		installing[prereqID] = true

		// Download and install the prerequisite
		fmt.Fprintf(os.Stderr, "→ Installing prerequisite: %s\n", prereqID)

		if err := pm.installFromPackageID(ctx, prereqID, installing); err != nil {
			return fmt.Errorf("failed to install prerequisite %s: %w", prereqID, err)
		}

		fmt.Fprintf(os.Stderr, "  ✓ Installed %s\n", prereqID)

		// Unmark after successful installation
		delete(installing, prereqID)
	}

	return nil
}

// installFromPackageID downloads and installs a package from its ID (e.g., "wazeos/echo:1.0.0").
func (pm *PackageManager) installFromPackageID(ctx context.Context, packageID string, installing map[string]bool) error {
	// Resolve package URL
	url, err := pm.resolvePackageURL(packageID)
	if err != nil {
		return fmt.Errorf("failed to resolve package URL: %w", err)
	}

	// Download package
	zipData, err := pm.downloadPackage(url)
	if err != nil {
		return fmt.Errorf("failed to download package: %w", err)
	}

	// Install package (this will recursively install its prerequisites)
	_, err = pm.Install(ctx, zipData)
	return err
}

// resolvePackageURL converts a package ID to a download URL.
// Automatically detects whether the package is an app or driver by checking both repositories.
// Supports formats:
//   - "author/name:version" -> GitHub packages URL (checks both apps/ and drivers/)
//   - "author/name" -> Latest version from GitHub packages (tries apps first, then drivers)
func (pm *PackageManager) resolvePackageURL(packageID string) (string, error) {
	// Parse author/package:version format
	parts := strings.SplitN(packageID, ":", 2)
	nameAndAuthor := parts[0]
	version := ""
	if len(parts) == 2 {
		version = parts[1]
	}

	// Split author/name
	nameParts := strings.SplitN(nameAndAuthor, "/", 2)
	if len(nameParts) != 2 {
		return "", fmt.Errorf("invalid package ID format, expected author/name:version, got: %s", packageID)
	}
	author := nameParts[0]
	name := nameParts[1]

	// Determine if this is an app or driver by trying both
	// Try apps first
	if version == "" {
		versions, err := pm.listVersions("apps", author, name)
		if err == nil && len(versions) > 0 {
			version = pm.findHighestVersion(versions)
		} else {
			// Try drivers
			versions, err = pm.listVersions("drivers", author, name)
			if err == nil && len(versions) > 0 {
				version = pm.findHighestVersion(versions)
			} else {
				return "", fmt.Errorf("package not found: %s", packageID)
			}
		}
	}

	// Try apps URL first, then drivers
	appsURL := fmt.Sprintf("https://github.com/wazeos/packages/raw/main/apps/%s/%s/%s.zip", author, name, version)
	driversURL := fmt.Sprintf("https://github.com/wazeos/packages/raw/main/drivers/%s/%s/%s.zip", author, name, version)

	// Check which one exists (try HEAD request)
	if pm.urlExists(appsURL) {
		return appsURL, nil
	}
	if pm.urlExists(driversURL) {
		return driversURL, nil
	}

	// Default to apps URL if we can't determine
	return appsURL, nil
}

// downloadPackage downloads a package from a URL and returns the ZIP data.
func (pm *PackageManager) downloadPackage(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status: %d %s", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return data, nil
}

// urlExists checks if a URL returns 200 OK with a HEAD request.
func (pm *PackageManager) urlExists(url string) bool {
	resp, err := http.Head(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// listVersions lists available versions for a package type from GitHub.
func (pm *PackageManager) listVersions(packageType, author, name string) ([]string, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/wazeos/packages/contents/%s/%s/%s", packageType, author, name)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var items []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}

	var versions []string
	for _, item := range items {
		if item.Type == "file" && strings.HasSuffix(item.Name, ".zip") {
			version := strings.TrimSuffix(item.Name, ".zip")
			versions = append(versions, version)
		}
	}

	return versions, nil
}

// findHighestVersion finds the highest semantic version from a list.
func (pm *PackageManager) findHighestVersion(versions []string) string {
	if len(versions) == 0 {
		return "latest"
	}

	highest := versions[0]
	highestSemVer := pm.parseSemVer(highest)

	for _, v := range versions[1:] {
		semVer := pm.parseSemVer(v)
		if pm.compareSemVer(semVer, highestSemVer) > 0 {
			highest = v
			highestSemVer = semVer
		}
	}

	return highest
}

type semVer struct {
	major      int
	minor      int
	patch      int
	prerelease string
	original   string
}

func (pm *PackageManager) parseSemVer(v string) semVer {
	sv := semVer{original: v}

	if v == "latest" {
		sv.major = 999999
		return sv
	}

	v = strings.TrimPrefix(v, "v")

	re := regexp.MustCompile(`^(\d+)(?:\.(\d+))?(?:\.(\d+))?(?:-(.+))?$`)
	matches := re.FindStringSubmatch(v)

	if matches == nil {
		return sv
	}

	sv.major, _ = strconv.Atoi(matches[1])
	if matches[2] != "" {
		sv.minor, _ = strconv.Atoi(matches[2])
	}
	if matches[3] != "" {
		sv.patch, _ = strconv.Atoi(matches[3])
	}
	if matches[4] != "" {
		sv.prerelease = matches[4]
	}

	return sv
}

func (pm *PackageManager) compareSemVer(a, b semVer) int {
	if a.major != b.major {
		if a.major > b.major {
			return 1
		}
		return -1
	}

	if a.minor != b.minor {
		if a.minor > b.minor {
			return 1
		}
		return -1
	}

	if a.patch != b.patch {
		if a.patch > b.patch {
			return 1
		}
		return -1
	}

	if a.prerelease == "" && b.prerelease != "" {
		return 1
	}
	if a.prerelease != "" && b.prerelease == "" {
		return -1
	}

	if a.prerelease != b.prerelease {
		if a.prerelease > b.prerelease {
			return 1
		}
		return -1
	}

	return 0
}

// getPackagePath returns the filesystem path for a package based on its metadata.
func (pm *PackageManager) getPackagePath(metadata *types.AppMetadata) string {
	basePath := pm.appsPath
	if metadata.IsDriver() {
		basePath = pm.driversPath
	}
	return filepath.Join(basePath, metadata.Author, metadata.Name, metadata.Version)
}

// Uninstall removes an installed app.
// If other packages depend on this app, the uninstall is aborted and a dependency tree is shown.
// After successful uninstall, unused dependencies/prerequisites are automatically removed.
func (pm *PackageManager) Uninstall(ctx context.Context, appID string) error {
	pm.mu.RLock()
	metadata, exists := pm.apps[appID]
	pm.mu.RUnlock()

	if !exists {
		return types.ErrNotFound
	}

	// Check if any other packages depend on this one
	dependents := pm.findDependents(appID)
	if len(dependents) > 0 {
		// Build and display dependency tree
		tree := pm.buildDependencyTree(appID, dependents, 0)
		return fmt.Errorf("cannot uninstall %s: other packages depend on it\n\nPackages that need to be uninstalled first:\n%s", appID, tree)
	}

	// Collect this package's dependencies/prerequisites for later cleanup
	var depsAndPrereqs []string
	depsAndPrereqs = append(depsAndPrereqs, metadata.GetAllDependencies()...)
	depsAndPrereqs = append(depsAndPrereqs, metadata.GetAllPrerequisites()...)

	// Unload from runtime
	if pm.runtime != nil {
		if err := pm.runtime.UnloadApp(ctx, appID); err != nil {
			// Log error but continue with uninstall
		}
	}

	// Remove from memory
	pm.mu.Lock()
	delete(pm.apps, appID)
	delete(pm.wasmData, appID)
	pm.mu.Unlock()

	// Remove from disk
	packageDir := pm.getPackagePath(metadata)
	if err := os.RemoveAll(packageDir); err != nil {
		return fmt.Errorf("failed to remove package directory: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✓ Uninstalled %s\n", appID)

	// Clean up unused dependencies/prerequisites
	pm.cleanupUnusedDependencies(ctx, depsAndPrereqs)

	// Notify listeners of package change
	pm.notifyListeners()

	return nil
}

// findDependents returns all packages that depend on or have the given package as a prerequisite.
func (pm *PackageManager) findDependents(targetID string) []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var dependents []string
	for id, meta := range pm.apps {
		if id == targetID {
			continue
		}

		// Check dependencies (supports both old and new format)
		for _, dep := range meta.GetAllDependencies() {
			if dep == targetID {
				dependents = append(dependents, id)
				break
			}
		}

		// Check prerequisites (supports both old and new format)
		for _, prereq := range meta.GetAllPrerequisites() {
			if prereq == targetID {
				dependents = append(dependents, id)
				break
			}
		}
	}

	return dependents
}

// buildDependencyTree builds a formatted dependency tree showing which packages depend on the target.
func (pm *PackageManager) buildDependencyTree(targetID string, dependents []string, depth int) string {
	indent := strings.Repeat("  ", depth)
	var tree strings.Builder

	for i, depID := range dependents {
		isLast := i == len(dependents)-1
		prefix := "├─"
		if isLast {
			prefix = "└─"
		}

		tree.WriteString(fmt.Sprintf("%s%s %s\n", indent, prefix, depID))

		// Recursively show packages that depend on this dependent
		subDependents := pm.findDependents(depID)
		if len(subDependents) > 0 {
			subIndent := "│ "
			if isLast {
				subIndent = "  "
			}
			subTree := pm.buildDependencyTree(depID, subDependents, depth+1)
			lines := strings.Split(strings.TrimRight(subTree, "\n"), "\n")
			for _, line := range lines {
				tree.WriteString(fmt.Sprintf("%s%s%s\n", indent, subIndent, line))
			}
		}
	}

	return tree.String()
}

// cleanupUnusedDependencies removes dependencies/prerequisites that are no longer needed.
func (pm *PackageManager) cleanupUnusedDependencies(ctx context.Context, candidates []string) {
	for _, candidate := range candidates {
		// Check if any other package uses this dependency
		if pm.isPackageUsed(candidate) {
			continue
		}

		// No other package uses it, safe to uninstall
		pm.mu.RLock()
		candidateMeta, exists := pm.apps[candidate]
		pm.mu.RUnlock()

		if !exists {
			// Already uninstalled or never installed
			continue
		}

		fmt.Fprintf(os.Stderr, "→ Removing unused dependency: %s\n", candidate)

		// Recursively uninstall (this will clean up its dependencies too)
		if err := pm.Uninstall(ctx, candidate); err != nil {
			// Log error but continue
			fmt.Fprintf(os.Stderr, "  Warning: failed to uninstall %s: %v\n", candidate, err)
		}

		// Note: Uninstall already prints "✓ Uninstalled" so we don't need to print it again
		_ = candidateMeta // Silence unused variable warning
	}
}

// isPackageUsed checks if any installed package depends on or has the target as a prerequisite.
func (pm *PackageManager) isPackageUsed(targetID string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, meta := range pm.apps {
		// Check dependencies (supports both old and new format)
		for _, dep := range meta.GetAllDependencies() {
			if dep == targetID {
				return true
			}
		}

		// Check prerequisites (supports both old and new format)
		for _, prereq := range meta.GetAllPrerequisites() {
			if prereq == targetID {
				return true
			}
		}
	}

	return false
}

// List returns metadata for all installed apps.
func (pm *PackageManager) List(ctx context.Context) ([]*types.AppMetadata, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	apps := make([]*types.AppMetadata, 0, len(pm.apps))
	for _, meta := range pm.apps {
		apps = append(apps, meta)
	}

	return apps, nil
}

// Get returns metadata for a specific app.
func (pm *PackageManager) Get(ctx context.Context, appID string) (*types.AppMetadata, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	meta, exists := pm.apps[appID]
	if !exists {
		return nil, types.ErrNotFound
	}

	return meta, nil
}

// Resolve resolves a package identifier to a full app ID.
// Supports two formats:
//   - Full ID: "author/name:version" - exact match
//   - Partial ID: "author/name" - matches author and name, returns any available version
// Both author and name are required.
func (pm *PackageManager) Resolve(ctx context.Context, nameOrID string) (string, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// If nameOrID is already a full ID (author/name:version), check if it exists
	if strings.Contains(nameOrID, ":") {
		if _, exists := pm.apps[nameOrID]; exists {
			return nameOrID, nil
		}
		return "", types.ErrNotFound
	}

	// Parse input as "author/name"
	if !strings.Contains(nameOrID, "/") {
		return "", fmt.Errorf("invalid format: must be 'author/name' or 'author/name:version'")
	}

	parts := strings.SplitN(nameOrID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("invalid format: must be 'author/name' or 'author/name:version'")
	}

	targetAuthor := parts[0]
	targetName := parts[1]

	// Find all apps matching author and name
	var matches []string
	for appID, meta := range pm.apps {
		if meta.Author == targetAuthor && meta.Name == targetName {
			matches = append(matches, appID)
		}
	}

	if len(matches) == 0 {
		return "", types.ErrNotFound
	}

	// For MVP, just return the first match
	// In production, would parse versions and return latest
	return matches[0], nil
}

// loadExistingApps loads apps and drivers from disk into memory.
func (pm *PackageManager) loadExistingApps() error {
	// Load from both apps and drivers directories
	if err := pm.loadFromDirectory(pm.appsPath); err != nil {
		return fmt.Errorf("failed to load apps: %w", err)
	}
	if err := pm.loadFromDirectory(pm.driversPath); err != nil {
		return fmt.Errorf("failed to load drivers: %w", err)
	}
	return nil
}

// loadFromDirectory scans a directory tree for packages with structure: {author}/{name}/{version}/
func (pm *PackageManager) loadFromDirectory(basePath string) error {
	// Check if directory exists
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return nil // Directory doesn't exist yet, no packages to load
	}

	// Walk the directory tree: {author}/{name}/{version}/
	return filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip if not a directory
		if !d.IsDir() {
			return nil
		}

		// Check if this directory contains metadata.json
		metadataPath := filepath.Join(path, "metadata.json")
		if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
			return nil // Not a package directory, continue walking
		}

		// Found a package directory - load it
		metadataData, err := os.ReadFile(metadataPath)
		if err != nil {
			return nil // Skip invalid packages
		}

		metadata := &types.AppMetadata{}
		if err := json.Unmarshal(metadataData, metadata); err != nil {
			return nil // Skip invalid metadata
		}

		// Get actual appID from metadata
		appID := metadata.AppID()

		// Read WASM binary
		wasmPath := filepath.Join(path, "app.wasm")
		wasmData, err := os.ReadFile(wasmPath)
		if err != nil {
			return nil // Skip packages without WASM
		}

		// Store in memory
		pm.apps[appID] = metadata
		pm.wasmData[appID] = wasmData

		// Load into runtime if available
		if pm.runtime != nil {
			if metadata.IsDriver() {
				pm.runtime.LoadDriver(context.Background(), appID, wasmData, metadata)
			} else {
				pm.runtime.LoadApp(context.Background(), appID, wasmData)
			}
		}

		// Don't recurse into package directories
		return filepath.SkipDir
	})
}

// GetWasmBinary returns the WASM binary for an app (for testing).
func (pm *PackageManager) GetWasmBinary(appID string) ([]byte, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	data, exists := pm.wasmData[appID]
	if !exists {
		return nil, types.ErrNotFound
	}

	return data, nil
}

// StartWatching starts the file system watcher for hot-reload.
func (pm *PackageManager) StartWatching() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.watching {
		return nil // Already watching
	}

	// Create file system watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	pm.watcher = watcher
	pm.watching = true

	// Watch apps and drivers directories recursively
	if err := pm.addWatchRecursive(pm.appsPath); err != nil {
		pm.watcher.Close()
		pm.watching = false
		return fmt.Errorf("failed to watch apps directory: %w", err)
	}
	if err := pm.addWatchRecursive(pm.driversPath); err != nil {
		pm.watcher.Close()
		pm.watching = false
		return fmt.Errorf("failed to watch drivers directory: %w", err)
	}

	// Start watching in background goroutine
	go pm.watchLoop()

	return nil
}

// addWatchRecursive adds a directory and all its subdirectories to the watcher.
func (pm *PackageManager) addWatchRecursive(path string) error {
	// Check if directory exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // Directory doesn't exist yet
	}

	// Add the directory itself
	if err := pm.watcher.Add(path); err != nil {
		return err
	}

	// Walk subdirectories
	return filepath.WalkDir(path, func(walkPath string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if d.IsDir() && walkPath != path {
			// Add subdirectory to watcher
			if err := pm.watcher.Add(walkPath); err != nil {
				return nil // Skip errors
			}
		}
		return nil
	})
}

// watchLoop monitors file system events and loads/unloads packages.
func (pm *PackageManager) watchLoop() {
	// Debounce timer to avoid processing rapid successive events
	debounceTimer := time.NewTimer(0)
	<-debounceTimer.C // Drain initial tick
	pendingLoads := make(map[string]bool)
	pendingUnloads := make(map[string]bool)

	for {
		select {
		case <-pm.stopChan:
			return

		case event, ok := <-pm.watcher.Events:
			if !ok {
				return
			}

			// Handle directory creation (new installations)
			if event.Op&fsnotify.Create == fsnotify.Create {
				// Check if it's a directory
				info, err := os.Stat(event.Name)
				if err == nil && info.IsDir() {
					// Add new directory to watcher
					pm.watcher.Add(event.Name)

					// Check if this is a package directory (contains metadata.json)
					metadataPath := filepath.Join(event.Name, "metadata.json")
					if _, err := os.Stat(metadataPath); err == nil {
						// Mark for loading with debounce
						pendingLoads[event.Name] = true
						debounceTimer.Reset(500 * time.Millisecond)
					}
				}
			}

			// Handle directory removal (uninstalls)
			if event.Op&fsnotify.Remove == fsnotify.Remove {
				// Mark for unloading with debounce
				pendingUnloads[event.Name] = true
				debounceTimer.Reset(500 * time.Millisecond)
			}

		case <-debounceTimer.C:
			// Process all pending unloads first
			for path := range pendingUnloads {
				pm.unloadPackageByPath(path)
			}
			pendingUnloads = make(map[string]bool)

			// Then process all pending loads
			for path := range pendingLoads {
				pm.loadPackage(path)
			}
			pendingLoads = make(map[string]bool)

		case err, ok := <-pm.watcher.Errors:
			if !ok {
				return
			}
			fmt.Fprintf(os.Stderr, "Watcher error: %v\n", err)
		}
	}
}

// loadPackage dynamically loads a single package from a directory.
func (pm *PackageManager) loadPackage(packagePath string) {
	// Read metadata
	metadataPath := filepath.Join(packagePath, "metadata.json")
	metadataData, err := os.ReadFile(metadataPath)
	if err != nil {
		return // Skip if metadata can't be read
	}

	metadata := &types.AppMetadata{}
	if err := json.Unmarshal(metadataData, metadata); err != nil {
		return // Skip invalid metadata
	}

	appID := metadata.AppID()

	// Check if already loaded
	pm.mu.RLock()
	_, exists := pm.apps[appID]
	pm.mu.RUnlock()

	if exists {
		return // Already loaded
	}

	// Read WASM binary
	wasmPath := filepath.Join(packagePath, "app.wasm")
	wasmData, err := os.ReadFile(wasmPath)
	if err != nil {
		return // Skip packages without WASM
	}

	// Store in memory
	pm.mu.Lock()
	pm.apps[appID] = metadata
	pm.wasmData[appID] = wasmData
	pm.mu.Unlock()

	// Load into runtime if available
	if pm.runtime != nil {
		ctx := context.Background()
		if metadata.IsDriver() {
			if err := pm.runtime.LoadDriver(ctx, appID, wasmData, metadata); err != nil {
				fmt.Fprintf(os.Stderr, "Hot-reload: failed to load driver %s: %v\n", appID, err)
				// Rollback
				pm.mu.Lock()
				delete(pm.apps, appID)
				delete(pm.wasmData, appID)
				pm.mu.Unlock()
				return
			}
		} else {
			if err := pm.runtime.LoadApp(ctx, appID, wasmData); err != nil {
				fmt.Fprintf(os.Stderr, "Hot-reload: failed to load app %s: %v\n", appID, err)
				// Rollback
				pm.mu.Lock()
				delete(pm.apps, appID)
				delete(pm.wasmData, appID)
				pm.mu.Unlock()
				return
			}
		}
	}

	pkgType := "app"
	if metadata.IsDriver() {
		pkgType = "driver"
	}
	fmt.Fprintf(os.Stderr, "Hot-reload: loaded %s %s\n", pkgType, appID)
}

// unloadPackageByPath dynamically unloads a package based on its directory path.
func (pm *PackageManager) unloadPackageByPath(packagePath string) {
	// Parse the path to extract author/name/version
	// Path format: {appsPath or driversPath}/author/name/version/
	var relPath string

	// Check if this is under apps or drivers
	if strings.HasPrefix(packagePath, pm.appsPath) {
		relPath = strings.TrimPrefix(packagePath, pm.appsPath)
	} else if strings.HasPrefix(packagePath, pm.driversPath) {
		relPath = strings.TrimPrefix(packagePath, pm.driversPath)
	} else {
		return // Not a package path we recognize
	}

	// Clean up the relative path
	relPath = strings.Trim(relPath, string(filepath.Separator))
	parts := strings.Split(relPath, string(filepath.Separator))

	// We expect at least 3 parts: author/name/version
	// But deletion events might come for any level
	if len(parts) < 3 {
		return
	}

	author := parts[0]
	name := parts[1]
	version := parts[2]

	// Construct appID
	appID := fmt.Sprintf("%s/%s:%s", author, name, version)

	// Check if this package exists
	pm.mu.RLock()
	metadata, exists := pm.apps[appID]
	pm.mu.RUnlock()

	if !exists {
		return // Package not loaded
	}

	// Unload from runtime
	if pm.runtime != nil {
		ctx := context.Background()
		if err := pm.runtime.UnloadApp(ctx, appID); err != nil {
			// Log error but continue with unload
			fmt.Fprintf(os.Stderr, "Hot-reload: error unloading from runtime %s: %v\n", appID, err)
		}
	}

	// Remove from memory
	pm.mu.Lock()
	delete(pm.apps, appID)
	delete(pm.wasmData, appID)
	pm.mu.Unlock()

	pkgType := "app"
	if metadata.IsDriver() {
		pkgType = "driver"
	}
	fmt.Fprintf(os.Stderr, "Hot-reload: unloaded %s %s\n", pkgType, appID)
}

// Stop stops the file system watcher and cleans up resources.
func (pm *PackageManager) Stop() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if !pm.watching {
		return nil
	}

	// Signal stop
	close(pm.stopChan)
	pm.watching = false

	// Close watcher
	if pm.watcher != nil {
		return pm.watcher.Close()
	}

	return nil
}
