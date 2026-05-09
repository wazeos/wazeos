package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/wazeos/wazeos/core/internal/pkg"
)

var appCmd = &cobra.Command{
	Use:   "app",
	Short: "Manage applications",
	Long:  `Create, build, test, install, and manage WazeOS applications.`,
}

var appNewCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new app",
	Long: `Create a new app scaffold with MCP tool boilerplate.

Examples:
  wazeos app new my-tool --language rust
  wazeos app new data-processor --language go --json`,
	Args: cobra.ExactArgs(1),
	Run:  runAppNew,
}

var appBuildCmd = &cobra.Command{
	Use:   "build <name>",
	Short: "Build an app",
	Long:  `Compile an app to WASM.`,
	Args:  cobra.ExactArgs(1),
	Run:   runAppBuild,
}

var appTestCmd = &cobra.Command{
	Use:   "test <name>",
	Short: "Test an app",
	Long:  `Run app unit tests.`,
	Args:  cobra.ExactArgs(1),
	Run:   runAppTest,
}

var appInstallCmd = &cobra.Command{
	Use:   "install <path-or-name>",
	Short: "Install an app",
	Long:  `Register an app with the MCP server.`,
	Args:  cobra.ExactArgs(1),
	Run:   runAppInstall,
}

var appListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed apps",
	Long:  `Show all registered apps.`,
	Run:   runAppList,
}

var appUninstallCmd = &cobra.Command{
	Use:   "uninstall <name>",
	Short: "Uninstall an app",
	Long:  `Remove an app from the system.`,
	Args:  cobra.ExactArgs(1),
	Run:   runAppUninstall,
}

var appPackageCmd = &cobra.Command{
	Use:   "package <name>",
	Short: "Package an app for distribution",
	Long: `Create a .wazpkg file containing the app and its manifest.

The package can be shared with others and installed using:
  wazeos app install path/to/package.wazpkg

Examples:
  wazeos app package my-tool
  wazeos app package test-tool --output ./dist/`,
	Args: cobra.ExactArgs(1),
	Run:  runAppPackage,
}

var (
	appLanguage   string
	appTemplate   string
	appAuthor     string
	packageOutput string
)

func init() {
	// app new flags
	appNewCmd.Flags().StringVar(&appLanguage, "language", "rust", "Language (rust|go|python)")
	appNewCmd.Flags().StringVar(&appTemplate, "template", "mcp-tool", "Template (mcp-tool|cli|http-handler)")
	appNewCmd.Flags().StringVar(&appAuthor, "author", "", "Author/organization name (e.g., 'acme' for acme/my-app)")

	// app package flags
	appPackageCmd.Flags().StringVarP(&packageOutput, "output", "o", ".", "Output directory for package file")

	// Add subcommands
	appCmd.AddCommand(appNewCmd)
	appCmd.AddCommand(appBuildCmd)
	appCmd.AddCommand(appTestCmd)
	appCmd.AddCommand(appInstallCmd)
	appCmd.AddCommand(appListCmd)
	appCmd.AddCommand(appUninstallCmd)
	appCmd.AddCommand(appPackageCmd)
}

func runAppNew(cmd *cobra.Command, args []string) {
	nameArg := args[0]

	// Parse author and name from argument or flags
	var author, name string
	if strings.Contains(nameArg, "/") {
		// Format: author/name
		parts := strings.SplitN(nameArg, "/", 2)
		author = parts[0]
		name = parts[1]
	} else {
		// Just name, use --author flag or default
		name = nameArg
		author = appAuthor
		if author == "" {
			author = "default"
		}
	}

	// Validate author and name
	if author == "" || name == "" {
		outputError("app new", "INVALID_NAME", "invalid app name format",
			"Use 'author/name' or provide --author flag")
	}

	appPath := filepath.Join("apps", author, name)

	if _, err := os.Stat(appPath); err == nil {
		outputError("app new", "APP_EXISTS", "app already exists: "+appPath,
			"Choose a different name or delete the existing app")
	}

	if err := os.MkdirAll(appPath, 0755); err != nil {
		outputError("app new", "CREATE_FAILED", err.Error(), "")
	}

	var filesCreated []string
	switch appLanguage {
	case "rust":
		filesCreated = generateRustApp(appPath, author, name)
	case "go", "python":
		outputError("app new", "NOT_IMPLEMENTED", appLanguage+" apps not yet supported", "Use --language rust")
	default:
		outputError("app new", "INVALID_LANGUAGE", "invalid language: "+appLanguage, "Valid languages: rust, go, python")
	}

	fullName := author + "/" + name
	if jsonOutput {
		outputSuccess("app new", map[string]interface{}{
			"author":        author,
			"name":          name,
			"full_name":     fullName,
			"path":          appPath,
			"language":      appLanguage,
			"files_created": filesCreated,
		})
	} else {
		logSuccess("✓", fmt.Sprintf("Created app scaffold at %s/", appPath))
		logNextSteps(
			fmt.Sprintf("cd %s", appPath),
			"Edit src/lib.rs to implement your tool logic",
			fmt.Sprintf("wazeos app build %s", fullName),
			"wazeos app install "+fullName,
		)
	}
}

