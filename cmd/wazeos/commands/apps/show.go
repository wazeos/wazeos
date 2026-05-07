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

var showCmd = &cobra.Command{
	Use:   "show <app-id>",
	Short: "Show details of an installed package",
	Long: `Show detailed information about an installed package.

The app-id format is: author/name_version
Example: wazeos/echo_1.0.0

Examples:
  # Show package details
  wazeos apps show wazeos/echo_1.0.0

  # Show as JSON
  wazeos apps show wazeos/echo_1.0.0 --output=json`,
	Args: cobra.ExactArgs(1),
	Run:  runShow,
}

func runShow(cmd *cobra.Command, args []string) {
	appID := args[0]

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
	metadata, err := cli.GetPackage(ctx, appID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: package not found: %v\n", err)
		os.Exit(1)
	}

	// Format output
	format := output.ParseFormat(outputFormat)
	noColor := viper.GetBool("no_color") || os.Getenv("NO_COLOR") != ""
	formatter := output.NewFormatter(format, noColor)

	result, err := formatter.FormatPackageDetails(metadata)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting output: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(result)
}
