package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wazeos/wazeos/core/internal/pkg"
	"github.com/wazeos/wazeos/core/kernel/iobus"
)

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Development utilities",
	Long:  `Tools for developing drivers and apps.`,
}

var devServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start development server",
	Long:  `Start local MCP server with hot-reload.`,
	Run:   runDevServe,
}

var devInspectCmd = &cobra.Command{
	Use:   "inspect <type> <name>",
	Short: "Inspect driver or app",
	Long:  `Show metadata and capabilities.`,
	Args:  cobra.ExactArgs(2),
	Run:   runDevInspect,
}

var devDebugCmd = &cobra.Command{
	Use:   "debug <app>/<tool> [args...]",
	Short: "Debug tool invocation",
	Long:  `Run tool with verbose logging.`,
	Args:  cobra.MinimumNArgs(1),
	Run:   runDevDebug,
}

var devRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run isolated test environment",
	Long: `Create an isolated IOBus with specific drivers and apps for testing.

This command loads drivers and apps from packaged .wazpkg files without
affecting installed packages. Perfect for development and testing interactions
between multiple components.

✨ Package-based architecture with isolated runtimes!
   Load as many packages as you need - true parallelism, perfect isolation.

Examples:
  # ✅ Load driver + app packages (recommended!)
  wazeos dev run --driver drivers/shell/build/shell.wazpkg \
                 --app apps/tool/build/tool.wazpkg \
                 --interactive

  # ✅ Multiple driver packages + app
  wazeos dev run --driver drivers/api/build/api.wazpkg \
                 --driver drivers/db/build/db.wazpkg \
                 --app apps/admin/build/admin.wazpkg \
                 --interactive

  # ✅ Test driver package standalone
  wazeos dev run --driver drivers/api/build/api.wazpkg --interactive

Package format:
  - .wazpkg files contain the WASM binary + metadata.json
  - Created with: wazeos app build or wazeos driver build
  - Ensures metadata and binary stay synchronized`,
	Run: runDevRun,
}

var (
	devPort       int
	devDrivers    []string
	devApps       []string
	devInvoke     string
	devInvokeArgs string
	devInteractive bool
)

// loadedApp stores metadata about loaded apps for invocation
type loadedApp struct {
	name   string
	author string
	uri    string // Full URI: app://127.0.0.1/{author}/{name}
}

var loadedAppRegistry []loadedApp

func init() {
	// dev run flags
	devRunCmd.Flags().StringSliceVar(&devDrivers, "driver", []string{}, "Load driver from path (repeatable)")
	devRunCmd.Flags().StringSliceVar(&devApps, "app", []string{}, "Load app from path (repeatable)")
	devRunCmd.Flags().StringVar(&devInvoke, "invoke", "", "Tool to invoke (format: app-name/tool-name)")
	devRunCmd.Flags().StringVar(&devInvokeArgs, "args", "{}", "JSON arguments for tool invocation")
	devRunCmd.Flags().BoolVarP(&devInteractive, "interactive", "i", false, "Start interactive REPL")

	// Only add fully implemented commands
	devCmd.AddCommand(devRunCmd)

	// Note: serve, inspect, and debug commands removed until implemented
	// TODO: Implement dev serve, inspect, and debug commands
}

func runDevServe(cmd *cobra.Command, args []string) {
	logInfo("Starting development server on port %d...", devPort)
	logInfo("Press Ctrl+C to stop")

	// TODO: Implement dev server
	fmt.Println("Development server not yet implemented")
}

func runDevInspect(cmd *cobra.Command, args []string) {
	typ := args[0]  // "driver" or "app"
	name := args[1]

	// TODO: Implement inspection
	result := map[string]interface{}{
		"type": typ,
		"name": name,
	}

	if jsonOutput {
		outputSuccess("dev inspect", result)
	} else {
		fmt.Printf("Inspecting %s: %s\n", typ, name)
		fmt.Println("(Inspection not yet implemented)")
	}
}

