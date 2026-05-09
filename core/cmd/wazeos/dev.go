package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
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

This command loads drivers and apps from local paths without affecting
installed packages. Perfect for development and testing interactions
between multiple components.

Examples:
  # Load a driver and invoke an app tool
  wazeos dev run --driver drivers/acme/api/build/api.so \
                 --app apps/test/tool/target/wasm32-wasip1/release/tool.wasm \
                 --invoke test-tool '{"input":"value"}'

  # Interactive mode with multiple drivers
  wazeos dev run --driver drivers/db/build/db.so \
                 --driver drivers/cache/build/cache.so \
                 --app apps/admin/tool.wasm \
                 --interactive

  # Verbose logging for debugging
  wazeos dev run -v --driver ./my-driver.so --app ./my-app.wasm --interactive`,
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

func init() {
	devServeCmd.Flags().IntVar(&devPort, "port", 8080, "Server port")

	// dev run flags
	devRunCmd.Flags().StringSliceVar(&devDrivers, "driver", []string{}, "Load driver from path (repeatable)")
	devRunCmd.Flags().StringSliceVar(&devApps, "app", []string{}, "Load app from path (repeatable)")
	devRunCmd.Flags().StringVar(&devInvoke, "invoke", "", "Tool to invoke (format: app-name/tool-name)")
	devRunCmd.Flags().StringVar(&devInvokeArgs, "args", "{}", "JSON arguments for tool invocation")
	devRunCmd.Flags().BoolVarP(&devInteractive, "interactive", "i", false, "Start interactive REPL")

	devCmd.AddCommand(devServeCmd)
	devCmd.AddCommand(devInspectCmd)
	devCmd.AddCommand(devDebugCmd)
	devCmd.AddCommand(devRunCmd)
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
	// Enable verbose logging if requested
	var logger *slog.Logger
	if verbose {
		log.SetFlags(log.Ltime | log.Lmicroseconds | log.Lshortfile)
		log.SetOutput(os.Stdout)
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}

	logInfo("Creating isolated test environment...")

	// Create isolated IOBus
	bus := iobus.NewIOBus(logger)
	logInfo("✓ Created isolated IOBus")

	// Load drivers
	if len(devDrivers) > 0 {
		logInfo("Loading %d driver(s)...", len(devDrivers))
		for i, driverPath := range devDrivers {
			if _, err := os.Stat(driverPath); os.IsNotExist(err) {
				outputError("dev run", "DRIVER_NOT_FOUND",
					fmt.Sprintf("driver not found: %s", driverPath), "")
			}
			logInfo("  [%d/%d] Loading: %s", i+1, len(devDrivers), driverPath)
			// TODO: Load plugin driver
			// For now, just validate it exists
			logSuccess("    ✓", "Driver validated")
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
			// TODO: Load WASM app
			// For now, just validate it exists
			logSuccess("    ✓", "App validated")
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

	// TODO: Actually invoke the tool
	// For now, just log what we would do
	logInfo("")
	logSuccess("✓", "Tool invocation completed (stub)")

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
			logInfo("Invoking: %s with args: %s", toolPath, argsJSON)
			// TODO: Actually invoke
			logInfo("(Invocation not yet implemented)")

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