func generateRustApp(appPath, author, name string) []string {
	files := []string{}

	// Create src directory
	srcPath := filepath.Join(appPath, "src")
	os.MkdirAll(srcPath, 0755)

	// Generate wazeos.toml manifest
	manifestFile := filepath.Join(appPath, "wazeos.toml")
	manifest := &pkg.Manifest{
		Package: pkg.PackageInfo{
			Name:        name,
			Version:     "0.1.0",
			Description: fmt.Sprintf("A WazeOS MCP tool: %s", name),
			Authors:     []string{},
		},
		Tool: &pkg.ToolInfo{
			Name:        name,
			Description: fmt.Sprintf("MCP tool: %s", name),
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"input": map[string]interface{}{
						"type":        "string",
						"description": "Input text",
					},
				},
				"required": []string{"input"},
			},
		},
		Permissions: pkg.PermissionsInfo{
			File: []string{"read"},
		},
	}
	if err := pkg.SaveManifest(manifestFile, manifest); err != nil {
		outputError("app new", "MANIFEST_FAILED", err.Error(), "")
	}
	files = append(files, manifestFile)

	// Generate Cargo.toml
	cargoFile := filepath.Join(appPath, "Cargo.toml")
	cargoContent := fmt.Sprintf(`[package]
name = "%s"
version = "0.1.0"
edition = "2021"

[dependencies]
wazeos-app = { path = "../../sdk/rust/app" }
serde = { version = "1.0", features = ["derive"] }
serde_json = "1.0"

[lib]
crate-type = ["cdylib"]

[profile.release]
opt-level = "z"
lto = true
strip = true
`, name)
	os.WriteFile(cargoFile, []byte(cargoContent), 0644)
	files = append(files, cargoFile)

	// Generate lib.rs with proper MCP tool implementation
	mainFile := filepath.Join(srcPath, "lib.rs")
	mainContent := fmt.Sprintf(`//! %s - WazeOS MCP Tool
//!
//! This tool is built with the WazeOS App SDK and exposes an MCP tool interface.
//!
//! ## Quick Start
//!
//! Build:    cargo build --target wasm32-wasip1 --release
//!           (or: wazeos app build %s/%s)
//!
//! Test:     cargo test
//!           (or: wazeos app test %s/%s)
//!
//! Dev/Debug: wazeos dev run -v --app target/wasm32-wasip1/release/%s.wasm \
//!                               --driver path/to/driver.so \
//!                               --interactive
//!            (test in isolated environment without installing)
//!
//! Install:  wazeos app install target/wasm32-wasip1/release/%s.wasm
//!           (or: wazeos app install %s/%s)
//!
//! Package:  wazeos app package %s/%s
//!           (creates %s-0.1.0.wazpkg for distribution)
//!
//! ## Development & Testing
//!
//! Use 'wazeos dev run' to test your app in an isolated environment:
//!   - Doesn't affect installed packages
//!   - Load specific drivers and apps
//!   - Verbose logging with -v flag
//!   - Interactive REPL mode for testing
//!
//! ## Usage from Claude Desktop
//!
//! Once installed, this tool will be available as an MCP tool in Claude Desktop.
//! Claude can invoke it automatically when needed, or you can reference it directly.
//!
//! ## I/O Access
//!
//! Access I/O drivers through the AppContext:
//!   - Files: ctx.call("file:///path/to/file.txt", ...)
//!   - HTTP:  ctx.call("https://api.example.com/data", ...)
//!   - Shell: ctx.call("shell://exec", headers, ...)
//!
//! Always return proper errors: Err("Descriptive error message".to_string())

use serde_json::{json, Value};
use wazeos_app::{AppContext, AppResult, register_tool};

/// Main tool entry point
///
/// This function is called when the MCP tool is invoked from Claude.
///
/// # Arguments
/// * ctx - Application context with access to I/O drivers
/// * args - JSON arguments passed from the MCP client
///
/// # Returns
/// JSON response that will be sent back to the MCP client
#[no_mangle]
pub extern "C" fn tool_main(ctx: &AppContext, args: Value) -> AppResult {
    // Extract input from args
    let input = args["input"].as_str().unwrap_or("world");

    // TODO: Implement your tool logic here
    //
    // Example: Use drivers to read files, make HTTP requests, etc.
    // let file_content = ctx.read_file("path/to/file.txt")?;
    // let http_response = ctx.http_get("https://api.example.com/data")?;

    // Return result
    Ok(json!({
        "message": format!("Hello, {}!", input),
        "tool": "%s",
        "status": "success"
    }))
}

// Register this function as an MCP tool
// This macro generates the required exports for WazeOS to call your tool
register_tool!(tool_main);
`, name, author, name, author, name, name, name, author, name, author, name, name)
	os.WriteFile(mainFile, []byte(mainContent), 0644)
	files = append(files, mainFile)

	// Note: README generation removed - all documentation is in source code comments

	return files
}

