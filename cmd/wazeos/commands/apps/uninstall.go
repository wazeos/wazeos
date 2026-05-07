package apps

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
	uninstallForce bool
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall <app-id>",
	Short: "Uninstall a package",
	Long: `Uninstall an installed package.

The app-id format is: author/name:version
Version is optional - if omitted, will use an available version.

The command will fail if other packages depend on this package,
unless --force is used.

Examples:
  # Uninstall a specific version
  wazeos apps uninstall wazeos/echo:1.0.0

  # Uninstall (version auto-resolved)
  wazeos apps uninstall wazeos/echo

  # Force uninstall (ignore dependencies)
  wazeos apps uninstall wazeos/echo:1.0.0 --force`,
	Args: cobra.ExactArgs(1),
	Run:  runUninstall,
}

func init() {
	uninstallCmd.Flags().BoolVar(&uninstallForce, "force", false, "ignore dependency checks")
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
	appID, err := cli.ResolvePackage(ctx, nameOrID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Uninstall package
	err = cli.UninstallPackage(ctx, appID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error uninstalling package: %v\n", err)
		os.Exit(1)
	}

	// Format success message
	format := output.ParseFormat(outputFormat)
	noColor := viper.GetBool("no_color") || os.Getenv("NO_COLOR") != ""
	formatter := output.NewFormatter(format, noColor)

	message := fmt.Sprintf("Successfully uninstalled %s", appID)
	fmt.Println(formatter.FormatSuccess(message))
}
