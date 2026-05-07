package apps

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
	listType   string
	listAuthor string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed applications",
	Long: `List installed applications (excludes drivers by default).

Use --type=all to show both apps and drivers, or use 'wazeos drivers list' for drivers only.

Examples:
  # List only applications (default)
  wazeos apps list

  # List all packages (apps and drivers)
  wazeos apps list --type=all

  # List packages by author
  wazeos apps list --author=wazeos

  # Output as JSON
  wazeos apps list --output=json`,
	Run: runList,
}

func init() {
	listCmd.Flags().StringVar(&listType, "type", "", "filter by type (app|driver|all)")
	listCmd.Flags().StringVar(&listAuthor, "author", "", "filter by author")
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

	// List packages
	ctx := context.Background()
	packages, err := cli.ListPackages(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing packages: %v\n", err)
		os.Exit(1)
	}

	// Filter packages
	// By default, only show apps (not drivers)
	filtered := make([]*types.AppMetadata, 0, len(packages))
	for _, pkg := range packages {
		// Type filter
		if listType == "" {
			// Default: only show apps
			if pkg.Type != "app" {
				continue
			}
		} else if listType != "all" {
			// Explicit type filter
			if string(pkg.Type) != listType {
				continue
			}
		}
		// else: listType == "all", show everything

		// Author filter
		if listAuthor != "" && pkg.Author != listAuthor {
			continue
		}

		filtered = append(filtered, pkg)
	}

	// Format output
	format := output.ParseFormat(outputFormat)
	noColor := viper.GetBool("no_color") || os.Getenv("NO_COLOR") != ""
	formatter := output.NewFormatter(format, noColor)

	result, err := formatter.FormatPackageList(filtered)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting output: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(result)
}
