package secrets

import (
	"github.com/spf13/cobra"
)

var (
	outputFormat string
	dataPath     string
)

// SecretsCmd is the parent command for secrets management
var SecretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Manage secrets",
	Long: `Manage secrets stored in WazeOS.

Secrets are key-value pairs that can be securely stored and retrieved.
They are typically used for API keys, credentials, and other sensitive data.

Examples:
  # Set a secret
  wazeos secrets set mykey myvalue

  # Get a secret
  wazeos secrets get mykey

  # List all secrets
  wazeos secrets list`,
}

func init() {
	// Add subcommands
	SecretsCmd.AddCommand(setCmd)
	SecretsCmd.AddCommand(getCmd)
	SecretsCmd.AddCommand(listCmd)
	SecretsCmd.AddCommand(deleteCmd)
	SecretsCmd.AddCommand(matchCmd)

	// Add persistent flags
	SecretsCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "output format (table, json, yaml)")
	SecretsCmd.PersistentFlags().StringVarP(&dataPath, "data-path", "d", "", "data directory path")
}
