package apps

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var packageCmd = &cobra.Command{
	Use:   "package [directory]",
	Short: "Build and package a WASM app into a ZIP file",
	Long: `Build and package a WazeOS WASM application into an installable ZIP file.

This command:
1. Runs 'wazeos apps build' to compile the app
2. Creates a ZIP file containing metadata.json and app.wasm
3. Names the ZIP file as {appname}.zip

Examples:
  # Package app in current directory
  wazeos apps package .

  # Package app in specific directory
  wazeos apps package bin/mycompany/myapp`,
	Args: cobra.MaximumNArgs(1),
	Run:  runPackage,
}

func runPackage(cmd *cobra.Command, args []string) {
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

	fmt.Printf("Packaging app in %s\n\n", absDir)

	// Step 1: Build the app first
	fmt.Println("→ Building app...")
	runBuild(cmd, []string{absDir})

	// Step 2: Read metadata to get app name
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

	// Step 3: Create ZIP file
	zipFile := filepath.Join(absDir, appName+".zip")
	fmt.Printf("\n→ Creating package %s...\n", appName+".zip")

	if err := createZipPackage(absDir, zipFile); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating package: %v\n", err)
		os.Exit(1)
	}

	// Get file size
	info, _ := os.Stat(zipFile)
	sizeKB := float64(info.Size()) / 1024.0

	fmt.Printf("  ✓ Created %s (%.1f KB)\n", appName+".zip", sizeKB)
	fmt.Printf("\n✓ Package complete!\n\n")
	fmt.Println("Next steps:")
	fmt.Printf("  wazeos apps install %s  # Install the package\n", zipFile)
}

func createZipPackage(dir, zipPath string) error {
	// Remove existing zip
	os.Remove(zipPath)

	// Create zip file
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("failed to create zip: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Files to include in package
	files := []string{"metadata.json", "app.wasm"}

	for _, filename := range files {
		srcPath := filepath.Join(dir, filename)

		// Check if file exists
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			return fmt.Errorf("required file not found: %s", filename)
		}

		// Add to zip
		if err := addFileToZip(zipWriter, srcPath, filename); err != nil {
			return fmt.Errorf("failed to add %s to zip: %w", filename, err)
		}
	}

	return nil
}

func addFileToZip(zipWriter *zip.Writer, srcPath, zipPath string) error {
	// Open source file
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Get file info
	info, err := srcFile.Stat()
	if err != nil {
		return err
	}

	// Create zip file header
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = zipPath
	header.Method = zip.Deflate

	// Create writer for file in zip
	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	// Copy file contents
	_, err = io.Copy(writer, srcFile)
	return err
}
