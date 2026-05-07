package apps

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
	newAppDescription string
)

var newCmd = &cobra.Command{
	Use:   "new [author] [app-name]",
	Short: "Create a new WASM app project",
	Long: `Create a new WazeOS WASM application project with a skeleton structure.

Creates a directory structure at ./{author}/{app-name}/ with:
  - main.go: Basic WASM app template
  - metadata.json: App metadata
  - go.mod: Go module file
  - Makefile: Build commands
  - README.md: Documentation
  - .gitignore: Ignore build artifacts

Examples:
  # Interactive mode (prompts for author and name)
  wazeos apps new

  # With arguments
  wazeos apps new mycompany myapp

  # With description
  wazeos apps new mycompany myapp --description "My awesome app"`,
	Args: cobra.MaximumNArgs(2),
	Run:  runNew,
}

func init() {
	newCmd.Flags().StringVar(&newAppDescription, "description", "", "package description")
}

func runNew(cmd *cobra.Command, args []string) {
	var author, appName string

	// Get author from args or prompt
	if len(args) >= 1 {
		author = args[0]
	} else {
		author = promptRequired("Author name")
	}

	// Get app name from args or prompt
	if len(args) >= 2 {
		appName = args[1]
	} else {
		appName = promptRequired("App name")
	}

	// Validate names
	if err := validateName(author); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid author name: %v\n", err)
		os.Exit(1)
	}
	if err := validateName(appName); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid app name: %v\n", err)
		os.Exit(1)
	}

	// Get description if not provided
	description := newAppDescription
	if description == "" {
		description = fmt.Sprintf("WASM application: %s", appName)
	}

	// Create project directory
	projectDir := filepath.Join(".", author, appName)
	if _, err := os.Stat(projectDir); !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: directory already exists: %s\n", projectDir)
		os.Exit(1)
	}

	if err := os.MkdirAll(projectDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating directory: %v\n", err)
		os.Exit(1)
	}

	// Generate project files
	if err := generateProjectFiles(projectDir, author, appName, description); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating project: %v\n", err)
		os.Exit(1)
	}

	// Success message
	fmt.Printf("✓ Created new app project: %s\n\n", projectDir)
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
	// Names must be lowercase alphanumeric with hyphens/underscores
	// Must start with a letter
	matched, _ := regexp.MatchString(`^[a-z][a-z0-9\-_]*$`, name)
	if !matched {
		return fmt.Errorf("must start with a letter and contain only lowercase letters, numbers, hyphens, and underscores")
	}
	return nil
}

