package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/wazeos/wazeos/internal/cli/client"
	"github.com/wazeos/wazeos/internal/cli/output"
)

var (
	installForce bool
)

var installCmd = &cobra.Command{
	Use:   "install <package.zip|directory>",
	Short: "Install a package from a ZIP file or build and install from directory",
	Long: `Install a WebAssembly application or driver from a ZIP file or directory.

If a ZIP file is provided, it will be installed directly.
If a directory is provided, the app will be built, packaged, and installed.

The ZIP file must contain:
  - app.wasm: The WebAssembly binary
  - metadata.json: Package metadata (optional if embedded in WASM)

Examples:
  # Install from ZIP file
  wazeos apps install myapp.zip

  # Build and install from directory
  wazeos apps install .
  wazeos apps install bin/mycompany/myapp

  # Force reinstall (overwrite existing)
  wazeos apps install myapp.zip --force`,
	Args: cobra.ExactArgs(1),
	Run:  runInstall,
}

func init() {
	installCmd.Flags().BoolVar(&installForce, "force", false, "overwrite if package already exists")
}

func runInstall(cmd *cobra.Command, args []string) {
	path := args[0]

	// Check if path is a directory or file
	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var zipPath string
	if info.IsDir() {
		// Build and package the app first
		fmt.Println("Building and packaging app...")
		runPackage(cmd, []string{path})

		// Find the generated ZIP file
		absDir, _ := filepath.Abs(path)
		metadataFile := filepath.Join(absDir, "metadata.json")
		data, err := os.ReadFile(metadataFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading metadata.json: %v\n", err)
			os.Exit(1)
		}

		var metadata map[string]interface{}
		if err := json.Unmarshal(data, &metadata); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing metadata.json: %v\n", err)
			os.Exit(1)
		}

		appName, ok := metadata["name"].(string)
		if !ok || appName == "" {
			fmt.Fprintf(os.Stderr, "Error: metadata.json missing 'name' field\n")
			os.Exit(1)
		}

		zipPath = filepath.Join(absDir, appName+".zip")
		fmt.Printf("\n→ Installing from %s...\n", zipPath)
	} else {
		zipPath = path
	}

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
