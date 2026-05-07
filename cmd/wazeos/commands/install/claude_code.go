package install

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	claudeCodeDataPath string
	claudeCodeForce    bool
)

var claudeCodeCmd = &cobra.Command{
	Use:   "claude-code",
	Short: "Configure WazeOS as a global MCP server for Claude Code",
	Long: `Configure WazeOS as a global Model Context Protocol (MCP) server for Claude Code.

This command will:
  1. Locate your Claude Code configuration file (~/.claude.json)
  2. Add WazeOS as a global MCP server
  3. Configure the data path and stdio mode
  4. Make it available in all Claude Code projects

After running this command, WazeOS apps will be available as tools in all Claude Code sessions.

Examples:
  # Install globally with default data path
  wazeos install claude-code

  # Install with custom data path
  wazeos install claude-code --data-path ~/my-wazeos-data

  # Force overwrite existing WazeOS configuration
  wazeos install claude-code --force`,
	Run: runClaudeCode,
}

func init() {
	claudeCodeCmd.Flags().StringVar(&claudeCodeDataPath, "data-path", "", "data directory path (default: $HOME/.wazeos/data)")
	claudeCodeCmd.Flags().BoolVarP(&claudeCodeForce, "force", "f", false, "force overwrite existing configuration")
}

func runClaudeCode(cmd *cobra.Command, args []string) {
	// Get Claude Code config path
	configPath, err := getClaudeCodeConfigPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Get wazeos binary path
	wazeosPath, err := getWazeosBinaryPathForClaudeCode()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Determine data path
	dataPath := claudeCodeDataPath
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
	} else {
		fmt.Fprintf(os.Stderr, "Error: Claude Code config not found at %s\n", configPath)
		fmt.Fprintf(os.Stderr, "Please run Claude Code at least once to create the config file.\n")
		os.Exit(1)
	}

	// Get or create global mcpServers section
	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		mcpServers = make(map[string]interface{})
		config["mcpServers"] = mcpServers
	}

	// Check if wazeos already exists
	if _, exists := mcpServers["wazeos"]; exists && !claudeCodeForce {
		fmt.Fprintf(os.Stderr, "Error: WazeOS is already configured as a global MCP server in Claude Code\n")
		fmt.Fprintf(os.Stderr, "Use --force to overwrite the existing configuration\n")
		os.Exit(1)
	}

	// Add wazeos global MCP server configuration
	mcpServers["wazeos"] = map[string]interface{}{
		"type":    "stdio",
		"command": wazeosPath,
		"args": []string{
			"server",
			"--mode=stdio",
			fmt.Sprintf("--data-path=%s", dataPath),
		},
		"env": map[string]string{},
	}

	// Write config back
	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling config: %v\n", err)
		os.Exit(1)
	}

	// Write config file
	if err := os.WriteFile(configPath, configJSON, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
		os.Exit(1)
	}

	// Success message
	fmt.Println("✓ WazeOS configured globally for Claude Code")
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Printf("  Binary:    %s\n", wazeosPath)
	fmt.Printf("  Data path: %s\n", dataPath)
	fmt.Printf("  Config:    %s\n", configPath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Install apps: wazeos apps install <app.zip>")
	fmt.Println("  2. Apps will appear as tools in all Claude Code sessions")
	fmt.Println("  3. Tools are available globally across all projects")
}

// getClaudeCodeConfigPath returns the path to Claude Code's config file
func getClaudeCodeConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(home, ".claude.json")
	return configPath, nil
}

// getWazeosBinaryPathForClaudeCode returns the absolute path to the wazeos binary
func getWazeosBinaryPathForClaudeCode() (string, error) {
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