func runAppBuild(cmd *cobra.Command, args []string) {
	name := args[0]
	appPath := filepath.Join("apps", name)

	if _, err := os.Stat(appPath); os.IsNotExist(err) {
		outputError("app build", "APP_NOT_FOUND", "app not found: "+appPath, "")
	}

	logInfo("Building Rust app: %s", name)

	// Build WASM
	buildCmd := fmt.Sprintf("cd %s && cargo build --target wasm32-wasip1 --release", appPath)
	if err := executeCommand(buildCmd); err != nil {
		outputError("app build", "BUILD_FAILED", err.Error(), "")
	}

	wasmPath := filepath.Join(appPath, "target/wasm32-wasip1/release", name+".wasm")

	if jsonOutput {
		outputSuccess("app build", map[string]interface{}{
			"wasm": wasmPath,
		})
	} else {
		logSuccess("✓", fmt.Sprintf("Build complete: %s", wasmPath))
	}
}

func runAppTest(cmd *cobra.Command, args []string) {
	name := args[0]
	appPath := filepath.Join("apps", name)

	logInfo("Running tests for %s app...", name)

	testCmd := fmt.Sprintf("cd %s && cargo test", appPath)
	if err := executeCommand(testCmd); err != nil {
		outputError("app test", "TESTS_FAILED", err.Error(), "")
	}

	if jsonOutput {
		outputSuccess("app test", map[string]interface{}{
			"tests_passed": "all",
		})
	} else {
		logSuccess("✓", "All tests passed")
	}
}

