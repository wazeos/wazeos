package drivers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	buildFile   string
	buildOutput string
)

var buildCmd = &cobra.Command{
	Use:   "build [directory]",
	Short: "Build a WASM driver",
	Long: `Build a WazeOS WASM driver.

This command compiles the driver to WASM using TinyGo.

Examples:
  # Build driver in current directory
  wazeos drivers build .

  # Build driver in specific directory
  wazeos drivers build drivers/file`,
	Args: cobra.MaximumNArgs(1),
	Run:  runDriverBuild,
}

func init() {
	buildCmd.Flags().StringVarP(&buildFile, "file", "f", "main.go", "Go source file to compile")
	buildCmd.Flags().StringVarP(&buildOutput, "output", "o", "app.wasm", "Output WASM file name")
}

func runDriverBuild(cmd *cobra.Command, args []string) {
	// Determine directory
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	// Make absolute path
	absDir, err := filepath.Abs(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Building driver in %s\n\n", absDir)

	// Find and validate files
	mainFile := filepath.Join(absDir, buildFile)
	metadataFile := filepath.Join(absDir, "metadata.json")

	if _, err := os.Stat(mainFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: %s not found in %s\n", buildFile, absDir)
		os.Exit(1)
	}

	if _, err := os.Stat(metadataFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: metadata.json not found in %s\n", absDir)
		fmt.Fprintf(os.Stderr, "Run 'wazeos drivers new' to create a new driver project\n")
		os.Exit(1)
	}

	// Check TinyGo
	fmt.Println("→ Checking TinyGo installation...")
	if err := checkDriverTinyGo(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "Install TinyGo: https://tinygo.org/getting-started/install/\n")
		os.Exit(1)
	}
	fmt.Println("  ✓ TinyGo found")

	// Build WASM
	fmt.Println("\n→ Compiling to WASM...")
	outputPath := filepath.Join(absDir, buildOutput)
	if err := buildDriverWASM(mainFile, outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error building WASM: %v\n", err)
		os.Exit(1)
	}

	// Get file size
	info, _ := os.Stat(outputPath)
	sizeKB := float64(info.Size()) / 1024.0

	fmt.Printf("  ✓ Built %s (%.1f KB)\n", buildOutput, sizeKB)
	fmt.Printf("\n✓ Build complete!\n\n")
	fmt.Println("Next steps:")
	fmt.Printf("  wazeos drivers package %s  # Create ZIP package\n", absDir)
}

// checkDriverTinyGo verifies TinyGo is installed
func checkDriverTinyGo() error {
	cmd := exec.Command("tinygo", "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("TinyGo not found")
	}
	return nil
}

// buildDriverWASM compiles Go code to WASM using TinyGo
func buildDriverWASM(inputFile, outputFile string) error {
	cmd := exec.Command("tinygo", "build",
		"-o", outputFile,
		"-target=wasi",
		inputFile)

	// Set working directory to the driver directory so go.mod is found
	cmd.Dir = filepath.Dir(inputFile)

	// Capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("compilation failed:\n%s", string(output))
	}

	return nil
}
