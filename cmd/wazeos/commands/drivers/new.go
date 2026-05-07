package drivers

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var (
	newDriverClass       string
	newDriverDescription string
	newDriverPatterns    []string
)

var newCmd = &cobra.Command{
	Use:   "new [author] [driver-name]",
	Short: "Create a new WASM driver project",
	Long: `Create a new WazeOS WASM driver project with a skeleton structure.

Creates a directory structure at ./{author}/{driver-name}/ with:
  - main.go: Driver template with HandleCall
  - metadata.json: Driver metadata
  - go.mod: Go module file
  - Makefile: Build commands
  - README.md: Documentation
  - .gitignore: Ignore build artifacts

Examples:
  # Interactive mode (prompts for author, name, and driver class)
  wazeos drivers new

  # With arguments
  wazeos drivers new mycompany mydriver --driver-class=io.resource

  # With URI patterns
  wazeos drivers new mycompany mydriver --driver-class=io.resource --patterns=custom://**`,
	Args: cobra.MaximumNArgs(2),
	Run:  runNewDriver,
}

func init() {
	newCmd.Flags().StringVar(&newDriverClass, "driver-class", "", "driver class (e.g., io.resource, kernel.security.authn)")
	newCmd.Flags().StringVar(&newDriverDescription, "description", "", "driver description")
	newCmd.Flags().StringSliceVar(&newDriverPatterns, "patterns", nil, "URI patterns (e.g., custom://**)")
}

func runNewDriver(cmd *cobra.Command, args []string) {
	var author, driverName string

	// Get author from args or prompt
	if len(args) >= 1 {
		author = args[0]
	} else {
		author = promptRequired("Author name")
	}

	// Get driver name from args or prompt
	if len(args) >= 2 {
		driverName = args[1]
	} else {
		driverName = promptRequired("Driver name")
	}

	// Validate names
	if err := validateName(author); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid author name: %v\n", err)
		os.Exit(1)
	}
	if err := validateName(driverName); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid driver name: %v\n", err)
		os.Exit(1)
	}

	// Get driver class if not provided
	if newDriverClass == "" {
		fmt.Println("\nStandard driver classes:")
		fmt.Println("  io.resource             - Resource driver (files, HTTP, databases, caches, etc.)")
		fmt.Println("                            URI patterns determine the specific resource type")
		fmt.Println("  io.function             - Function execution driver")
		fmt.Println("  kernel.security.authn   - Authentication provider")
		fmt.Println("  security.authz   - Authorization/permissions provider")
		fmt.Println("  kernel.security.audit   - Audit logging driver")
		fmt.Println("  kernel.ipc              - Inter-process communication / message queues")
		fmt.Println()
		newDriverClass = promptRequired("Driver class")
	}

	// Get description if not provided
	description := newDriverDescription
	if description == "" {
		description = fmt.Sprintf("WASM driver: %s", driverName)
	}

	// Get URI patterns if not provided
	patterns := newDriverPatterns
	if len(patterns) == 0 {
		// Default pattern based on driver class
		switch newDriverClass {
		case "io.resource":
			patterns = []string{"custom://**"}
		case "io.function":
			patterns = []string{"fn://**"}
		case "kernel.ipc":
			patterns = []string{"queue://**"}
		default:
			// For other classes (security, audit, etc.), use a generic pattern
			patterns = []string{fmt.Sprintf("%s://**", driverName)}
		}
	}

	// Create project directory
	projectDir := filepath.Join(".", author, driverName)
	if _, err := os.Stat(projectDir); !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: directory already exists: %s\n", projectDir)
		os.Exit(1)
	}

	if err := os.MkdirAll(projectDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating directory: %v\n", err)
		os.Exit(1)
	}

	// Generate project files
	if err := generateDriverProjectFiles(projectDir, author, driverName, description, newDriverClass, patterns); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating project: %v\n", err)
		os.Exit(1)
	}

	// Success message
	fmt.Printf("✓ Created new driver project: %s\n\n", projectDir)
	fmt.Println("Driver details:")
	fmt.Printf("  Class:        %s\n", newDriverClass)
	fmt.Printf("  Patterns:     %v\n\n", patterns)
	fmt.Println("Next steps:")
	fmt.Printf("  cd %s\n", projectDir)
	fmt.Println("  make build    # Build the WASM binary")
	fmt.Println("  make package  # Create installable ZIP")
	fmt.Println("  make install  # Install locally")
}

func promptRequired(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s: ", prompt)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			return input
		}
		fmt.Println("This field is required.")
	}
}

func validateName(name string) error {
	// Names must be lowercase alphanumeric with hyphens/underscores/dots
	// Must start with a letter
	matched, _ := regexp.MatchString(`^[a-z][a-z0-9\-_.]*$`, name)
	if !matched {
		return fmt.Errorf("must start with a letter and contain only lowercase letters, numbers, hyphens, underscores, and dots")
	}
	return nil
}

