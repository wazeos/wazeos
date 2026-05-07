package secrets

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/wazeos/wazeos/internal/cli/client"
	"github.com/wazeos/wazeos/internal/cli/output"
)

var (
	prefixFilter string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List secret keys",
	Long: `List all secret keys.

Use the --prefix flag to filter secrets by key prefix.

Examples:
  # List all secrets
  wazeos secrets list

  # List secrets with a specific prefix
  wazeos secrets list --prefix api.

  # List secrets as JSON
  wazeos secrets list --output json`,
	Args: cobra.NoArgs,
	Run:  runList,
}

func init() {
	listCmd.Flags().StringVar(&prefixFilter, "prefix", "", "filter by key prefix")
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

	// List secrets
	ctx := context.Background()
	keys, err := cli.ListSecrets(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list secrets: %v\n", err)
		os.Exit(1)
	}

	// Apply prefix filter if specified
	if prefixFilter != "" {
		filtered := make([]string, 0)
		for _, key := range keys {
			if strings.HasPrefix(key, prefixFilter) {
				filtered = append(filtered, key)
			}
		}
		keys = filtered
	}

	// Format output
	format := output.ParseFormat(outputFormat)
	noColor := viper.GetBool("no_color") || os.Getenv("NO_COLOR") != ""
	formatter := output.NewFormatter(format, noColor)

	result, err := formatter.FormatSecretList(keys)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting output: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(result)
}
