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

var showCmd = &cobra.Command{
	Use:   "show <driver-id>",
	Short: "Show driver details",
	Long: `Show detailed information about an installed driver.

The driver ID format is: author/name_version

Examples:
  # Show driver details
  wazeos drivers show wazeos/io.resource.file_1.0.0

  # Output as JSON
  wazeos drivers show wazeos/io.resource.file_1.0.0 --output json`,
	Args: cobra.ExactArgs(1),
	Run:  runShow,
}

func runShow(cmd *cobra.Command, args []string) {
	driverID := args[0]

	// Get data path from viper
	dataPath := viper.GetString("data_path")

	// Create client
	cli, err := client.NewDirectClient(dataPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer cli.Close()

	// Get package
	ctx := context.Background()
	pkg, err := cli.GetPackage(ctx, driverID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Verify it's a driver
	if pkg.Type != "driver" {
		fmt.Fprintf(os.Stderr, "Error: %s is not a driver (type: %s)\n", driverID, pkg.Type)
		os.Exit(1)
	}

	// Format output
	format := output.ParseFormat(outputFormat)
	noColor := viper.GetBool("no_color") || os.Getenv("NO_COLOR") != ""
	formatter := output.NewFormatter(format, noColor)

	result, err := formatter.FormatPackageDetails(pkg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting output: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(result)
}