func runAppInstall(cmd *cobra.Command, args []string) {
	pathOrName := args[0]

	// Check if this is a .wazpkg file
	if strings.HasSuffix(pathOrName, ".wazpkg") {
		installFromPackage(pathOrName)
		return
	}

	// Determine if this is a local path or a package name
	var appPath string
	if _, err := os.Stat(pathOrName); err == nil {
		// Local path exists
		appPath = pathOrName
	} else {
		// Try apps/ directory
		appPath = filepath.Join("apps", pathOrName)
		if _, err := os.Stat(appPath); os.IsNotExist(err) {
			outputError("app install", "APP_NOT_FOUND",
				fmt.Sprintf("app not found: %s", pathOrName),
				"Build it first with: wazeos app build "+pathOrName)
		}
	}

	// Load manifest
	manifestPath := filepath.Join(appPath, "wazeos.toml")
	manifest, err := pkg.LoadManifest(manifestPath)
	if err != nil {
		outputError("app install", "INVALID_MANIFEST", err.Error(),
			"Check wazeos.toml for errors")
	}

	if !manifest.IsApp() {
		outputError("app install", "NOT_AN_APP",
			"this package is a driver, not an app",
			"Use: wazeos driver install "+pathOrName)
	}

	// Check if WASM binary exists
	// Note: Cargo converts hyphens to underscores in filenames
	cargoName := strings.ReplaceAll(manifest.Package.Name, "-", "_")
	wasmPath := filepath.Join(appPath, "target/wasm32-wasip1/release",
		fmt.Sprintf("%s.wasm", cargoName))
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		outputError("app install", "WASM_NOT_FOUND",
			"WASM binary not found: "+wasmPath,
			"Build it first with: wazeos app build "+manifest.Package.Name)
	}

	// Create installation directory
	installDir := filepath.Join(os.Getenv("HOME"), ".wazeos", "apps", manifest.Package.Name)
	if err := os.MkdirAll(installDir, 0755); err != nil {
		outputError("app install", "INSTALL_FAILED", err.Error(), "")
	}

	// Copy WASM binary to installation directory
	destWasm := filepath.Join(installDir, manifest.Package.Name+".wasm")
	wasmData, err := os.ReadFile(wasmPath)
	if err != nil {
		outputError("app install", "READ_FAILED", err.Error(), "")
	}
	if err := os.WriteFile(destWasm, wasmData, 0755); err != nil {
		outputError("app install", "WRITE_FAILED", err.Error(), "")
	}

	// Copy manifest
	destManifest := filepath.Join(installDir, "wazeos.toml")
	if err := pkg.SaveManifest(destManifest, manifest); err != nil {
		outputError("app install", "MANIFEST_SAVE_FAILED", err.Error(), "")
	}

	toolName := manifest.Tool.Name

	if jsonOutput {
		outputSuccess("app install", map[string]interface{}{
			"app":         manifest.Package.Name,
			"version":     manifest.Package.Version,
			"tool_name":   toolName,
			"install_dir": installDir,
			"wasm_path":   destWasm,
		})
	} else {
		logSuccess("✓", fmt.Sprintf("Installed %s v%s",
			manifest.Package.Name, manifest.Package.Version))
		logInfo("Tool name: %s", toolName)
		logInfo("Location: %s", installDir)
		logInfo("")
		logInfo("The tool is now available for use.")
		logInfo("Restart your MCP server to pick up the new tool.")
	}
}

func runAppList(cmd *cobra.Command, args []string) {
	installDir := filepath.Join(os.Getenv("HOME"), ".wazeos", "apps")

	// Check if directory exists
	if _, err := os.Stat(installDir); os.IsNotExist(err) {
		if jsonOutput {
			outputSuccess("app list", map[string]interface{}{
				"apps": []interface{}{},
			})
		} else {
			logInfo("No apps installed")
		}
		return
	}

	type appInfo struct {
		Name        string `json:"name"`
		Version     string `json:"version"`
		Description string `json:"description"`
		ToolName    string `json:"tool_name"`
	}

	apps := []appInfo{}

	// Walk through installed apps
	filepath.Walk(installDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return nil
		}
		if path == installDir {
			return nil
		}

		// Try to load manifest
		manifestPath := filepath.Join(path, "wazeos.toml")
		if manifest, err := pkg.LoadManifest(manifestPath); err == nil {
			apps = append(apps, appInfo{
				Name:        manifest.Package.Name,
				Version:     manifest.Package.Version,
				Description: manifest.Package.Description,
				ToolName:    manifest.Tool.Name,
			})
		}

		return filepath.SkipDir // Don't recurse into subdirectories
	})

	if jsonOutput {
		outputSuccess("app list", map[string]interface{}{
			"apps": apps,
		})
	} else {
		if len(apps) == 0 {
			logInfo("No apps installed")
		} else {
			logInfo("Installed apps (%d):", len(apps))
			logInfo("")
			for _, a := range apps {
				fmt.Printf("  %s (%s)\n", a.Name, a.Version)
				if a.Description != "" {
					fmt.Printf("    %s\n", a.Description)
				}
				fmt.Printf("    Tool: %s\n", a.ToolName)
				fmt.Println()
			}
		}
	}
}

func runAppUninstall(cmd *cobra.Command, args []string) {
	name := args[0]

	if jsonOutput {
		outputSuccess("app uninstall", map[string]interface{}{
			"app": name,
		})
	} else {
		logSuccess("✓", fmt.Sprintf("App uninstalled: %s", name))
	}
}

