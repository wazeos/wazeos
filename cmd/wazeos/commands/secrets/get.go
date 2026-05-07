package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/wazeos/wazeos/internal/cli/client"
	"github.com/wazeos/wazeos/internal/cli/output"
)

var getCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a secret value",
	Long: `Get a secret value.

The secret value will be printed to stdout. Use the --output flag to control the format.

Examples:
  # Get a secret
  wazeos secrets get mykey

  # Get a secret as JSON
  wazeos secrets get mykey --output json`,
	Args: cobra.ExactArgs(1),
	Run:  runGet,
}

func runGet(cmd *cobra.Command, args []string) {
	key := args[0]

	// Get data path from viper
	dataPath := viper.GetString("data_path")

	// Create client
	cli, err := client.NewDirectClient(dataPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer cli.Close()

	// Get secret
	ctx := context.Background()
	value, err := cli.GetSecret(ctx, key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get secret: %v\n", err)
		os.Exit(1)
	}

	// Format output
	format := output.ParseFormat(outputFormat)
	switch format {
	case output.FormatJSON:
		data := map[string]interface{}{
			"key":   key,
			"value": value,
		}
		jsonData, _ := json.MarshalIndent(data, "", "  ")
		fmt.Println(string(jsonData))
	case output.FormatYAML:
		fmt.Printf("key: %s\n", key)
		fmt.Printf("value: %v\n", value)
	default:
		// Table format - just print the value
		fmt.Println(value)
	}
}
