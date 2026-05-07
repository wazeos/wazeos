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

var validateCmd = &cobra.Command{
	Use:   "validate <package.zip>",
	Short: "Validate a package without installing",
	Long: `Validate a package ZIP file without installing it.

This checks that the package:
  - Contains required files (app.wasm, metadata.json)
  - Has valid metadata
  - Has valid input schema (if present)
  - Dependencies are satisfied

Examples:
  # Validate a package
  wazeos apps validate myapp.zip`,
	Args: cobra.ExactArgs(1),
	Run:  runValidate,
}

func runValidate(cmd *cobra.Command, args []string) {
	zipPath := args[0]

	// Get data path from viper
	dataPath := viper.GetString("data_path")

	// Read ZIP file
	zipData, err := os.ReadFile(zipPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading package file: %v\n", err)
		os.Exit(1)
	}

	// Create client
	cli, err := client.NewDirectClient(dataPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer cli.Close()

	// Validate package
	ctx := context.Background()
	metadata, err := cli.ValidatePackage(ctx, zipData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Validation failed: %v\n", err)
		os.Exit(1)
	}

	// Format success message
	format := output.ParseFormat(outputFormat)
	noColor := viper.GetBool("no_color") || os.Getenv("NO_COLOR") != ""
	formatter := output.NewFormatter(format, noColor)

	message := fmt.Sprintf("Package is valid: %s/%s_%s", metadata.Author, metadata.Name, metadata.Version)
	fmt.Println(formatter.FormatSuccess(message))

	// Show package details if not in quiet mode
	if !viper.GetBool("quiet") {
		fmt.Println()
		result, err := formatter.FormatPackageDetails(metadata)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error formatting output: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(result)
	}
}