func runAppPackage(cmd *cobra.Command, args []string) {
	name := args[0]
	appPath := filepath.Join("apps", name)

	// Verify app exists
	if _, err := os.Stat(appPath); os.IsNotExist(err) {
		outputError("app package", "APP_NOT_FOUND", "app not found: "+appPath, "")
	}

	// Load manifest
	manifestPath := filepath.Join(appPath, "wazeos.toml")
	manifest, err := pkg.LoadManifest(manifestPath)
	if err != nil {
		outputError("app package", "INVALID_MANIFEST", err.Error(),
			"Check wazeos.toml for errors")
	}

	if !manifest.IsApp() {
		outputError("app package", "NOT_AN_APP",
			"this package is a driver, not an app",
			"Use: wazeos driver package "+name)
	}

	// Check if WASM binary exists
	cargoName := strings.ReplaceAll(manifest.Package.Name, "-", "_")
	wasmPath := filepath.Join(appPath, "target/wasm32-wasip1/release",
		fmt.Sprintf("%s.wasm", cargoName))
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		outputError("app package", "WASM_NOT_FOUND",
			"WASM binary not found: "+wasmPath,
			"Build it first with: wazeos app build "+name)
	}

	// Create output directory if needed
	if err := os.MkdirAll(packageOutput, 0755); err != nil {
		outputError("app package", "OUTPUT_DIR_FAILED", err.Error(), "")
	}

	// Create package filename
	packageName := fmt.Sprintf("%s-%s.wazpkg", manifest.Package.Name, manifest.Package.Version)
	packagePath := filepath.Join(packageOutput, packageName)

	// Create package metadata
	packageMeta := map[string]interface{}{
		"name":                   manifest.Package.Name,
		"version":                manifest.Package.Version,
		"package_format_version": "1.0",
		"created_at":             time.Now().UTC().Format(time.RFC3339),
		"sdk_version":            "0.1.0",
	}

	// Create tar.gz archive
	if err := createPackage(packagePath, manifest, wasmPath, packageMeta); err != nil {
		outputError("app package", "PACKAGE_FAILED", err.Error(), "")
	}

	// Get package file size
	stat, _ := os.Stat(packagePath)
	sizeKB := stat.Size() / 1024

	if jsonOutput {
		outputSuccess("app package", map[string]interface{}{
			"name":    manifest.Package.Name,
			"version": manifest.Package.Version,
			"package": packagePath,
			"size_kb": sizeKB,
		})
	} else {
		logSuccess("✓", fmt.Sprintf("Created package: %s (%d KB)", packagePath, sizeKB))
		logInfo("")
		logInfo("Share this package with others:")
		logInfo("  wazeos app install %s", packagePath)
	}
}

// createPackage creates a .wazpkg tar.gz archive
func createPackage(packagePath string, manifest *pkg.Manifest, wasmPath string, metadata map[string]interface{}) error {
	// Create the output file
	file, err := os.Create(packagePath)
	if err != nil {
		return fmt.Errorf("failed to create package file: %w", err)
	}
	defer file.Close()

	// Create gzip writer
	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Add wazeos.toml
	manifestData, err := os.ReadFile(filepath.Join(filepath.Dir(wasmPath), "../../..", "wazeos.toml"))
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	if err := addFileToTar(tarWriter, "wazeos.toml", manifestData); err != nil {
		return fmt.Errorf("failed to add manifest: %w", err)
	}

	// Add WASM binary
	wasmData, err := os.ReadFile(wasmPath)
	if err != nil {
		return fmt.Errorf("failed to read WASM binary: %w", err)
	}

	wasmFilename := manifest.Package.Name + ".wasm"
	if err := addFileToTar(tarWriter, wasmFilename, wasmData); err != nil {
		return fmt.Errorf("failed to add WASM binary: %w", err)
	}

	// Add package.json metadata
	metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := addFileToTar(tarWriter, "package.json", metadataJSON); err != nil {
		return fmt.Errorf("failed to add metadata: %w", err)
	}

	return nil
}

