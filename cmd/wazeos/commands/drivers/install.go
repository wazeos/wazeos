package drivers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/wazeos/wazeos/internal/cli/client"
	"github.com/wazeos/wazeos/internal/cli/output"
)

var installCmd = &cobra.Command{
	Use:   "install <driver.zip|directory|url>",
	Short: "Install a driver from ZIP, URL, or build from directory",
	Long: `Install a driver package from a ZIP file, remote URL, or directory.

If a ZIP file is provided, it will be installed directly.
If a URL is provided, the package will be downloaded and installed.
If a directory is provided, the driver will be built, packaged, and installed.

The ZIP file must contain:
  - metadata.json (with type: "driver")
  - app.wasm (the driver binary)

Examples:
  # Install from driver repository (shorthand)
  wazeos drivers install wazeos/file
  wazeos drivers install wazeos/file:1.0.0

  # Install from local ZIP
  wazeos drivers install io.resource.file.zip

  # Install from remote URL
  wazeos drivers install https://github.com/org/repo/releases/download/v1.0.0/file.zip

  # Build and install from directory
  wazeos drivers install drivers/file

  # Install with custom data path
  wazeos drivers install io.resource.file.zip --data-path /var/lib/wazeos`,
	Args: cobra.ExactArgs(1),
	Run:  runDriverInstall,
}

