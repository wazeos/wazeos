package drivers

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/wazeos/wazeos/internal/cli/client"
	"github.com/wazeos/wazeos/internal/cli/output"
)

var (
	forceUninstall bool
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall <driver-id>",
	Short: "Uninstall a driver",
	Long: `Uninstall an installed driver.

The driver ID format is: author/name:version
Version is optional - if omitted, will use an available version.

Examples:
  # Uninstall a specific version
  wazeos drivers uninstall wazeos/io.resource.file:1.0.0

  # Uninstall (version auto-resolved)
  wazeos drivers uninstall wazeos/io.resource.file

  # Uninstall without confirmation
  wazeos drivers uninstall wazeos/io.resource.file:1.0.0 --force`,
	Args: cobra.ExactArgs(1),
	Run:  runUninstall,
}

func init() {
	uninstallCmd.Flags().BoolVarP(&forceUninstall, "force", "f", false, "skip confirmation")
}

func runUninstall(cmd *cobra.Command, args []string) {
	nameOrID := args[0]

	// Get data path from viper
	dataPath := viper.GetString("data_path")

	// Create client
	cli, err := client.NewDirectClient(dataPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer cli.Close()

	// Resolve to full ID (handles partial IDs and names)
	ctx := context.Background()
	driverID, err := cli.ResolvePackage(ctx, nameOrID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Verify it exists and is a driver
	pkg, err := cli.GetPackage(ctx, driverID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if pkg.Type != "driver" {
		fmt.Fprintf(os.Stderr, "Error: %s is not a driver (type: %s)\n", driverID, pkg.Type)
		os.Exit(1)
	}

	// Confirm uninstall unless --force is used
	if !forceUninstall {
		fmt.Printf("Uninstall driver '%s'? [y/N]: ", driverID)
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Cancelled")
			return
		}
	}

	// Uninstall package
	if err := cli.UninstallPackage(ctx, driverID); err != nil {
		fmt.Fprintf(os.Stderr, "Error uninstalling driver: %v\n", err)
		os.Exit(1)
	}

	// Format success message
	format := output.ParseFormat(outputFormat)
	noColor := viper.GetBool("no_color") || os.Getenv("NO_COLOR") != ""
	formatter := output.NewFormatter(format, noColor)

	message := fmt.Sprintf("Successfully uninstalled %s", driverID)
	fmt.Println(formatter.FormatSuccess(message))
}
