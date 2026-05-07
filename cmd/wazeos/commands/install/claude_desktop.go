package install

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	claudeDesktopDataPath string
	claudeDesktopForce    bool
)

var claudeDesktopCmd = &cobra.Command{
	Use:   "claude-desktop",
	Short: "Configure WazeOS as an MCP server for Claude Desktop",
	Long: `Configure WazeOS as a Model Context Protocol (MCP) server for Claude Desktop.

This command will:
  1. Locate your Claude Desktop configuration file
  2. Add WazeOS as an MCP server
  3. Configure the data path and stdio mode
  4. Preserve any existing MCP servers

After running this command, restart Claude Desktop to see WazeOS apps as available tools.

Examples:
  # Install with default data path
  wazeos install claude-desktop

  # Install with custom data path
  wazeos install claude-desktop --data-path ~/my-wazeos-data

  # Force overwrite existing WazeOS configuration
  wazeos install claude-desktop --force`,
	Run: runClaudeDesktop,
}

func init() {
	claudeDesktopCmd.Flags().StringVar(&claudeDesktopDataPath, "data-path", "", "data directory path (default: $HOME/.wazeos/data)")
	claudeDesktopCmd.Flags().BoolVarP(&claudeDesktopForce, "force", "f", false, "force overwrite existing configuration")
}

func runClaudeDesktop(cmd *cobra.Command, args []string) {
	// Get Claude Desktop config path
	configPath, err := getClaudeDesktopConfigPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Get wazeos binary path
	wazeosPath, err := getWazeosBinaryPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Determine data path
	dataPath := claudeDesktopDataPath
	if dataPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			dataPath = "./data"
		} else {
			dataPath = filepath.Join(home, ".wazeos", "data")
		}
	}

	// Ensure data path is absolute
	if !filepath.IsAbs(dataPath) {
		absDataPath, err := filepath.Abs(dataPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving data path: %v\n", err)
			os.Exit(1)
		}
		dataPath = absDataPath
	}

	// Read existing config or create new
	config := make(map[string]interface{})
	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing existing config: %v\n", err)
			os.Exit(1)
		}
	}

	// Get or create mcpServers section
	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		mcpServers = make(map[string]interface{})
		config["mcpServers"] = mcpServers
	}

	// Check if wazeos already exists
	if _, exists := mcpServers["wazeos"]; exists && !claudeDesktopForce {
		fmt.Fprintf(os.Stderr, "Error: WazeOS is already configured in Claude Desktop\n")
		fmt.Fprintf(os.Stderr, "Use --force to overwrite the existing configuration\n")
		os.Exit(1)
	}

	// Add wazeos MCP server configuration
	mcpServers["wazeos"] = map[string]interface{}{
		"command": wazeosPath,
		"args": []string{
			"server",
			"--mode=stdio",
			fmt.Sprintf("--data-path=%s", dataPath),
		},
	}

	// Write config back
	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling config: %v\n", err)
		os.Exit(1)
	}

	// Ensure config directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating config directory: %v\n", err)
		os.Exit(1)
	}

	// Write config file
	if err := os.WriteFile(configPath, configJSON, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
		os.Exit(1)
	}

	// Success message
	fmt.Println("✓ WazeOS configured for Claude Desktop")
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Printf("  Binary:    %s\n", wazeosPath)
	fmt.Printf("  Data path: %s\n", dataPath)
	fmt.Printf("  Config:    %s\n", configPath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Restart Claude Desktop")
	fmt.Println("  2. Install apps: wazeos apps install <app.zip>")
	fmt.Println("  3. Apps will appear as tools in Claude Desktop")
}

// getClaudeDesktopConfigPath returns the path to Claude Desktop's config file
func getClaudeDesktopConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	var configPath string
	switch runtime.GOOS {
	case "darwin":
		configPath = filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	case "linux":
		configPath = filepath.Join(home, ".config", "Claude", "claude_desktop_config.json")
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		configPath = filepath.Join(appData, "Claude", "claude_desktop_config.json")
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return configPath, nil
}

// getWazeosBinaryPath returns the absolute path to the wazeos binary
func getWazeosBinaryPath() (string, error) {
	// Try to find wazeos in PATH
	path, err := exec.LookPath("wazeos")
	if err == nil {
		// Resolve to absolute path
		absPath, err := filepath.Abs(path)
		if err == nil {
			return absPath, nil
		}
		return path, nil
	}

	// Fallback: use current executable path
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to determine wazeos binary path: %w", err)
	}

	// Resolve symlinks
	realPath, err := filepath.EvalSymlinks(exePath)
	if err != nil {
		return exePath, nil
	}

	return realPath, nil
}