func runDevDebug(cmd *cobra.Command, args []string) {
	toolPath := args[0]

	logInfo("Debugging tool: %s", toolPath)
	// TODO: Implement debug mode
	fmt.Println("Debug mode not yet implemented")
}

func runDevRun(cmd *cobra.Command, args []string) {
	logInfo("Starting dev environment...")

	// Clear loaded app registry for this run
	loadedAppRegistry = []loadedApp{}

	// Use the default IOBus (same as normal operation)
	// All runtimes are already registered from package init()
	bus := iobus.GetDefaultBus()

	logInfo("✓ Using default IOBus (runtimes already registered)")

	// Load drivers
	if len(devDrivers) > 0 {
		logInfo("Loading %d driver(s)...", len(devDrivers))
		for i, driverPath := range devDrivers {
			if _, err := os.Stat(driverPath); os.IsNotExist(err) {
				outputError("dev run", "DRIVER_NOT_FOUND",
					fmt.Sprintf("driver not found: %s", driverPath), "")
			}
			logInfo("  [%d/%d] Loading: %s", i+1, len(devDrivers), driverPath)

			// Load WASM driver
			if err := loadWASMDriver(bus, driverPath, i); err != nil {
				outputError("dev run", "DRIVER_LOAD_FAILED",
					fmt.Sprintf("failed to load driver: %v", err),
					"Check that the driver is a valid WASM module")
			}
			logSuccess("    ✓", "Driver loaded")
		}
	} else {
		fmt.Println("⚠ No drivers specified")
	}

	// Load apps
	if len(devApps) > 0 {
		logInfo("Loading %d app(s)...", len(devApps))
		for i, appPath := range devApps {
			if _, err := os.Stat(appPath); os.IsNotExist(err) {
				outputError("dev run", "APP_NOT_FOUND",
					fmt.Sprintf("app not found: %s", appPath), "")
			}
			logInfo("  [%d/%d] Loading: %s", i+1, len(devApps), appPath)

			// Load WASM app
			if err := loadWASMApp(bus, appPath, i); err != nil {
				outputError("dev run", "APP_LOAD_FAILED",
					fmt.Sprintf("failed to load app: %v", err),
					"Check that the app is a valid WASM module")
			}
			logSuccess("    ✓", "App loaded")
		}
	} else {
		fmt.Println("⚠ No apps specified")
	}

	logInfo("")
	logSuccess("✓", "Environment ready")
	logInfo("")

	// Handle invocation or interactive mode
	if devInvoke != "" {
		// Single invocation mode
		runDevInvocation(bus)
	} else if devInteractive {
		// Interactive REPL mode
		runDevREPL(bus)
	} else {
		outputError("dev run", "NO_ACTION",
			"specify --invoke or --interactive",
			"Use --invoke to run a tool or --interactive for REPL mode")
	}
}

