package drivers

import (
	"github.com/spf13/cobra"
)

var (
	outputFormat string
	dataPath     string
)

// DriversCmd is the parent command for driver management
var DriversCmd = &cobra.Command{
	Use:   "drivers",
	Short: "Manage WazeOS drivers",
	Long: `Manage WazeOS resource drivers.

Drivers are specialized packages that provide access to external resources
like filesystems, databases, APIs, and more.

Examples:
  # List all drivers
  wazeos drivers list

  # Show driver details
  wazeos drivers show wazeos/io.resource.file_1.0.0

  # Install a driver
  wazeos drivers install driver.zip`,
}

func init() {
	// Add subcommands
	DriversCmd.AddCommand(newCmd)
	DriversCmd.AddCommand(listCmd)
	DriversCmd.AddCommand(showCmd)
	DriversCmd.AddCommand(installCmd)
	DriversCmd.AddCommand(uninstallCmd)

	// Add persistent flags
	DriversCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "output format (table, json, yaml)")
	DriversCmd.PersistentFlags().StringVarP(&dataPath, "data-path", "d", "", "data directory path")
}
