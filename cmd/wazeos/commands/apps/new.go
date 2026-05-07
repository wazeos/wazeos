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
	newAppLanguage    string
)

var newCmd = &cobra.Command{
	Use:   "new [author] [app-name]",
	Short: "Create a new WASM app project",
	Long: `Create a new WazeOS WASM application project with a skeleton structure.

Supports multiple languages:
  - Go (default): Uses TinyGo for WASM compilation
  - Rust: Uses Cargo with wasm32-wasi target

Creates a directory structure at ./{author}/{app-name}/ with language-specific templates.

Examples:
  # Interactive mode (prompts for author and name)
  wazeos apps new

  # Create Go app (default)
  wazeos apps new mycompany myapp

  # Create Rust app
  wazeos apps new mycompany myapp --language rust

  # With description
  wazeos apps new mycompany myapp --description "My awesome app"`,
	Args: cobra.MaximumNArgs(2),
	Run:  runNew,
}

func init() {
	newCmd.Flags().StringVar(&newAppDescription, "description", "", "package description")
	newCmd.Flags().StringVar(&newAppLanguage, "language", "go", "programming language (go or rust)")
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

	// Validate and normalize language
	language := strings.ToLower(newAppLanguage)
	if language != "go" && language != "rust" {
		fmt.Fprintf(os.Stderr, "Error: unsupported language '%s'. Supported: go, rust\n", language)
		os.Exit(1)
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
	if err := generateProjectFiles(projectDir, author, appName, description, language); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating project: %v\n", err)
		os.Exit(1)
	}

	// Success message
	fmt.Printf("✓ Created new %s app project: %s\n\n", strings.Title(language), projectDir)
	fmt.Println("Next steps:")
	fmt.Printf("  cd %s\n", projectDir)
	fmt.Println("  wazeos apps build .    # Build the WASM binary")
	fmt.Println("  wazeos apps package .  # Create installable ZIP")
	fmt.Println("  wazeos apps install .  # Build, package, and install")
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

func generateProjectFiles(projectDir, author, appName, description, language string) error {
	var files map[string]string

	if language == "rust" {
		// Create src directory for Rust projects
		srcDir := filepath.Join(projectDir, "src")
		if err := os.MkdirAll(srcDir, 0755); err != nil {
			return fmt.Errorf("failed to create src directory: %w", err)
		}

		files = map[string]string{
			"src/main.rs":   generateMainRust(),
			"Cargo.toml":    generateCargoToml(author, appName, description),
			"metadata.json": generateMetadata(author, appName, description),
			"README.md":     generateReadmeRust(author, appName, description),
			".gitignore":    generateGitignoreRust(),
		}
	} else {
		// Go project
		files = map[string]string{
			"main.go":       generateMainGo(),
			"metadata.json": generateMetadata(author, appName, description),
			"go.mod":        generateGoMod(author, appName),
			"README.md":     generateReadme(author, appName, description),
			".gitignore":    generateGitignore(),
		}
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
	"encoding/json"
	"fmt"

	"github.com/wazeos/wazeos/sdk/app"
)

// Input defines the tool's input parameters.
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
// Example fields:
//   Name  string ` + "`" + `json:"name" description:"User name" minLength:"1" maxLength:"50"` + "`" + `
//   Age   int    ` + "`" + `json:"age,omitempty" description:"User age" min:"0" max:"150" default:"0"` + "`" + `
//   Email string ` + "`" + `json:"email" description:"Email address" pattern:"^[^@]+@[^@]+\\.[^@]+$"` + "`" + `
//   Role  string ` + "`" + `json:"role,omitempty" description:"User role" enum:"admin,user,guest" default:"guest"` + "`" + `
type Input struct {
	Message string ` + "`json:\"message\" description:\"Message to echo back\" example:\"World\"`" + `
}

// Tool implements the MCPToolHandler interface.
type Tool struct{}

// Handle processes the MCP tool invocation with typed input.
func (t *Tool) Handle(ctx *app.Context, inputRaw map[string]interface{}) (map[string]interface{}, error) {
	// Parse input into typed struct
	var input Input
	data, _ := json.Marshal(inputRaw)
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, app.WrapError(err, "INVALID_INPUT", "Invalid input", 400)
	}

	// Log execution
	ctx.Log.Info("tool invoked", app.String("message", input.Message))

	// Process the input with type safety!
	greeting := "Hello from WazeOS!"
	if input.Message != "" {
		greeting = fmt.Sprintf("Hello, %s!", input.Message)
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
  "entrypoint": "_start"
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
# Build the app
wazeos apps build .

# Build and create package
wazeos apps package .

# Build, package, and install
wazeos apps install .
`+"```"+`

## Development

This app uses the WazeOS SDK with automatic schema generation from Go structs.

### Defining Input Parameters

Define your input parameters as a Go struct with JSON Schema tags:

`+"```go"+`
type Input struct {
    Message string `+"`json:\"message\" description:\"Message to echo back\" example:\"World\"`"+`
    Count   int    `+"`json:\"count,omitempty\" description:\"Repeat count\" default:\"1\" min:\"1\" max:\"10\"`"+`
}
`+"```"+`

**Available struct tags:**
- `+"`json`"+` - JSON field name and options (required)
- `+"`description`"+` - Field description for MCP
- `+"`example`"+` - Example value
- `+"`default`"+` - Default value
- `+"`min`"+`, `+"`max`"+` - Validation for numbers
- `+"`minLength`"+`, `+"`maxLength`"+` - Validation for strings
- `+"`pattern`"+` - Regex pattern for strings
- `+"`enum`"+` - Comma-separated allowed values

The JSON Schema is automatically generated during build!

### Implementing the Handler

Edit the `+"`Handle`"+` method in `+"`main.go`"+` to process MCP tool calls:

`+"```go"+`
func (t *Tool) Handle(ctx *app.Context, inputRaw map[string]interface{}) (map[string]interface{}, error) {
    // Parse input into typed struct
    var input Input
    data, _ := json.Marshal(inputRaw)
    json.Unmarshal(data, &input)

    // Use typed fields with auto-completion!
    greeting := fmt.Sprintf("Hello, %%s!", input.Message)

    // Return JSON response
    return map[string]interface{}{
        "status": "success",
        "message": greeting,
    }, nil
}
`+"```"+`

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
The `+"`inputSchema`"+` is generated from your Input struct during build.

## Build Process

When you run `+"`wazeos apps build`"+`, it:
1. Scans your Go code for the Input struct
2. Extracts JSON Schema from struct tags
3. Updates metadata.json automatically
4. Compiles to WASM using TinyGo

No manual schema maintenance needed!

## Testing

`+"```bash"+`
# Build first
wazeos apps build .

# Test with sample input
echo '{"message": "World"}' | ./app.wasm

# Expected output:
# {"status":"success","message":"Hello, World!"}
`+"```"+`

## Structure

- `+"`main.go`"+` - Application implementation with Input struct and Handle method
- `+"`metadata.json`"+` - Package metadata (inputSchema auto-generated)
- `+"`go.mod`"+` - Go module definition

## Requirements

- [TinyGo](https://tinygo.org/getting-started/install/) for WASM compilation
- WazeOS CLI

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

# IDE
.vscode/
.idea/

# OS files
.DS_Store
Thumbs.db
`
}

// ============================================================================
// Rust Template Generation Functions
// ============================================================================

func generateMainRust() string {
	return `use serde_json::{json, Value};
use wazeos_app::{run_mcp_tool, Context, MCPToolHandler};

/// Input structure for the tool.
///
/// For Rust apps, you need to manually define the input schema in metadata.json.
/// The schema should match this structure.
#[derive(serde::Deserialize, Debug)]
struct Input {
    /// Message to echo back
    #[serde(default)]
    message: String,
}

struct Tool;

impl MCPToolHandler for Tool {
    fn handle(&self, ctx: &Context, input: Value) -> Result<Value, Box<dyn std::error::Error>> {
        // Parse input - handle both empty input and typed input
        let input: Input = serde_json::from_value(input).unwrap_or(Input {
            message: String::new(),
        });

        // Log execution
        ctx.info(&format!("Tool invoked with message: {}", input.message));

        // Process the input
        let greeting = if input.message.is_empty() {
            "Hello from WazeOS!".to_string()
        } else {
            format!("Hello, {}!", input.message)
        };

        // Example: Check permissions
        // if !ctx.has_permission("file:///tmp/*", &["read"]) {
        //     return Err("Permission denied".into());
        // }

        // Return JSON response
        Ok(json!({
            "status": "success",
            "message": greeting,
        }))
    }
}

fn main() {
    run_mcp_tool(&Tool);
}
`
}

func generateCargoToml(author, appName, description string) string {
	return fmt.Sprintf(`[package]
name = "%s"
version = "1.0.0"
edition = "2021"
authors = ["%s"]
description = "%s"

[dependencies]
wazeos-app = { path = "../../../sdk/rust/wazeos-app" }
serde = { version = "1.0", features = ["derive"] }
serde_json = "1.0"

[[bin]]
name = "app"
path = "src/main.rs"

[profile.release]
opt-level = "z"     # Optimize for size
lto = true          # Enable Link Time Optimization
codegen-units = 1   # Better optimization
strip = true        # Strip symbols
panic = "abort"     # Smaller binary
`, appName, author, description)
}

func generateReadmeRust(author, appName, description string) string {
	return fmt.Sprintf(`# %s

%s

A WazeOS WASM application written in Rust that can be called as an MCP tool.

## Quick Start

`+"```bash"+`
# Build the app
wazeos apps build .

# Build and create package
wazeos apps package .

# Build, package, and install
wazeos apps install .
`+"```"+`

## Development

This app uses the WazeOS Rust SDK for building MCP tools.

### Defining Input Parameters

For Rust apps, you need to manually define the input schema in `+"`metadata.json`"+`:

`+"```json"+`
{
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
        "description": "Message to echo back",
        "default": ""
      }
    }
  }
}
`+"```"+`

Then create a matching Rust struct:

`+"```rust"+`
#[derive(serde::Deserialize, Debug)]
struct Input {
    #[serde(default)]
    message: String,
}
`+"```"+`

### Implementing the Handler

Edit the `+"`handle`"+` method in `+"`src/main.rs`"+` to process MCP tool calls:

`+"```rust"+`
impl MCPToolHandler for Tool {
    fn handle(&self, ctx: &Context, input: Value) -> Result<Value, Box<dyn std::error::Error>> {
        // Parse input
        let input: Input = serde_json::from_value(input)?;

        // Process with type safety
        let greeting = format!("Hello, {}!", input.message);

        // Return JSON response
        Ok(json!({
            "status": "success",
            "message": greeting,
        }))
    }
}
`+"```"+`

### Using the Context

The `+"`Context`"+` provides access to execution metadata:

`+"```rust"+`
// Logging
ctx.info("Operation complete");
ctx.warn("This is a warning");
ctx.error("Something went wrong");

// Check permissions
if !ctx.has_permission("file:///tmp/*", &["read"]) {
    return Err("Permission denied".into());
}

// Access metadata
let request_id = &ctx.request_id;
let principal = &ctx.principal;
`+"```"+`

### MCP Tool Integration

When installed, this app is automatically exposed as an MCP tool that Claude can call.
Make sure the `+"`inputSchema`"+` in `+"`metadata.json`"+` matches your Input struct.

## Build Process

When you run `+"`wazeos apps build`"+`, it:
1. Detects the Rust language from `+"`Cargo.toml`"+`
2. Compiles to WASM using `+"`cargo build --target wasm32-wasi --release`"+`
3. Copies the WASM binary to `+"`app.wasm`"+`

**Note:** Unlike Go apps, Rust apps require manual schema maintenance in `+"`metadata.json`"+`.

## Testing

`+"```bash"+`
# Build first
wazeos apps build .

# Test with sample input (requires wasmtime or similar)
echo '{"message": "World"}' | wasmtime app.wasm

# Expected output:
# {"status":"success","message":"Hello, World!"}
`+"```"+`

## Structure

- `+"`src/main.rs`"+` - Application implementation
- `+"`Cargo.toml`"+` - Rust package manifest
- `+"`metadata.json`"+` - Package metadata (manually maintained)

## Requirements

- [Rust](https://rustup.rs/) with `+"`wasm32-wasi`"+` target
- WazeOS CLI

Install the WASM target:
`+"```bash"+`
rustup target add wasm32-wasi
`+"```"+`

## Author

%s
`, appName, description, appName, author, description, author)
}

func generateGitignoreRust() string {
	return `# Build artifacts
app.wasm
*.zip

# Rust build
/target/
Cargo.lock

# IDE
.vscode/
.idea/
*.swp
*.swo
*~

# OS files
.DS_Store
Thumbs.db
`
}