func runDevInvocation(bus *iobus.IOBus) {
	parts := strings.SplitN(devInvoke, "/", 2)
	if len(parts) != 2 {
		outputError("dev run", "INVALID_TOOL",
			"tool must be in format: app-name/tool-name",
			"Example: --invoke my-app/my-tool")
	}

	appName := parts[0]
	toolName := parts[1]

	logInfo("Invoking: %s/%s", appName, toolName)
	logInfo("Arguments: %s", devInvokeArgs)

	// Parse arguments
	var argsJSON map[string]interface{}
	if err := json.Unmarshal([]byte(devInvokeArgs), &argsJSON); err != nil {
		outputError("dev run", "INVALID_ARGS",
			fmt.Sprintf("failed to parse arguments: %v", err),
			"Arguments must be valid JSON")
	}

	// Create context with full permissions for dev mode
	ctx := iobus.NewContext(
		context.Background(),
		"dev-user",
		"dev-req-001",
		"dev-trace-001",
		[]iobus.PermissionEntry{
			{
				URIPattern:  "**",
				Permissions: []string{"call", "read", "write"},
			},
		},
		bus,
	)

	// Call the app directly at its app:// URI
	// Look up the app from our loaded registry
	var appURI string
	for _, app := range loadedAppRegistry {
		if app.name == appName {
			appURI = app.uri
			break
		}
	}

	if appURI == "" {
		outputError("dev run", "APP_NOT_FOUND",
			fmt.Sprintf("app '%s' not found in loaded apps", appName),
			"Ensure the app was loaded successfully with --app flag")
	}

	// Prepare app call request
	req := iobus.Request{
		URI:       appURI,
		Operation: iobus.OpCall,
		Args:      argsJSON,
	}

	// Call the tool
	resp, err := bus.Call(ctx, req)
	if err != nil {
		outputError("dev run", "INVOKE_FAILED",
			fmt.Sprintf("tool invocation failed: %v", err),
			"Check that the app is properly loaded")
	}

	// Display results
	logInfo("")
	if resp.StatusCode == 200 {
		logSuccess("✓", "Tool invocation completed")
		logInfo("")
		logInfo("Result:")

		// Pretty print the response body as JSON
		var result interface{}
		if err := json.Unmarshal(resp.Body, &result); err == nil {
			prettyJSON, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(prettyJSON))
		} else {
			// If not JSON, print as string
			fmt.Println(string(resp.Body))
		}
	} else {
		logInfo("✗ Tool invocation failed: %s", resp.Error)
		if verbose {
			logInfo("Status: %d", resp.StatusCode)
			logInfo("Response: %s", string(resp.Body))
		}
	}

	if verbose {
		logInfo("")
		logInfo("Context: %+v", ctx)
		logInfo("Arguments: %+v", argsJSON)
	}
}

func runDevREPL(bus *iobus.IOBus) {
	logInfo("Starting interactive REPL...")
	logInfo("Type 'help' for commands, 'exit' to quit")
	logInfo("")

	// Create context with full permissions for dev mode
	ctx := iobus.NewContext(
		context.Background(),
		"dev-user",
		"dev-req-repl",
		"dev-trace-repl",
		[]iobus.PermissionEntry{
			{
				URIPattern:  "**",
				Permissions: []string{"call", "read", "write"},
			},
		},
		bus,
	)

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("wazeos> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			logInfo("Exiting...")
			break
		}

		if input == "help" {
			fmt.Println("Available commands:")
			fmt.Println("  invoke <app>/<tool> <json-args>  - Invoke a tool")
			fmt.Println("  call <uri>                       - Call IOBus URI directly")
			fmt.Println("  drivers                          - List loaded drivers")
			fmt.Println("  apps                             - List loaded apps")
			fmt.Println("  help                             - Show this help")
			fmt.Println("  exit                             - Exit REPL")
			continue
		}

		// Parse command
		parts := strings.Fields(input)
		if len(parts) == 0 {
			continue
		}

		command := parts[0]
		switch command {
		case "invoke":
			if len(parts) < 2 {
				fmt.Println("Error: Usage: invoke <app>/<tool> [json-args]")
				continue
			}
			toolPath := parts[1]
			argsJSON := "{}"
			if len(parts) > 2 {
				argsJSON = strings.Join(parts[2:], " ")
			}

			// Parse tool path (e.g., "date-test/tool_main")
			toolParts := strings.SplitN(toolPath, "/", 2)
			if len(toolParts) != 2 {
				fmt.Println("Error: Tool path must be in format <app>/<tool>")
				continue
			}
			appName := toolParts[0]
			toolName := toolParts[1]

			// Construct app URI (apps are registered as app://127.0.0.1/local/<appname>)
			// Use the exact app name (e.g., date_test not date-test)
			appURI := fmt.Sprintf("app://127.0.0.1/local/%s", appName)

			logInfo("Invoking: %s on %s with args: %s", toolName, appURI, argsJSON)

			// Parse args JSON
			var args map[string]any
			if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
				fmt.Printf("Error: Invalid JSON args: %v\n", err)
				continue
			}

			// Create request
			req := iobus.Request{
				URI:       appURI,
				Operation: iobus.OpCall,
				Args:      args,
			}

			// Call the app
			resp, err := bus.Call(ctx, req)
			if err != nil {
				fmt.Printf("Error: Invoke failed: %v\n", err)
				continue
			}

			// Display response
			fmt.Printf("Status: %d\n", resp.StatusCode)
			if resp.Error != "" {
				fmt.Printf("Error: %s\n", resp.Error)
			}
			if len(resp.Body) > 0 {
				// Try to pretty-print JSON
				var prettyJSON map[string]any
				if json.Unmarshal(resp.Body, &prettyJSON) == nil {
					prettyBytes, _ := json.MarshalIndent(prettyJSON, "", "  ")
					fmt.Printf("Response:\n%s\n", string(prettyBytes))
				} else {
					fmt.Printf("Response: %s\n", string(resp.Body))
				}
			}

		case "call":
			if len(parts) < 2 {
				fmt.Println("Error: Usage: call <uri>")
				continue
			}
			uri := parts[1]
			logInfo("Calling: %s", uri)

			req := iobus.Request{
				URI:       uri,
				Operation: iobus.OpCall,
			}

			resp, err := bus.Call(ctx, req)
			if err != nil {
				fmt.Printf("Error: Call failed: %v\n", err)
				continue
			}

			fmt.Printf("Status: %d\n", resp.StatusCode)
			if len(resp.Body) > 0 {
				fmt.Printf("Body: %s\n", string(resp.Body))
			}

		case "drivers":
			logInfo("Loaded drivers: %d", len(devDrivers))
			for i, path := range devDrivers {
				fmt.Printf("  [%d] %s\n", i+1, path)
			}

		case "apps":
			logInfo("Loaded apps: %d", len(devApps))
			for i, path := range devApps {
				fmt.Printf("  [%d] %s\n", i+1, path)
			}

		default:
			fmt.Printf("Error: Unknown command: %s (type 'help' for commands)\n", command)
		}
	}
}

