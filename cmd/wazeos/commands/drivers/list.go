package drivers

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/wazeos/wazeos/internal/cli/client"
	"github.com/wazeos/wazeos/internal/cli/output"
	"github.com/wazeos/wazeos/internal/types"
)

var (
	authorFilter string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed drivers",
	Long: `List all installed drivers.

Examples:
  # List all drivers
  wazeos drivers list

  # Filter by author
  wazeos drivers list --author wazeos

  # Output as JSON
  wazeos drivers list --output json`,
	Args: cobra.NoArgs,
	Run:  runList,
}

func init() {
	listCmd.Flags().StringVar(&authorFilter, "author", "", "filter by author")
}

func runList(cmd *cobra.Command, args []string) {
	// Get data path from viper
	dataPath := viper.GetString("data_path")

	// Create client
	cli, err := client.NewDirectClient(dataPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer cli.Close()

	// List all packages
	ctx := context.Background()
	allPackages, err := cli.ListPackages(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing packages: %v\n", err)
		os.Exit(1)
	}

	// Filter for drivers only
	var drivers []*types.AppMetadata
	for _, pkg := range allPackages {
		if pkg.Type == "driver" {
			// Apply author filter if specified
			if authorFilter != "" && pkg.Author != authorFilter {
				continue
			}
			drivers = append(drivers, pkg)
		}
	}

	// Format output
	format := output.ParseFormat(outputFormat)
	noColor := viper.GetBool("no_color") || os.Getenv("NO_COLOR") != ""
	formatter := output.NewFormatter(format, noColor)

	result, err := formatter.FormatPackageList(drivers)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting output: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(result)
}
