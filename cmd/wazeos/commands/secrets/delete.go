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

var (
	forceDelete bool
)

var deleteCmd = &cobra.Command{
	Use:   "delete <key>",
	Short: "Delete a secret",
	Long: `Delete a secret by key.

Use the --force flag to skip confirmation.

Examples:
  # Delete a secret (with confirmation)
  wazeos secrets delete mykey

  # Delete a secret without confirmation
  wazeos secrets delete mykey --force`,
	Args: cobra.ExactArgs(1),
	Run:  runDelete,
}

func init() {
	deleteCmd.Flags().BoolVarP(&forceDelete, "force", "f", false, "skip confirmation")
}

func runDelete(cmd *cobra.Command, args []string) {
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

	// Confirm deletion unless --force is used
	if !forceDelete {
		fmt.Printf("Delete secret '%s'? [y/N]: ", key)
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Cancelled")
			return
		}
	}

	// Delete secret
	ctx := context.Background()
	if err := cli.DeleteSecret(ctx, key); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to delete secret: %v\n", err)
		os.Exit(1)
	}

	// Format success message
	format := output.ParseFormat(outputFormat)
	noColor := viper.GetBool("no_color") || os.Getenv("NO_COLOR") != ""
	formatter := output.NewFormatter(format, noColor)

	message := fmt.Sprintf("Secret deleted: %s", key)
	fmt.Println(formatter.FormatSuccess(message))
}