func runDriverInstall(cmd *cobra.Command, args []string) {
	path := args[0]

	var zipPath string
	var cleanupTemp bool

	// Check if path matches author/package:version pattern
	if packageURL, ok := parseDriverPackageShorthand(path); ok {
		fmt.Printf("→ Resolving driver %s...\n", path)
		fmt.Printf("→ Downloading from %s...\n", packageURL)
		tempFile, err := downloadDriverPackage(packageURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error downloading driver: %v\n", err)
			os.Exit(1)
		}
		zipPath = tempFile
		cleanupTemp = true
		defer func() {
			if cleanupTemp {
				os.Remove(tempFile)
			}
		}()
		fmt.Println("  ✓ Driver downloaded")
	} else if isDriverURL(path) {
		fmt.Printf("→ Downloading driver from %s...\n", path)
		tempFile, err := downloadDriverPackage(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error downloading driver: %v\n", err)
			os.Exit(1)
		}
		zipPath = tempFile
		cleanupTemp = true
		defer func() {
			if cleanupTemp {
				os.Remove(tempFile)
			}
		}()
		fmt.Println("  ✓ Driver downloaded")
	} else {
		// Check if path is a directory or file
		info, err := os.Stat(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if info.IsDir() {
			// Build and package the driver first
			fmt.Println("Building and packaging driver...")
			runDriverPackage(cmd, []string{path})

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

			driverName, ok := metadata["name"].(string)
			if !ok || driverName == "" {
				fmt.Fprintf(os.Stderr, "Error: metadata.json missing 'name' field\n")
				os.Exit(1)
			}

			zipPath = filepath.Join(absDir, driverName+".zip")
			fmt.Printf("\n→ Installing from %s...\n", zipPath)
		} else {
			zipPath = path
		}
	}

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

// parseDriverPackageShorthand parses author/package:version format and returns the GitHub URL
// Examples:
//   - "wazeos/file" -> resolves to highest version in github.com/wazeos/packages/drivers/wazeos/file/
//   - "wazeos/file:1.0.0" -> "https://github.com/wazeos/packages/raw/main/drivers/wazeos/file/1.0.0.zip"
func parseDriverPackageShorthand(s string) (string, bool) {
	// Check if it looks like author/package format (contains / but not :// for URLs)
	if !strings.Contains(s, "/") || strings.Contains(s, "://") {
		return "", false
	}

	// Check if it's a local path (starts with ./ or ../ or /)
	if strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") || strings.HasPrefix(s, "/") {
		return "", false
	}

	// Parse author/package:version
	var author, pkg, version string

	// Split by : to get version
	parts := strings.SplitN(s, ":", 2)
	authorAndPkg := parts[0]
	if len(parts) == 2 {
		version = parts[1]
	}

	// Split author/package
	pkgParts := strings.SplitN(authorAndPkg, "/", 2)
	if len(pkgParts) != 2 {
		return "", false
	}
	author = pkgParts[0]
	pkg = pkgParts[1]

	// If no version specified, find the highest version
	if version == "" {
		fmt.Printf("→ Finding latest version for %s/%s...\n", author, pkg)
		versions, err := listDriverVersions(author, pkg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not list versions: %v\n", err)
			fmt.Fprintf(os.Stderr, "Trying 'latest' as fallback...\n")
			version = "latest"
		} else if len(versions) == 0 {
			fmt.Fprintf(os.Stderr, "Warning: No versions found, trying 'latest'...\n")
			version = "latest"
		} else {
			version = findHighestDriverVersion(versions)
			fmt.Printf("  ✓ Using version %s\n", version)
		}
	}

	// Construct GitHub URL with new structure
	url := fmt.Sprintf("https://github.com/wazeos/packages/raw/main/drivers/%s/%s/%s.zip", author, pkg, version)
	return url, true
}

// listDriverVersions lists available versions for a driver from GitHub
func listDriverVersions(author, pkg string) ([]string, error) {
	// Use GitHub API to list directory contents
	apiURL := fmt.Sprintf("https://api.github.com/repos/wazeos/packages/contents/drivers/%s/%s", author, pkg)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch versions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var items []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract .zip files and get version names
	var versions []string
	for _, item := range items {
		if item.Type == "file" && strings.HasSuffix(item.Name, ".zip") {
			// Remove .zip extension to get version
			version := strings.TrimSuffix(item.Name, ".zip")
			versions = append(versions, version)
		}
	}

	return versions, nil
}

// findHighestDriverVersion finds the highest semantic version from a list
func findHighestDriverVersion(versions []string) string {
	if len(versions) == 0 {
		return "latest"
	}

	highest := versions[0]
	highestSemVer := parseDriverSemVer(highest)

	for _, v := range versions[1:] {
		semVer := parseDriverSemVer(v)
		if compareDriverSemVer(semVer, highestSemVer) > 0 {
			highest = v
			highestSemVer = semVer
		}
	}

	return highest
}

// driverSemVer represents a semantic version
type driverSemVer struct {
	major      int
	minor      int
	patch      int
	prerelease string
	original   string
}

// parseDriverSemVer parses a semantic version string
func parseDriverSemVer(v string) driverSemVer {
	sv := driverSemVer{original: v}

	// Handle special cases
	if v == "latest" {
		sv.major = 999999
		return sv
	}

	// Remove 'v' prefix if present
	v = strings.TrimPrefix(v, "v")

	// Match semantic version pattern: major.minor.patch[-prerelease]
	re := regexp.MustCompile(`^(\d+)(?:\.(\d+))?(?:\.(\d+))?(?:-(.+))?$`)
	matches := re.FindStringSubmatch(v)

	if matches == nil {
		// Not a valid semver, treat as string comparison
		return sv
	}

	sv.major, _ = strconv.Atoi(matches[1])
	if matches[2] != "" {
		sv.minor, _ = strconv.Atoi(matches[2])
	}
	if matches[3] != "" {
		sv.patch, _ = strconv.Atoi(matches[3])
	}
	if matches[4] != "" {
		sv.prerelease = matches[4]
	}

	return sv
}

// compareDriverSemVer compares two semantic versions
// Returns: -1 if a < b, 0 if a == b, 1 if a > b
func compareDriverSemVer(a, b driverSemVer) int {
	if a.major != b.major {
		if a.major > b.major {
			return 1
		}
		return -1
	}

	if a.minor != b.minor {
		if a.minor > b.minor {
			return 1
		}
		return -1
	}

	if a.patch != b.patch {
		if a.patch > b.patch {
			return 1
		}
		return -1
	}

	// Handle prerelease versions (versions without prerelease are > than with prerelease)
	if a.prerelease == "" && b.prerelease != "" {
		return 1
	}
	if a.prerelease != "" && b.prerelease == "" {
		return -1
	}

	// Compare prerelease strings lexicographically
	if a.prerelease != b.prerelease {
		if a.prerelease > b.prerelease {
			return 1
		}
		return -1
	}

	return 0
}

// isDriverURL checks if a string is an HTTP or HTTPS URL
func isDriverURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// downloadDriverPackage downloads a driver package from a URL to a temporary file
func downloadDriverPackage(url string) (string, error) {
	// Create temporary file
	tempFile, err := os.CreateTemp("", "wazeos-driver-*.zip")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	// Download the file
	resp, err := http.Get(url)
	if err != nil {
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("download failed with status: %d %s", resp.StatusCode, resp.Status)
	}

	// Copy response body to temp file
	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	return tempFile.Name(), nil
}
