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

var (
	installForce bool
)

var installCmd = &cobra.Command{
	Use:   "install <package.zip>",
	Short: "Install a package from a ZIP file",
	Long: `Install a WebAssembly application or driver from a ZIP file.

The ZIP file must contain:
  - app.wasm: The WebAssembly binary
  - metadata.json: Package metadata (optional if embedded in WASM)

Examples:
  # Install a package
  wazeos apps install myapp.zip

  # Force reinstall (overwrite existing)
  wazeos apps install myapp.zip --force`,
	Args: cobra.ExactArgs(1),
	Run:  runInstall,
}

func init() {
	installCmd.Flags().BoolVar(&installForce, "force", false, "overwrite if package already exists")
}

func runInstall(cmd *cobra.Command, args []string) {
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

	// Install package
	ctx := context.Background()
	metadata, err := cli.InstallPackage(ctx, zipData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error installing package: %v\n", err)
		os.Exit(1)
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