// extractPackageToTemp extracts a .wazpkg tar.gz archive to a temporary directory
func extractPackageToTemp(packagePath string) (destDir string, cleanup func(), err error) {
	// Create temp directory
	destDir, err = os.MkdirTemp("", "wazeos-dev-*")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	cleanup = func() {
		os.RemoveAll(destDir)
	}

	// Open package file
	file, err := os.Open(packagePath)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to open package: %w", err)
	}
	defer file.Close()

	// Create gzip reader
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to decompress package: %w", err)
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
			cleanup()
			return "", nil, fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Construct destination path
		destPath := filepath.Join(destDir, header.Name)

		// Security: prevent path traversal
		if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			cleanup()
			return "", nil, fmt.Errorf("invalid file path in package: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeReg:
			// Create file
			outFile, err := os.Create(destPath)
			if err != nil {
				cleanup()
				return "", nil, fmt.Errorf("failed to create file %s: %w", header.Name, err)
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				cleanup()
				return "", nil, fmt.Errorf("failed to extract file %s: %w", header.Name, err)
			}
			outFile.Close()

		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(destPath, 0755); err != nil {
				cleanup()
				return "", nil, fmt.Errorf("failed to create directory %s: %w", header.Name, err)
			}
		}
	}

	return destDir, cleanup, nil
}

