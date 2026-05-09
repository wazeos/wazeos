package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var mcpInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install WazeOS into Claude Desktop's MCP configuration",
	Long: `Automatically configures Claude Desktop to use WazeOS as an MCP server.

This command:
  1. Locates the Claude Desktop configuration directory
  2. Creates or updates the mcp_servers.json file
  3. Adds WazeOS with the correct binary path
  4. Preserves existing MCP server configurations

After running this command, restart Claude Desktop to enable WazeOS tools.
`,
	RunE: runMCPInstall,
}

var (
	mcpServerName string
	forceInstall  bool
)

func init() {
	mcpCmd.AddCommand(mcpInstallCmd)

	mcpInstallCmd.Flags().StringVar(&mcpServerName, "name", "wazeos", "Name for the MCP server entry")
	mcpInstallCmd.Flags().BoolVar(&forceInstall, "force", false, "Overwrite existing WazeOS configuration")
}

// MCPServersConfig represents the Claude Desktop MCP configuration
type MCPServersConfig map[string]MCPServerEntry

// MCPServerEntry represents a single MCP server configuration
type MCPServerEntry struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}

func runMCPInstall(cmd *cobra.Command, args []string) error {
	// Get the path to the wazeos binary
	wazeosPath, err := getWazeOSPath()
	if err != nil {
		return fmt.Errorf("failed to locate wazeos binary: %w", err)
	}

	logInfo("Found wazeos binary: %s", wazeosPath)

	// Get Claude Desktop config path
	configPath := getClaudeDesktopConfigPath()
	configDir := filepath.Dir(configPath)

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Read existing configuration
	config, err := readMCPConfig(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read existing config: %w", err)
	}
	if config == nil {
		config = make(MCPServersConfig)
	}

	// Check if wazeos is already configured
	if existing, ok := config[mcpServerName]; ok {
		if !forceInstall {
			logInfo("✓ WazeOS is already configured in Claude Desktop")
			logInfo("  Name: %s", mcpServerName)
			logInfo("  Command: %s", existing.Command)
			logInfo("")
			logInfo("To update the configuration, run:")
			logInfo("  wazeos mcp install --force")
			return nil
		}
		logInfo("Updating existing configuration...")
	}

	// Add or update WazeOS configuration
	config[mcpServerName] = MCPServerEntry{
		Command: wazeosPath,
		Args:    []string{"mcp", "server"},
	}

	// Write configuration back
	if err := writeMCPConfig(configPath, config); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	logSuccess("✓", "WazeOS installed to Claude Desktop")
	logInfo("")
	logInfo("Configuration written to: %s", configPath)
	logInfo("Server name: %s", mcpServerName)
	logInfo("")

	// Show installed apps
	installedCount := countInstalledApps()
	if installedCount > 0 {
		logInfo("📦 %d WazeOS app(s) installed and ready to use", installedCount)
	} else {
		logInfo("⚠️  No WazeOS apps installed yet")
		logInfo("")
		logInfo("Create your first app:")
		logInfo("  wazeos app new my-tool")
		logInfo("  cd apps/my-tool")
		logInfo("  cargo build --target wasm32-wasip1 --release")
		logInfo("  cd ../..")
		logInfo("  wazeos app install my-tool")
		logInfo("")
	}

	logNextSteps(
		"Restart Claude Desktop to enable WazeOS tools",
		"Check server logs at: /tmp/wazeos-mcp.log",
		"View installed apps: wazeos app list",
	)

	return nil
}

// getWazeOSPath returns the absolute path to the wazeos binary
func getWazeOSPath() (string, error) {
	// First, check if we're running from the current directory
	if exe, err := os.Executable(); err == nil {
		if abs, err := filepath.Abs(exe); err == nil {
			// Check if it's actually named 'wazeos'
			if filepath.Base(abs) == "wazeos" {
				return abs, nil
			}
		}
	}

	// Try to find wazeos in PATH
	if path, err := findInPath("wazeos"); err == nil {
		return path, nil
	}

	// Try common installation locations
	homeDir, err := os.UserHomeDir()
	if err == nil {
		commonPaths := []string{
			filepath.Join(homeDir, "bin", "wazeos"),
			filepath.Join(homeDir, ".local", "bin", "wazeos"),
			"/usr/local/bin/wazeos",
			"/usr/bin/wazeos",
		}

		for _, path := range commonPaths {
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("wazeos binary not found")
}

// findInPath searches for an executable in PATH
func findInPath(name string) (string, error) {
	path := os.Getenv("PATH")
	if path == "" {
		return "", fmt.Errorf("PATH environment variable not set")
	}

	for _, dir := range filepath.SplitList(path) {
		fullPath := filepath.Join(dir, name)
		if info, err := os.Stat(fullPath); err == nil {
			if !info.IsDir() && info.Mode()&0111 != 0 { // Check if executable
				return fullPath, nil
			}
		}
	}

	return "", fmt.Errorf("%s not found in PATH", name)
}

// getClaudeDesktopConfigPath returns the path to Claude Desktop's MCP config
func getClaudeDesktopConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to environment variable
		homeDir = os.Getenv("HOME")
	}

	// Claude Desktop stores config at ~/.config/claude/mcp_servers.json
	return filepath.Join(homeDir, ".config", "claude", "mcp_servers.json")
}

// readMCPConfig reads the MCP servers configuration file
func readMCPConfig(path string) (MCPServersConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config MCPServersConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("invalid JSON in config file: %w", err)
	}

	return config, nil
}

// writeMCPConfig writes the MCP servers configuration file
func writeMCPConfig(path string, config MCPServersConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	// Add newline at end of file
	data = append(data, '\n')

	return os.WriteFile(path, data, 0644)
}

// countInstalledApps returns the number of installed WazeOS apps
func countInstalledApps() int {
	appsDir := filepath.Join(os.Getenv("HOME"), ".wazeos", "apps")
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return 0
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			count++
		}
	}
	return count
}