func generateProjectFiles(projectDir, author, appName, description string) error {
	files := map[string]string{
		"main.go":       generateMainGo(),
		"metadata.json": generateMetadata(author, appName, description),
		"go.mod":        generateGoMod(author, appName),
		"Makefile":      generateMakefile(author, appName),
		"README.md":     generateReadme(author, appName, description),
		".gitignore":    generateGitignore(),
	}

	for filename, content := range files {
		path := filepath.Join(projectDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	return nil
}

func generateMainGo() string {
	return `package main

import (
	"fmt"

	"github.com/wazeos/wazeos/sdk/app"
)

// Tool implements the MCPToolHandler interface.
type Tool struct{}

// Handle processes the MCP tool invocation.
// input: JSON object with parameters (matches inputSchema in metadata.json)
// returns: JSON object to send back to the client
func (t *Tool) Handle(ctx *app.Context, input map[string]interface{}) (map[string]interface{}, error) {
	// Log execution
	ctx.Log.Info("tool invoked", app.Int("argCount", len(input)))

	// Get input parameters (type-safe)
	message, _ := input["message"].(string)

	// Process the input
	greeting := "Hello from WazeOS!"
	if message != "" {
		greeting = fmt.Sprintf("Hello, %s!", message)
	}

	// Example: Use context for authentication, tracing, etc.
	// requestID := ctx.RequestID
	// principal := ctx.Principal
	// if !ctx.HasPermission("file:///tmp/*", "r") {
	//     return nil, app.Error(403, "Permission denied")
	// }

	// Example: File operations (requires io.resource.file driver)
	// data, err := ctx.IO.ReadFile("/tmp/example.txt")
	// if err != nil {
	//     return nil, err
	// }

	// Example: HTTP requests (requires io.resource.http driver)
	// resp, err := ctx.IO.Get("https://api.example.com/data")
	// if err != nil {
	//     return nil, err
	// }

	// Example: Call another app
	// result, err := ctx.IO.CallApp("other-app", map[string]interface{}{"key": "value"})
	// if err != nil {
	//     return nil, err
	// }

	// Return response
	return map[string]interface{}{
		"status":  "success",
		"message": greeting,
	}, nil
}

func main() {
	app.RunMCPTool(&Tool{})
}
`
}

func generateMetadata(author, appName, description string) string {
	return fmt.Sprintf(`{
  "name": "%s",
  "version": "1.0.0",
  "author": "%s",
  "description": "%s",
  "type": "app",
  "entrypoint": "_start",
  "inputSchema": {
    "type": "object",
    "properties": {
      "message": {
        "type": "string",
        "description": "Message to echo back"
      }
    }
  }
}
`, appName, author, description)
}

func generateGoMod(author, appName string) string {
	return fmt.Sprintf(`module github.com/%s/%s

go 1.21

require github.com/wazeos/wazeos v0.1.0
`, author, appName)
}

func generateMakefile(author, appName string) string {
	return fmt.Sprintf(`.PHONY: build clean package install test

# Build the WASM binary
build:
	@echo "Building %s WASM app..."
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

# Install the package using wazeos CLI
install: package
	@echo "Installing %s..."
	../../wazeos apps install %s.zip
	@echo "Installation complete"

# Test basic compilation
test:
	@echo "Testing WASM compilation..."
	@tinygo build -o /dev/null -target=wasi main.go && echo "✓ Compilation test passed"

# Show package info
info:
	@echo "=== %s/%s Info ==="
	@echo "Name:         %s"
	@echo "Version:      1.0.0"
	@echo "Type:         app"
	@echo "Author:       %s"
	@echo ""
	@echo "Commands:"
	@echo "  make build   - Compile to WASM"
	@echo "  make package - Create ZIP package"
	@echo "  make install - Install via wazeos CLI"
	@echo "  make test    - Test compilation"
`, appName, appName, appName, appName, appName, appName, appName, author, appName, appName, author)
}

func generateReadme(author, appName, description string) string {
	return fmt.Sprintf(`# %s

%s

A WazeOS WASM application that can be called as an MCP tool.

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

This app uses the WazeOS SDK which handles all JSON parsing, error handling, and context management automatically.

### Implementing the Handler

Edit the `+"`Handle`"+` method in `+"`main.go`"+` to process MCP tool calls:

`+"```go"+`
func (t *Tool) Handle(ctx *app.Context, input map[string]interface{}) (map[string]interface{}, error) {
    // Get input parameters (type-safe)
    message, _ := input["message"].(string)

    // Use context for logging, auth, I/O
    ctx.Log.Info("processing request")

    // Return JSON response
    return map[string]interface{}{
        "status": "success",
        "result": "processed",
    }, nil
}
`+"```"+`

The SDK's `+"`app.RunMCPTool()`"+` function handles:
- Reading and parsing JSON input from stdin
- Building execution context from environment variables
- Calling your Handle method with parsed input
- Writing the JSON response to stdout
- Error handling and exit codes

### Using the Context

The `+"`Context`"+` provides access to execution metadata and I/O:

`+"```go"+`
// Logging with structured fields
ctx.Log.Info("operation complete", app.Int("count", 5))
ctx.Log.Error("failed", app.ErrorField(err))

// Check permissions
if !ctx.HasPermission("file:///tmp/*", "r") {
    return nil, app.Error(403, "Permission denied")
}

// File operations (requires io.resource.file driver)
data, err := ctx.IO.ReadFile("/tmp/example.txt")

// HTTP requests (requires io.resource.http driver)
resp, err := ctx.IO.Get("https://api.example.com/data")

// Call another app
result, err := ctx.IO.CallApp("other-app", map[string]interface{}{"key": "value"})
`+"```"+`

### MCP Tool Integration

When installed, this app is automatically exposed as an MCP tool that Claude can call.
The `+"`inputSchema`"+` in `+"`metadata.json`"+` defines the tool's parameters.

## Testing

`+"```bash"+`
# Test with sample input
echo '{"message": "World"}' | ./app.wasm

# Expected output:
# {"status":"success","message":"Hello, World!"}
`+"```"+`

## Structure

- `+"`main.go`"+` - Application implementation with Handle method
- `+"`metadata.json`"+` - Package metadata with inputSchema for MCP
- `+"`go.mod`"+` - Go module definition
- `+"`Makefile`"+` - Build automation

## Customization

1. Update the `+"`inputSchema`"+` in `+"`metadata.json`"+` to define your tool's parameters
2. Implement your business logic in the `+"`Handle`"+` method
3. Use `+"`ctx.IO`"+` for file, HTTP, and inter-app operations
4. Use `+"`ctx.Log`"+` for structured logging

## Author

%s
`, appName, description, author)
}

func generateGitignore() string {
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