func generateDriverProjectFiles(projectDir, author, driverName, description, driverClass string, patterns []string) error {
	files := map[string]string{
		"main.go":       generateDriverMainGo(),
		"metadata.json": generateDriverMetadata(author, driverName, description, driverClass, patterns),
		"go.mod":        generateDriverGoMod(author, driverName),
		"Makefile":      generateDriverMakefile(author, driverName, driverClass),
		"README.md":     generateDriverReadme(author, driverName, description, driverClass, patterns),
		".gitignore":    generateDriverGitignore(),
	}

	for filename, content := range files {
		path := filepath.Join(projectDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	return nil
}

func generateDriverMainGo() string {
	return `package main

import (
	"fmt"

	"github.com/wazeos/wazeos/sdk/driver"
)

// Input defines the configuration/operation parameters for this driver.
// The build command will automatically generate JSON Schema from these struct tags.
//
// Supported struct tags:
//   json:"name,omitempty"         - Field name and optionality
//   description:"text"            - Field description
//   default:"value"               - Default value
//   enum:"a,b,c"                  - Allowed values (comma-separated)
//   min:"1" / max:"100"           - Numeric constraints (for int/float)
//   minLength:"1" / maxLength:"255" - String length constraints
//   pattern:"^[a-z]+$"            - Regex pattern (for strings)
//   example:"sample"              - Example value
//
// Example Input struct (uncomment to use):
// type Input struct {
//     Operation string ` + "`" + `json:"operation" description:"Operation to perform" enum:"read,write,delete"` + "`" + `
//     Path      string ` + "`" + `json:"path" description:"Resource path" minLength:"1"` + "`" + `
//     Timeout   int    ` + "`" + `json:"timeout,omitempty" description:"Timeout in seconds" default:"30" min:"1" max:"300"` + "`" + `
// }

// Driver implements the ResourceHandler interface.
type Driver struct{}

// HandleCall processes resource operations.
// The operation is determined by the permissions granted to the call.
func (d *Driver) HandleCall(call *driver.ResourceCall) (*driver.ResourceResult, error) {
	// Check permissions to determine which operation to perform
	hasRead := false
	hasWrite := false
	hasDelete := false
	hasList := false

	for _, perm := range call.Permissions {
		switch perm {
		case "read", "GET":
			hasRead = true
		case "write", "PUT", "POST":
			hasWrite = true
		case "delete", "DELETE":
			hasDelete = true
		case "list", "LIST":
			hasList = true
		}
	}

	// Handle operations based on permissions (priority: write > delete > list > read)
	if hasWrite {
		// TODO: Implement write logic
		// Example: Write call.Body to call.URI
		return driver.NewResourceResult(200, []byte(` + "`" + `{"message":"Write operation"}` + "`" + `)), nil
	}

	if hasDelete {
		// TODO: Implement delete logic
		return driver.NewResourceResult(200, []byte(` + "`" + `{"message":"Delete operation"}` + "`" + `)), nil
	}

	if hasList {
		// TODO: Implement list logic
		return driver.NewResourceResult(200, []byte(` + "`" + `{"items":[]}` + "`" + `)), nil
	}

	if hasRead {
		// TODO: Implement read logic
		// Example: Read data from call.URI
		return driver.NewResourceResult(200, []byte(` + "`" + `{"message":"Read operation"}` + "`" + `)), nil
	}

	return driver.NewErrorResult(403, "no valid operation permission provided"), nil
}

func main() {
	driver.ServeResourceOnce(&Driver{})
}
`
}

func generateDriverMetadata(author, driverName, description, driverClass string, patterns []string) string {
	// Format patterns as JSON array
	patternsJSON := "["
	for i, p := range patterns {
		if i > 0 {
			patternsJSON += ", "
		}
		patternsJSON += fmt.Sprintf("\"%s\"", p)
	}
	patternsJSON += "]"

	return fmt.Sprintf(`{
  "name": "%s",
  "version": "1.0.0",
  "author": "%s",
  "description": "%s",
  "type": "driver",
  "driverClass": "%s",
  "uriPatterns": %s,
  "entrypoint": "_start",
  "prerequisitesV2": {
    "apps": {},
    "drivers": {}
  },
  "dependenciesV2": {
    "apps": {},
    "drivers": {}
  },
  "privileges": {
    "wazero": {},
    "hostFunctions": []
  },
  "permissions": [
    {"name": "read", "description": "Read access", "bit": 1},
    {"name": "write", "description": "Write access", "bit": 2}
  ]
}
`, driverName, author, description, driverClass, patternsJSON)
}

func generateDriverGoMod(author, driverName string) string {
	return fmt.Sprintf(`module github.com/%s/%s

go 1.21

require github.com/wazeos/wazeos v0.1.0
`, author, driverName)
}

func generateDriverMakefile(author, driverName, driverClass string) string {
	return fmt.Sprintf(`.PHONY: build clean package install test

# Build the WASM binary
build:
	@echo "Building %s WASM driver..."
	tinygo build -o app.wasm -target=wasi main.go
	@echo "Build complete: app.wasm"

# Create the package
package: build
	@echo "Creating package..."
	@rm -f %s.zip
	zip %s.zip metadata.json app.wasm
	@echo "Package created: %s.zip"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f app.wasm %s.zip
	@echo "Clean complete"

# Install the driver using wazeos CLI
install: package
	@echo "Installing %s..."
	../../wazeos drivers install %s.zip
	@echo "Installation complete"

# Test basic compilation
test:
	@echo "Testing WASM compilation..."
	@tinygo build -o /dev/null -target=wasi main.go && echo "✓ Compilation test passed"

# Show driver info
info:
	@echo "=== %s/%s Info ==="
	@echo "Name:         %s"
	@echo "Version:      1.0.0"
	@echo "Type:         driver"
	@echo "Class:        %s"
	@echo "Author:       %s"
	@echo ""
	@echo "Commands:"
	@echo "  make build   - Compile to WASM"
	@echo "  make package - Create ZIP package"
	@echo "  make install - Install via wazeos CLI"
	@echo "  make test    - Test compilation"
`, driverName, driverClass, driverClass, driverClass, driverClass, driverName, driverClass, author, driverName, driverName, driverClass, author)
}

func generateDriverReadme(author, driverName, description, driverClass string, patterns []string) string {
	patternsStr := ""
	for _, p := range patterns {
		patternsStr += fmt.Sprintf("- `%s`\n", p)
	}

	return fmt.Sprintf(`# %s

%s

**Driver Class:** %s

**URI Patterns:**
%s

## Quick Start

`+"```bash"+`
make build    # Build WASM binary
make package  # Create ZIP package
make install  # Install locally
`+"```"+`

## Building

Requires [TinyGo](https://tinygo.org/) for WASM compilation:

`+"```bash"+`
tinygo build -o app.wasm -target=wasi main.go
`+"```"+`

## Development

The driver uses the WazeOS SDK which handles all JSON parsing, error handling, and response formatting automatically.

### Implementing HandleCall

Edit the `+"`HandleCall`"+` method in `+"`main.go`"+` to process resource operations:

`+"```go"+`
func (d *Driver) HandleCall(call *driver.ResourceCall) (*driver.ResourceResult, error) {
    // Access the resource URI and permissions
    uri := call.URI
    permissions := call.Permissions

    // Check which operations are allowed
    hasRead := contains(permissions, "read")
    hasWrite := contains(permissions, "write")

    // For write operations, access the body
    body := call.Body

    // Return a success result
    return driver.NewResourceResult(200, responseData), nil

    // Or return an error result
    return driver.NewErrorResult(404, "not found"), nil
}
`+"```"+`

The SDK's `+"`driver.ServeResourceOnce()`"+` function handles:
- Reading and parsing JSON input from stdin
- Calling your HandleCall method
- Writing the JSON response to stdout
- Error handling and exit codes

### Testing

Test with sample input:

`+"```bash"+`
echo '{"method":"GET","uri":"test://example"}' | ./app.wasm
`+"```"+`

## Structure

- `+"`main.go`"+` - Driver implementation with HandleCall method
- `+"`metadata.json`"+` - Driver metadata, privileges, and permissions
- `+"`go.mod`"+` - Go module definition
- `+"`Makefile`"+` - Build automation

### Metadata Structure

The `+"`metadata.json`"+` file contains several key sections:

**Prerequisites & Dependencies** - Required packages (apps or drivers):
- `+"`prerequisitesV2`"+` - Auto-installed before this package
- `+"`dependenciesV2`"+` - Must be manually installed first
- Both are structured with separate `+"`apps`"+` and `+"`drivers`"+` sections
- Format: `+"`\"author/package\": \"version\"`"+`

Example:
`+"```json"+`
"prerequisitesV2": {
  "apps": {
    "wazeos/logger": "1.0.0"
  },
  "drivers": {
    "wazeos/file": "2.0.0"
  }
}
`+"```"+`

**Privileges** - System capabilities the driver requests FROM wazero:
- `+"`wazero`"+` - Wazero-specific capabilities (network, filesystem, etc.)
- `+"`hostFunctions`"+` - Allowed host function namespaces

**Permissions** - Access control permissions the driver EXPOSES to apps:
- Defines the operations apps can request (e.g., GET, POST, read, write)
- Each permission has a name, description, and bit flag for efficient checking
- Apps request these permissions in their access control policies

## Author

%s
`, driverName, description, driverClass, patternsStr, author)
}

func generateDriverGitignore() string {
	return `# Build artifacts
app.wasm
*.zip

# TinyGo cache
.tinygo-cache/

# OS files
.DS_Store
Thumbs.db
`
}
