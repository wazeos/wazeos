package drivers

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/wazeos/wazeos/internal/cli/client"
	"github.com/wazeos/wazeos/internal/cli/output"
)

var installCmd = &cobra.Command{
	Use:   "install <driver.zip>",
	Short: "Install a driver",
	Long: `Install a driver package from a ZIP file.

The ZIP file must contain:
  - metadata.json (with type: "driver")
  - app.wasm (the driver binary)

Examples:
  # Install a driver
  wazeos drivers install io.resource.file.zip

  # Install with custom data path
  wazeos drivers install io.resource.file.zip --data-path /var/lib/wazeos`,
	Args: cobra.ExactArgs(1),
	Run:  runInstall,
}

func runInstall(cmd *cobra.Command, args []string) {
	zipPath := args[0]

	// Get data path from viper
	dataPath := viper.GetString("data_path")

	// Read ZIP file
	zipData, err := os.ReadFile(zipPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading driver file: %v\n", err)
		os.Exit(1)
	}

	// Create client
	cli, err := client.NewDirectClient(dataPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer cli.Close()

	// Install package
	ctx := context.Background()
	metadata, err := cli.InstallPackage(ctx, zipData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error installing driver: %v\n", err)
		os.Exit(1)
	}

	// Verify it's a driver
	if metadata.Type != "driver" {
		fmt.Fprintf(os.Stderr, "Warning: package type is '%s', not 'driver'\n", metadata.Type)
	}

	// Format success message
	format := output.ParseFormat(outputFormat)
	noColor := viper.GetBool("no_color") || os.Getenv("NO_COLOR") != ""
	formatter := output.NewFormatter(format, noColor)

	message := fmt.Sprintf("Successfully installed %s", metadata.AppID())
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
