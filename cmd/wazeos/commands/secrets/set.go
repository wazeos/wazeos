package secrets

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/wazeos/wazeos/internal/cli/client"
	"github.com/wazeos/wazeos/internal/cli/output"
)

var setCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a secret value",
	Long: `Set a secret value.

The secret will be stored securely and can be retrieved later.

Examples:
  # Set a secret with key and value
  wazeos secrets set mykey myvalue

  # Set an API key
  wazeos secrets set api.openai.key sk-...

  # Set a secret from stdin (value starting with -)
  echo "secret-value" | wazeos secrets set mykey -`,
	Args: cobra.ExactArgs(2),
	Run:  runSet,
}

func runSet(cmd *cobra.Command, args []string) {
	key := args[0]
	value := args[1]

	// Get data path from viper
	dataPath := viper.GetString("data_path")

	// Create client
	cli, err := client.NewDirectClient(dataPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer cli.Close()

	// Set secret
	ctx := context.Background()
	if err := cli.SetSecret(ctx, key, value); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set secret: %v\n", err)
		os.Exit(1)
	}

	// Format success message
	format := output.ParseFormat(outputFormat)
	noColor := viper.GetBool("no_color") || os.Getenv("NO_COLOR") != ""
	formatter := output.NewFormatter(format, noColor)

	message := fmt.Sprintf("Secret set: %s", key)
	fmt.Println(formatter.FormatSuccess(message))
}