// loadWASMDriver loads a driver package (.wazpkg) and registers it with the IOBus
func loadWASMDriver(bus *iobus.IOBus, packagePath string, index int) error {
	// Ensure it's a .wazpkg file
	if !strings.HasSuffix(packagePath, ".wazpkg") {
		return fmt.Errorf("driver must be a .wazpkg file, got: %s", packagePath)
	}

	// Extract package to temp directory
	extractDir, cleanup, err := extractPackageToTemp(packagePath)
	if err != nil {
		return fmt.Errorf("failed to extract package: %w", err)
	}
	// Note: We don't cleanup immediately - the binary needs to stay available
	// In production, this would be managed by the package manager
	// In dev mode, temp files are cleaned up on process exit
	_ = cleanup // Keep reference to avoid unused warning

	// Load metadata.json
	metadataPath := filepath.Join(extractDir, "metadata.json")
	manifest, err := pkg.LoadManifest(metadataPath)
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	// Verify it's a driver
	if !manifest.IsDriver() {
		return fmt.Errorf("package is not a driver")
	}

	// Find the binary (either .wasm or .so)
	var binaryPath string
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return fmt.Errorf("failed to read package directory: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			name := entry.Name()
			if strings.HasSuffix(name, ".wasm") || strings.HasSuffix(name, ".so") {
				binaryPath = filepath.Join(extractDir, name)
				break
			}
		}
	}
	if binaryPath == "" {
		return fmt.Errorf("no binary (.wasm or .so) found in package")
	}

	// Determine runtime
	runtime := "wasm"
	if strings.HasSuffix(binaryPath, ".so") {
		runtime = "native"
	}

	// Create DriverSpec from manifest
	spec := iobus.DriverSpec{
		Name:         fmt.Sprintf("dev-%s-%d", manifest.Package.Name, index),
		Version:      manifest.Package.Version,
		Class:        iobus.ConnectDriver,
		URIPattern:   manifest.Driver.URIPattern,
		Capabilities: []iobus.Capability{iobus.CapCall},
		Runtime:      runtime,
		Binary:       binaryPath,
		Permissions:  []string{"**"}, // Dev mode: full permissions
	}

	// Register the driver with the IOBus
	if err := bus.Register(spec); err != nil {
		return fmt.Errorf("failed to register driver: %w", err)
	}

	return nil
}

// loadWASMApp loads an app package (.wazpkg) and registers it with the IOBus
func loadWASMApp(bus *iobus.IOBus, packagePath string, index int) error {
	// Ensure it's a .wazpkg file
	if !strings.HasSuffix(packagePath, ".wazpkg") {
		return fmt.Errorf("app must be a .wazpkg file, got: %s", packagePath)
	}

	// Extract package to temp directory
	extractDir, cleanup, err := extractPackageToTemp(packagePath)
	if err != nil {
		return fmt.Errorf("failed to extract package: %w", err)
	}
	// Note: We don't cleanup immediately - the binary needs to stay available
	_ = cleanup // Keep reference to avoid unused warning

	// Load metadata.json
	metadataPath := filepath.Join(extractDir, "metadata.json")
	manifest, err := pkg.LoadManifest(metadataPath)
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	// Verify it's an app
	if !manifest.IsApp() {
		return fmt.Errorf("package is not an app")
	}

	// Find the binary (either .wasm or .so)
	var binaryPath string
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return fmt.Errorf("failed to read package directory: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			name := entry.Name()
			if strings.HasSuffix(name, ".wasm") || strings.HasSuffix(name, ".so") {
				binaryPath = filepath.Join(extractDir, name)
				break
			}
		}
	}
	if binaryPath == "" {
		return fmt.Errorf("no binary (.wasm or .so) found in package")
	}

	// Determine runtime
	runtime := "wasm"
	if strings.HasSuffix(binaryPath, ".so") {
		runtime = "native"
	}

	// Get author (default to "local" if not in metadata)
	author := "local"
	if len(manifest.Package.Authors) > 0 {
		author = manifest.Package.Authors[0]
	}

	// Apps register at app://127.0.0.1/{author}/{name}
	appURI := fmt.Sprintf("app://127.0.0.1/%s/%s", author, manifest.Package.Name)

	spec := iobus.DriverSpec{
		Name:         fmt.Sprintf("app-%s-%s-%d", author, manifest.Package.Name, index),
		Version:      manifest.Package.Version,
		Class:        iobus.ConnectDriver, // Apps are connect endpoints
		URIPattern:   appURI,
		Capabilities: []iobus.Capability{iobus.CapCall},
		Runtime:      runtime,
		Binary:       binaryPath,
		Permissions:  []string{"**"}, // Dev mode: full permissions
	}

	// Register the app with the IOBus
	if err := bus.Register(spec); err != nil {
		return fmt.Errorf("failed to register app: %w", err)
	}

	// Store app info for invocation
	loadedAppRegistry = append(loadedAppRegistry, loadedApp{
		name:   manifest.Package.Name,
		author: author,
		uri:    appURI,
	})

	return nil
}
