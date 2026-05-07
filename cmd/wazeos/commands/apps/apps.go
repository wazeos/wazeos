package apps

import (
	"github.com/spf13/cobra"
)

var (
	outputFormat string
)

// AppsCmd represents the apps command
var AppsCmd = &cobra.Command{
	Use:   "apps",
	Short: "Manage WazeOS applications and drivers",
	Long: `Manage WazeOS applications and drivers.

Applications are WebAssembly binaries that can be invoked as MCP tools.
Drivers extend the platform with new resource capabilities.

Examples:
  # List all installed packages
  wazeos apps list

  # Install a package
  wazeos apps install myapp.zip

  # Show package details
  wazeos apps show wazeos/echo_1.0.0

  # Uninstall a package
  wazeos apps uninstall wazeos/echo_1.0.0`,
}

func init() {
	// Add subcommands
	AppsCmd.AddCommand(newCmd)
	AppsCmd.AddCommand(buildCmd)
	AppsCmd.AddCommand(packageCmd)
	AppsCmd.AddCommand(listCmd)
	AppsCmd.AddCommand(installCmd)
	AppsCmd.AddCommand(showCmd)
	AppsCmd.AddCommand(uninstallCmd)
	AppsCmd.AddCommand(validateCmd)

	// Persistent flags for all apps subcommands
	AppsCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "output format (table|json|yaml)")
}