// addFileToTar adds a file to a tar archive
func addFileToTar(tw *tar.Writer, filename string, data []byte) error {
	header := &tar.Header{
		Name:    filename,
		Size:    int64(len(data)),
		Mode:    0644,
		ModTime: time.Now(),
	}

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	if _, err := io.Copy(tw, strings.NewReader(string(data))); err != nil {
		return err
	}

	return nil
}

// installFromPackage installs an app from a .wazpkg file
func installFromPackage(packagePath string) {
	// Verify package exists
	if _, err := os.Stat(packagePath); os.IsNotExist(err) {
		outputError("app install", "PACKAGE_NOT_FOUND",
			"package not found: "+packagePath, "")
	}

	// Create temporary directory for extraction
	tempDir, err := os.MkdirTemp("", "wazeos-pkg-*")
	if err != nil {
		outputError("app install", "TEMP_DIR_FAILED", err.Error(), "")
	}
	defer os.RemoveAll(tempDir)

	// Extract package
	if err := extractPackage(packagePath, tempDir); err != nil {
		outputError("app install", "EXTRACT_FAILED", err.Error(),
			"Package may be corrupted")
	}

	// Load manifest from extracted files
	manifestPath := filepath.Join(tempDir, "wazeos.toml")
	manifest, err := pkg.LoadManifest(manifestPath)
	if err != nil {
		outputError("app install", "INVALID_MANIFEST", err.Error(),
			"Package contains invalid manifest")
	}

	if !manifest.IsApp() {
		outputError("app install", "NOT_AN_APP",
			"this package is a driver, not an app",
			"Use: wazeos driver install "+packagePath)
	}

	// Verify WASM binary exists
	wasmFilename := manifest.Package.Name + ".wasm"
	wasmPath := filepath.Join(tempDir, wasmFilename)
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		outputError("app install", "INVALID_PACKAGE",
			"package missing WASM binary: "+wasmFilename,
			"Package may be corrupted")
	}

	// Create installation directory
	installDir := filepath.Join(os.Getenv("HOME"), ".wazeos", "apps", manifest.Package.Name)
	if err := os.MkdirAll(installDir, 0755); err != nil {
		outputError("app install", "INSTALL_FAILED", err.Error(), "")
	}

	// Copy WASM binary
	destWasm := filepath.Join(installDir, wasmFilename)
	wasmData, err := os.ReadFile(wasmPath)
	if err != nil {
		outputError("app install", "READ_FAILED", err.Error(), "")
	}
	if err := os.WriteFile(destWasm, wasmData, 0755); err != nil {
		outputError("app install", "WRITE_FAILED", err.Error(), "")
	}

	// Copy manifest
	destManifest := filepath.Join(installDir, "wazeos.toml")
	if err := pkg.SaveManifest(destManifest, manifest); err != nil {
		outputError("app install", "MANIFEST_SAVE_FAILED", err.Error(), "")
	}

	toolName := manifest.Tool.Name

	if jsonOutput {
		outputSuccess("app install", map[string]interface{}{
			"app":         manifest.Package.Name,
			"version":     manifest.Package.Version,
			"tool_name":   toolName,
			"install_dir": installDir,
			"source":      "package",
		})
	} else {
		logSuccess("✓", fmt.Sprintf("Installed %s v%s from package",
			manifest.Package.Name, manifest.Package.Version))
		logInfo("Tool name: %s", toolName)
		logInfo("Location: %s", installDir)
		logInfo("")
		logInfo("The tool is now available for use.")
		logInfo("Restart your MCP server to pick up the new tool.")
	}
}

// extractPackage extracts a .wazpkg tar.gz archive
func extractPackage(packagePath, destDir string) error {
	// Open package file
	file, err := os.Open(packagePath)
	if err != nil {
		return fmt.Errorf("failed to open package: %w", err)
	}
	defer file.Close()

	// Create gzip reader
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to decompress package: %w", err)
	}
	defer gzipReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzipReader)

	// Extract files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Construct destination path
		destPath := filepath.Join(destDir, header.Name)

		// Security: prevent path traversal
		if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path in package: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeReg:
			// Create file
			outFile, err := os.Create(destPath)
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", header.Name, err)
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to extract file %s: %w", header.Name, err)
			}
			outFile.Close()

		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", header.Name, err)
			}
		}
	}

	return nil
}
