package install

import (
	"github.com/spf13/cobra"
)

// InstallCmd is the parent command for installation tasks
var InstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install and configure WazeOS integrations",
	Long: `Install and configure WazeOS integrations with various platforms.

Examples:
  # Configure for Claude Code (global)
  wazeos install claude-code

  # Configure for Claude Desktop
  wazeos install claude-desktop`,
}

func init() {
	// Add subcommands
	InstallCmd.AddCommand(claudeCodeCmd)
	InstallCmd.AddCommand(claudeDesktopCmd)
}
