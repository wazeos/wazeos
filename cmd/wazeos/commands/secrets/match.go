package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	"github.com/wazeos/wazeos/internal/cli/client"
	"github.com/wazeos/wazeos/internal/cli/output"
)

var matchCmd = &cobra.Command{
	Use:   "match <prefix>",
	Short: "Match secrets by prefix",
	Long: `Match secrets by key prefix and show their values.

This returns both keys and values (unlike 'list' which only shows keys).

Examples:
  # Match all secrets starting with "api."
  wazeos secrets match api.

  # Match all secrets (empty prefix)
  wazeos secrets match ""

  # Match secrets as JSON
  wazeos secrets match api. --output json`,
	Args: cobra.ExactArgs(1),
	Run:  runMatch,
}

func runMatch(cmd *cobra.Command, args []string) {
	prefix := args[0]

	// Get data path from viper
	dataPath := viper.GetString("data_path")

	// Create client
	cli, err := client.NewDirectClient(dataPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer cli.Close()

	// Match secrets
	ctx := context.Background()
	matches, err := cli.MatchSecrets(ctx, prefix)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to match secrets: %v\n", err)
		os.Exit(1)
	}

	// Format output
	format := output.ParseFormat(outputFormat)
	noColor := viper.GetBool("no_color") || os.Getenv("NO_COLOR") != ""

	switch format {
	case output.FormatJSON:
		data := map[string]interface{}{
			"matches": matches,
			"count":   len(matches),
		}
		jsonData, _ := json.MarshalIndent(data, "", "  ")
		fmt.Println(string(jsonData))

	case output.FormatYAML:
		data := map[string]interface{}{
			"matches": matches,
			"count":   len(matches),
		}
		yamlData, _ := yaml.Marshal(data)
		fmt.Print(string(yamlData))

	default:
		// Table format
		if len(matches) == 0 {
			formatter := output.NewFormatter(format, noColor)
			fmt.Println(formatter.FormatSuccess("No matching secrets"))
			return
		}

		// Sort keys for consistent output
		keys := make([]string, 0, len(matches))
		for k := range matches {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		// Print table header
		fmt.Printf("%-40s %s\n", "KEY", "VALUE")
		fmt.Println("────────────────────────────────────────────────────────────────")

		// Print matches
		for _, key := range keys {
			value := matches[key]
			valueStr := fmt.Sprintf("%v", value)
			// Truncate long values
			if len(valueStr) > 50 {
				valueStr = valueStr[:47] + "..."
			}
			fmt.Printf("%-40s %s\n", key, valueStr)
		}

		fmt.Printf("\nTotal: %d match(es)\n", len(matches))
	}
}
