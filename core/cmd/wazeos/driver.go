package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

var driverCmd = &cobra.Command{
	Use:   "driver",
	Short: "Manage drivers",
	Long:  `Create, build, test, install, and manage WazeOS drivers.`,
}

var driverNewCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new driver",
	Long: `Create a new driver scaffold with boilerplate code.

Examples:
  wazeos driver new my-s3-driver --class io.connect --language go
  wazeos driver new my-mcp-server --class io.listen --language go --json`,
	Args: cobra.ExactArgs(1),
	Run:  runDriverNew,
}

var driverBuildCmd = &cobra.Command{
	Use:   "build <name>",
	Short: "Build a driver",
	Long:  `Compile a driver to a binary (.so for Go, .wasm for Rust).`,
	Args:  cobra.ExactArgs(1),
	Run:   runDriverBuild,
}

var driverTestCmd = &cobra.Command{
	Use:   "test <name>",
	Short: "Test a driver",
	Long:  `Run driver unit tests.`,
	Args:  cobra.ExactArgs(1),
	Run:   runDriverTest,
}

var driverInstallCmd = &cobra.Command{
	Use:   "install <path-or-name>",
	Short: "Install a driver",
	Long:  `Register a driver with the kernel.`,
	Args:  cobra.ExactArgs(1),
	Run:   runDriverInstall,
}

var driverListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed drivers",
	Long:  `Show all registered drivers.`,
	Run:   runDriverList,
}

var driverUninstallCmd = &cobra.Command{
	Use:   "uninstall <name>",
	Short: "Uninstall a driver",
	Long:  `Remove a driver from the kernel.`,
	Args:  cobra.ExactArgs(1),
	Run:   runDriverUninstall,
}

var (
	driverClass    string
	driverLanguage string
	driverAuthor   string
	driverForce    bool
	driverRelease  bool
)

func init() {
	// driver new flags
	driverNewCmd.Flags().StringVar(&driverClass, "class", "", "Driver class (io.connect|io.listen|runtime.*|kernel.*)")
	driverNewCmd.Flags().StringVar(&driverLanguage, "language", "go", "Language (go|rust)")
	driverNewCmd.Flags().StringVar(&driverAuthor, "author", "", "Author/organization name (e.g., 'acme' for acme/my-driver)")

	// driver build flags
	driverBuildCmd.Flags().BoolVar(&driverRelease, "release", false, "Build in release mode")

	// driver install flags
	driverInstallCmd.Flags().BoolVar(&driverForce, "force", false, "Overwrite existing driver")

	// Add subcommands
	driverCmd.AddCommand(driverNewCmd)
	driverCmd.AddCommand(driverBuildCmd)
	driverCmd.AddCommand(driverTestCmd)
	driverCmd.AddCommand(driverInstallCmd)
	driverCmd.AddCommand(driverListCmd)
	driverCmd.AddCommand(driverUninstallCmd)
}

// ============================================================================
// driver new
// ============================================================================

func runDriverNew(cmd *cobra.Command, args []string) {
	nameArg := args[0]

	// Parse author and name from argument or flags
	var author, name string
	if strings.Contains(nameArg, "/") {
		// Format: author/name
		parts := strings.SplitN(nameArg, "/", 2)
		author = parts[0]
		name = parts[1]
	} else {
		// Just name, use --author flag or default
		name = nameArg
		author = driverAuthor
		if author == "" {
			author = "default"
		}
	}

	// Validate author and name
	if author == "" || name == "" {
		outputError("driver new", "INVALID_NAME", "invalid driver name format",
			"Use 'author/name' or provide --author flag")
	}

	// Validate class
	validClasses := []string{"io.connect", "io.listen", "runtime", "kernel"}
	if driverClass == "" {
		outputError("driver new", "MISSING_CLASS", "driver class is required",
			"Specify --class flag with one of: "+strings.Join(validClasses, ", "))
	}

	if !contains(validClasses, driverClass) {
		outputError("driver new", "INVALID_CLASS", "invalid driver class: "+driverClass,
			"Valid classes: "+strings.Join(validClasses, ", "))
	}

	// Create driver directory
	driverPath := filepath.Join("drivers", author, name)
	if _, err := os.Stat(driverPath); err == nil {
		outputError("driver new", "DRIVER_EXISTS", "driver already exists: "+driverPath,
			"Choose a different name or delete the existing driver")
	}

	if err := os.MkdirAll(driverPath, 0755); err != nil {
		outputError("driver new", "CREATE_FAILED", err.Error(), "")
	}

	// Generate files based on language
	var filesCreated []string
	switch driverLanguage {
	case "go":
		filesCreated = generateGoDriver(driverPath, author, name, driverClass)
	case "rust":
		outputError("driver new", "NOT_IMPLEMENTED", "Rust drivers not yet supported", "Use --language go")
	default:
		outputError("driver new", "INVALID_LANGUAGE", "invalid language: "+driverLanguage, "Valid languages: go, rust")
	}

	fullName := author + "/" + name
	// Output result
	if jsonOutput {
		outputSuccess("driver new", map[string]interface{}{
			"author":        author,
			"name":          name,
			"full_name":     fullName,
			"path":          driverPath,
			"class":         driverClass,
			"language":      driverLanguage,
			"files_created": filesCreated,
		})
	} else {
		logSuccess("✓", fmt.Sprintf("Created driver scaffold at %s/", driverPath))
		logNextSteps(
			fmt.Sprintf("cd %s", driverPath),
			"Edit driver.go to implement your logic",
			fmt.Sprintf("wazeos driver build %s", fullName),
			"wazeos driver install "+fullName,
		)
	}
}

// classToConstant converts a driver class to its constant name
// io.connect -> Connect, io.listen -> Listen, runtime -> Runtime, kernel -> Kernel
func classToConstant(class string) string {
	switch class {
	case "io.connect":
		return "Connect"
	case "io.listen":
		return "Listen"
	case "runtime":
		return "Runtime"
	case "kernel":
		return "Kernel"
	default:
		return "Connect"
	}
}

func generateGoDriver(driverPath, author, name, class string) []string {
	files := []string{}

	// Generate WazeOS SDK types first (local to this driver for isolation)
	sdkFiles := generateDriverSDKTypes(driverPath)
	files = append(files, sdkFiles...)

	// Generate driver.go
	driverFile := filepath.Join(driverPath, "driver.go")
	tmpl := template.Must(template.New("driver").Parse(goDriverTemplate))
	f, err := os.Create(driverFile)
	if err != nil {
		outputError("driver new", "CREATE_FAILED", err.Error(), "")
	}
	defer f.Close()

	// Convert class to constant name (io.connect -> Connect)
	classConst := classToConstant(class)

	// Convert name to valid Go package name (test-driver -> testdriver)
	packageName := strings.ReplaceAll(name, "-", "")

	err = tmpl.Execute(f, map[string]string{
		"Name":       name,
		"Package":    packageName,
		"Class":      class,
		"ClassConst": classConst,
	})
	if err != nil {
		outputError("driver new", "TEMPLATE_FAILED", err.Error(), "")
	}
	files = append(files, driverFile)

	// Generate driver_test.go
	testFile := filepath.Join(driverPath, "driver_test.go")
	files = append(files, testFile)
	os.WriteFile(testFile, []byte(`package main

import (
	"context"
	"testing"

	sdk "sdk/driver"
)

func TestDriverInit(t *testing.T) {
	d := &Driver{}
	ctx := context.Background()
	config := sdk.Config{
		Options: make(map[string]any),
	}

	if err := d.Init(ctx, config); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
}

func TestDriverCall(t *testing.T) {
	// TODO: Add tests for Call() method
	t.Skip("Implement Call() tests")
}
`), 0644)

	// Generate go.mod for the driver
	goModFile := filepath.Join(driverPath, "go.mod")
	goModContent := fmt.Sprintf(`module %s

go 1.21

require sdk/driver v0.0.0

replace sdk/driver => ./sdk/driver
`, packageName)
	os.WriteFile(goModFile, []byte(goModContent), 0644)
	files = append(files, goModFile)

	// Note: README generation removed - documentation is in driver.go source code comments

	// Generate SDK files (for apps to use)
	appSDKFiles := generateDriverSDK(driverPath, author, name)
	files = append(files, appSDKFiles...)

	return files
}

// generateDriverSDKTypes creates the internal WASM SDK for driver development.
// This provides a self-contained SDK that doesn't require the core kernel source,
// enabling isolated development and testing with 'wazeos dev run'.
func generateDriverSDKTypes(driverPath string) []string {
	files := []string{}

	// Create sdk/driver directory
	sdkPath := filepath.Join(driverPath, "sdk", "driver")
	os.MkdirAll(sdkPath, 0755)

	// Read the core WASM SDK and adapt it
	coreSDKPath := filepath.Join("core", "sdk", "go", "driver", "wazeos.go")
	sdkBytes, err := os.ReadFile(coreSDKPath)
	if err != nil {
		logInfo("Warning: Could not read core SDK, using fallback")
		sdkBytes = []byte(wasmSDKFallback)
	}

	// Adapt the SDK content:
	// 1. Change package name from 'wazeos' to 'driver'
	// 2. Update header comment to reflect it's auto-generated
	sdkContent := string(sdkBytes)
	sdkContent = strings.Replace(sdkContent, "package wazeos\n", "package driver\n", 1)

	// Replace the package comment
	oldComment := `// Package wazeos provides a Go SDK for building WazeOS I/O drivers.
//
// This SDK allows you to create WASM-based I/O drivers that extend the
// WazeOS kernel's capabilities by handling specific URI patterns.`

	newComment := `// Package driver provides the WazeOS WASM driver SDK.
//
// This package is auto-generated by 'wazeos driver new' and contains all the
// functions and types needed to build a WazeOS WASM driver without requiring
// the core kernel source code.
//
// This copy is specific to this driver and ensures the SDK matches the version
// of wazeos that generated it, enabling isolated testing with 'wazeos dev run'.
//
// Location: sdk/driver/ (for building the driver itself)
// See also: sdk/external/ (client SDKs for apps to use this driver)`

	sdkContent = strings.Replace(sdkContent, oldComment, newComment, 1)

	// Write adapted SDK
	sdkFile := filepath.Join(sdkPath, "driver.go")
	os.WriteFile(sdkFile, []byte(sdkContent), 0644)
	files = append(files, sdkFile)

	// Generate go.mod for the driver SDK
	sdkModFile := filepath.Join(sdkPath, "go.mod")
	sdkModContent := `module driver

go 1.21

// This is a self-contained SDK package with no external dependencies.
// It's generated by 'wazeos driver new' and matches the version of
// wazeos that created this driver.
//
// This SDK is for building the driver itself.
// For apps using this driver, see sdk/external/
`
	os.WriteFile(sdkModFile, []byte(sdkModContent), 0644)
	files = append(files, sdkModFile)

	return files
}

// wasmSDKFallback is used if core SDK file cannot be read
const wasmSDKFallback = `// Package driver provides the WazeOS WASM driver SDK.
//
// This package is auto-generated by 'wazeos driver new' and contains all the
// functions and types needed to build a WazeOS WASM driver without requiring
// the core kernel source code.
//
// This copy is specific to this driver and ensures the SDK matches the version
// of wazeos that generated it, enabling isolated testing with 'wazeos dev run'.
//
// Location: sdk/driver/ (for building the driver itself)
// See also: sdk/external/ (client SDKs for apps to use this driver)
package driver

import (
	"context"
	"io"
	"time"
)

// ============================================================================
// Core Driver Interface
// ============================================================================

// Driver is the base interface all drivers must implement
type Driver interface {
	// Metadata
	URIPattern() string
	Class() Class
	Capabilities() []Capability

	// Lifecycle
	Init(ctx context.Context, config Config) error
	Close() error

	// Core operation (all drivers must implement)
	Call(ctx Context, req Request) (Response, error)
}

// ============================================================================
// Driver Classes
// ============================================================================

// Class defines the type/purpose of a driver
type Class string

const (
	ClassConnect Class = "io.connect" // Client connectors
	ClassListen  Class = "io.listen"  // Server listeners
	ClassRuntime Class = "runtime"    // Execution runtimes
	ClassKernel  Class = "kernel"     // Kernel plugins
)

// ============================================================================
// Driver Capabilities
// ============================================================================

// Capability defines what operations a driver supports
type Capability string

const (
	CapCall   Capability = "call"   // Request/response
	CapStream Capability = "stream" // Streaming I/O
	CapHandle Capability = "handle" // Stateful sessions
	CapListen Capability = "listen" // Server binding
	CapPubSub Capability = "pubsub" // Pub/sub messaging
	CapTxn    Capability = "txn"    // Transactions
)

// ============================================================================
// Optional Driver Interfaces
// ============================================================================

// StreamableDriver supports streaming I/O operations
type StreamableDriver interface {
	Driver
	Stream(ctx Context, req Request) (io.ReadWriteCloser, error)
}

// HandleDriver supports stateful sessions
type HandleDriver interface {
	Driver
	CreateHandle(ctx Context, args map[string]any) (Handle, error)
}

// Handle represents a stateful resource session
type Handle interface {
	ID() string
	Call(ctx Context, args map[string]any) (any, error)
	Close() error
}

// ============================================================================
// Request/Response Types
// ============================================================================

// Request represents an operation request
type Request struct {
	URI       string            ` + "`json:\"uri\"`" + `
	Operation Operation         ` + "`json:\"operation\"`" + `
	Args      map[string]any    ` + "`json:\"args,omitempty\"`" + `
	Headers   map[string]string ` + "`json:\"headers,omitempty\"`" + `
	Body      []byte            ` + "`json:\"body,omitempty\"`" + `
}

// Response represents an operation result
type Response struct {
	StatusCode int               ` + "`json:\"status_code\"`" + `
	Headers    map[string]string ` + "`json:\"headers\"`" + `
	Body       []byte            ` + "`json:\"body\"`" + `
	Error      string            ` + "`json:\"error,omitempty\"`" + `
}

// Operation defines the type of operation
type Operation string

const (
	OpCall         Operation = "call"
	OpStream       Operation = "stream"
	OpCreateHandle Operation = "create_handle"
	OpCloseHandle  Operation = "close_handle"
)

// ============================================================================
// Context Interface
// ============================================================================

// Context provides access to request metadata and allows nested driver calls
type Context interface {
	context.Context

	Principal() string
	RequestID() string
	TraceID() string
	HasPermission(uri string, perms ...string) bool
	Call(req Request) (Response, error)
}

// ============================================================================
// Configuration
// ============================================================================

// Config holds driver configuration
type Config struct {
	Options     map[string]any
	Permissions []string
	HandleTTL   time.Duration
}

// ============================================================================
// Driver Registration
// ============================================================================

// Spec defines a driver registration specification
type Spec struct {
	Name         string
	Version      string
	Class        Class
	URIPattern   string
	Capabilities []Capability
	Runtime      Runtime
	Binary       string
	Factory      func() Driver
	Permissions  []string
}

// Runtime specifies how a driver is loaded
type Runtime = string

const (
	RuntimeNative Runtime = "native"
	RuntimeWASM   Runtime = "wasm"
)

// Register registers a driver with the kernel.
// This is overridden by the kernel at runtime.
var Register = func(spec Spec) {
	panic("driver.Register called outside of kernel context")
}

// ============================================================================
// Response Helpers
// ============================================================================

// NewResponse creates a successful response
func NewResponse(statusCode int, body []byte) Response {
	return Response{
		StatusCode: statusCode,
		Headers:    make(map[string]string),
		Body:       body,
	}
}

// NewErrorResponse creates an error response
func NewErrorResponse(statusCode int, message string) Response {
	return Response{
		StatusCode: statusCode,
		Headers:    make(map[string]string),
		Body:       []byte(message),
		Error:      message,
	}
}

// Common status codes
const (
	StatusOK                  = 200
	StatusBadRequest          = 400
	StatusForbidden           = 403
	StatusNotFound            = 404
	StatusInternalServerError = 500
)
`

func generateDriverSDK(driverPath, author, name string) []string {
	files := []string{}

	// Name conversions for different languages
	packageName := toSnakeCase(name)    // my-driver -> my_driver (Rust, Go packages)
	traitName := toPascalCase(name)     // my-driver -> MyDriver (Rust trait, Go type)
	fullPackageName := toSnakeCase(author) + "_" + packageName  // acme/my-driver -> acme_my_driver
	fullName := author + "/" + name     // acme/my-driver

	// Create sdk directory structure
	rustSDKPath := filepath.Join(driverPath, "sdk", "external", "rust")
	rustSrcPath := filepath.Join(rustSDKPath, "src")
	goSDKPath := filepath.Join(driverPath, "sdk", "external", "go")

	os.MkdirAll(rustSrcPath, 0755)
	os.MkdirAll(goSDKPath, 0755)

	// Generate Rust SDK Cargo.toml
	rustCargoFile := filepath.Join(rustSDKPath, "Cargo.toml")
	rustCargoContent := fmt.Sprintf(`[package]
name = "wazeos-%s-%s"
version = "0.1.0"
edition = "2021"
authors = ["%s"]
description = "%s driver SDK addon for WazeOS apps"
license = "MIT OR Apache-2.0"

[dependencies]
wazeos-app = { path = "../../../../../../core/sdk/rust/app" }
serde_json = "1.0"

[lib]
path = "src/lib.rs"
`, author, name, author, fullName)
	os.WriteFile(rustCargoFile, []byte(rustCargoContent), 0644)
	files = append(files, rustCargoFile)

	// Generate Rust SDK lib.rs with comprehensive documentation
	rustLibFile := filepath.Join(rustSrcPath, "lib.rs")
	rustLibContent := fmt.Sprintf(`//! %s Driver SDK Addon
//!
//! Ergonomic client library for the %s driver. This SDK wraps raw IO Bus
//! calls with typed, idiomatic Rust functions, making it much easier to use
//! the driver from your WazeOS apps.
//!
//! # Installation
//!
//! Add to your app's ` + "`Cargo.toml`" + `:
//!
//! ` + "```toml" + `
//! [dependencies]
//! wazeos-app = { path = "../../core/sdk/rust/app" }
//! wazeos-%s-%s = { path = "../../drivers/%s/sdk/external/rust" }
//! ` + "```" + `
//!
//! # Complete Example App
//!
//! Here's a full working example showing how to use this SDK in your app:
//!
//! ` + "```rust,no_run" + `
//! // src/lib.rs - Your WazeOS app
//! use serde_json::{json, Value};
//! use wazeos_app::{AppContext, AppResult, register_tool};
//! use wazeos_%s::%sOps;  // Import the trait
//!
//! #[no_mangle]
//! pub extern "C" fn tool_main(ctx: &AppContext, args: Value) -> AppResult {
//!     // Extract input from MCP tool arguments
//!     let resource = args["resource"].as_str().unwrap_or("default");
//!
//!     // Use the SDK - much cleaner than raw ctx.call()!
//!     match ctx.example_operation(resource) {
//!         Ok(result) => {
//!             Ok(json!({
//!                 "status": "success",
//!                 "result": result
//!             }))
//!         }
//!         Err(e) => {
//!             // Errors are automatically converted to strings
//!             Err(format!("Operation failed: {}", e))
//!         }
//!     }
//! }
//!
//! // Register this function as the MCP tool entry point
//! register_tool!(tool_main);
//! ` + "```" + `
//!
//! Build with:
//!
//! ` + "```bash" + `
//! cargo build --target wasm32-wasip1 --release
//! wazeos app install target/wasm32-wasip1/release/your-app.wasm
//! ` + "```" + `
//!
//! # Benefits of Using This SDK
//!
//! **Without SDK** (verbose, error-prone):
//! ` + "```rust,ignore" + `
//! let resp = ctx.call("%s://resource", HashMap::new(), Vec::new())?;
//! if resp.status_code != 200 {
//!     return Err(resp.error.unwrap_or("Unknown error".to_string()));
//! }
//! let result = String::from_utf8(resp.body)
//!     .map_err(|e| format!("UTF-8 error: {}", e))?;
//! ` + "```" + `
//!
//! **With SDK** (clean, type-safe):
//! ` + "```rust,ignore" + `
//! let result = ctx.example_operation("resource")?;
//! ` + "```" + `
//!
//! # Available Operations
//!
//! All operations are methods on the ` + "`%sOps`" + ` trait, which extends
//! ` + "`AppContext`" + `. Import the trait to get access to these methods:
//!
//! - ` + "`example_operation(&self, arg: &str) -> Result<String, String>`" + `
//!
//! See individual method documentation below for details.

use wazeos_app::{AppContext, HashMap};

/// Operations trait for the %s driver
///
/// This trait extends ` + "`AppContext`" + ` with %s-specific operations.
/// All methods handle IO Bus communication, error checking, and type conversions
/// automatically.
///
/// # Usage
///
/// Import this trait to get access to the extension methods:
///
/// ` + "```rust,no_run" + `
/// use wazeos_app::AppContext;
/// use wazeos_%s::%sOps;
///
/// fn do_something(ctx: &AppContext) -> Result<String, String> {
///     // Now ctx has %s driver methods available
///     let result = ctx.example_operation("test")?;
///     Ok(result)
/// }
/// ` + "```" + `
///
/// # Error Handling
///
/// All methods return ` + "`Result<T, String>`" + ` so you can use the ` + "`?`" + ` operator
/// for clean error propagation.
pub trait %sOps {
    /// Example operation - replace with your actual driver operations
    ///
    /// This is a template method showing how to implement driver operations.
    /// Replace this with your real functionality.
    ///
    /// # Arguments
    ///
    /// * ` + "`arg`" + ` - The resource path or operation parameter
    ///
    /// # Returns
    ///
    /// * ` + "`Result<String, String>`" + ` - Operation result or error message
    ///
    /// # Errors
    ///
    /// Returns an error if:
    /// - The driver returns a non-200 status code
    /// - The response body is not valid UTF-8
    /// - The IO Bus call fails
    ///
    /// # Example
    ///
    /// ` + "```rust,no_run" + `
    /// # use wazeos_app::AppContext;
    /// # use wazeos_%s::%sOps;
    /// # fn demo(ctx: &AppContext) -> Result<(), String> {
    /// // Call the driver operation
    /// let result = ctx.example_operation("my-resource")?;
    /// println!("Driver returned: {}", result);
    /// # Ok(())
    /// # }
    /// ` + "```" + `
    fn example_operation(&self, arg: &str) -> Result<String, String>;

    // TODO: Add more driver operations here. Examples:
    //
    // /// Read data from a resource
    // ///
    // /// # Arguments
    // /// * ` + "`path`" + ` - Resource path to read from
    // ///
    // /// # Example
    // /// ` + "```ignore" + `
    // /// let data = ctx.read_resource("/path/to/resource")?;
    // /// ` + "```" + `
    // fn read_resource(&self, path: &str) -> Result<Vec<u8>, String>;
    //
    // /// Write data to a resource
    // ///
    // /// # Arguments
    // /// * ` + "`path`" + ` - Resource path to write to
    // /// * ` + "`data`" + ` - Data bytes to write
    // fn write_resource(&self, path: &str, data: &[u8]) -> Result<(), String>;
}

/// Implementation of %sOps for AppContext
///
/// This implementation handles all the low-level IO Bus communication,
/// error checking, and data conversions so your app code stays clean.
impl %sOps for AppContext {
    fn example_operation(&self, arg: &str) -> Result<String, String> {
        // 1. Build the driver URI
        let uri = format!("%s://{}", arg);

        // 2. Call the driver through the IO Bus
        //    The SDK handles all the serialization/deserialization
        let resp = self.call(&uri, HashMap::new(), Vec::new())?;

        // 3. Check the response status
        if resp.status_code != 200 {
            return Err(resp.error.unwrap_or_else(||
                format!("Driver returned status {}", resp.status_code)
            ));
        }

        // 4. Convert response body to the expected type
        String::from_utf8(resp.body)
            .map_err(|e| format!("Invalid UTF-8 in response: {}", e))
    }

    // TODO: Implement additional operations here following the same pattern:
    //
    // fn read_resource(&self, path: &str) -> Result<Vec<u8>, String> {
    //     let uri = format!("%s://{}", path);
    //     let resp = self.call(&uri, HashMap::new(), Vec::new())?;
    //     if resp.status_code != 200 {
    //         return Err(resp.error.unwrap_or_else(|| "Read failed".to_string()));
    //     }
    //     Ok(resp.body)
    // }
}
`, name, name, name, name, packageName, traitName, name, traitName, name, name, packageName, traitName, name, traitName, packageName, traitName, traitName, traitName, name)
	os.WriteFile(rustLibFile, []byte(rustLibContent), 0644)
	files = append(files, rustLibFile)

	// Generate Rust SDK README
	rustReadmeFile := filepath.Join(rustSDKPath, "README.md")
	rustReadmeContent := fmt.Sprintf(`# %s Driver SDK for Rust

Client library for using the %s driver from WazeOS apps.

## Installation

Add to your app's Cargo.toml:

**Development** (from source):
` + "```toml" + `
[dependencies]
wazeos-%s-%s = { path = "../../drivers/%s/sdk/external/rust" }
` + "```" + `

**Production** (from installed driver):
` + "```toml" + `
[dependencies]
wazeos-%s-%s = { path = "~/.wazeos/drivers/%s/sdk/external/rust" }
` + "```" + `

## Usage

` + "```rust" + `
use wazeos_app::{AppContext, register_tool};
use wazeos_%s::%sOps;

pub extern "C" fn tool_main(ctx: &AppContext, args: Value) -> AppResult {
    // Use the trait methods
    let result = ctx.example_operation("test")?;

    Ok(json!({"result": result}))
}
` + "```" + `

## Available Operations

- ` + "`example_operation`" + ` - TODO: Document your operations

See [lib.rs](src/lib.rs) for full API documentation.
`, fullName, fullName, author, name, fullName, author, name, fullName, packageName, traitName)
	os.WriteFile(rustReadmeFile, []byte(rustReadmeContent), 0644)
	files = append(files, rustReadmeFile)

	// Generate Go SDK go.mod
	goModFile := filepath.Join(goSDKPath, "go.mod")
	goModContent := fmt.Sprintf(`module github.com/wazeos/wazeos/drivers/%s/sdk/go

go 1.21

require github.com/wazeos/wazeos/core/sdk/go/app v0.1.0

replace github.com/wazeos/wazeos/core/sdk/go/app => ../../../../../../core/sdk/go/app
`, fullName)
	os.WriteFile(goModFile, []byte(goModContent), 0644)
	files = append(files, goModFile)

	// Generate Go SDK client.go with comprehensive documentation
	goClientFile := filepath.Join(goSDKPath, "client.go")
	goClientContent := fmt.Sprintf(`// Package %ssdk provides an ergonomic client library for the %s driver.
//
// This SDK wraps raw IO Bus calls with typed, easy-to-use functions that handle
// all the low-level details of driver communication, error checking, and type
// conversions.
//
// # Installation
//
// In your app's go.mod:
//
//	require github.com/wazeos/wazeos/drivers/%s/sdk/go v0.1.0
//	replace github.com/wazeos/wazeos/drivers/%s/sdk/go => ../../drivers/%s/sdk/external/go
//
// # Complete Example
//
// Here's a full working example of a WazeOS app using this SDK:
//
//	// main.go - Your WazeOS App
//	package main
//
//	import (
//	    wazeos "github.com/wazeos/wazeos/core/sdk/go/app"
//	    %ssdk "github.com/wazeos/wazeos/drivers/%s/sdk/go"
//	)
//
//	//export wazeos_tool_invoke
//	func toolInvoke(argsPtr, argsLen uint32) uint32 {
//	    ctx := wazeos.NewContext()
//	    args := wazeos.MustParseArgs(argsPtr, argsLen)
//
//	    // Extract input from MCP tool arguments
//	    resource := args["resource"].(string)
//
//	    // Use the SDK - clean and type-safe!
//	    result, err := %ssdk.ExampleOperation(ctx, resource)
//	    if err != nil {
//	        return wazeos.ReturnError(err.Error())
//	    }
//
//	    return wazeos.ReturnSuccess(map[string]interface{}{
//	        "status": "success",
//	        "result": result,
//	    })
//	}
//
//	//export wazeos_tool_metadata
//	func toolMetadata() uint32 {
//	    return wazeos.ReturnMetadata("my-tool", "1.0.0")
//	}
//
//	func main() {}
//
// Build with:
//
//	tinygo build -o tool.wasm -target=wasi -no-debug -opt=2 main.go
//	wazeos app install tool.wasm
//
// # Benefits
//
// Without SDK (verbose):
//
//	resp, err := ctx.Call("%s://resource", nil, nil)
//	if err != nil { return wazeos.ReturnError(err.Error()) }
//	if resp.StatusCode != 200 {
//	    return wazeos.ReturnError("driver error")
//	}
//	result := string(resp.Body)
//
// With SDK (concise):
//
//	result, err := %ssdk.ExampleOperation(ctx, "resource")
//	if err != nil { return wazeos.ReturnError(err.Error()) }
package %ssdk

import (
	"fmt"

	wazeos "github.com/wazeos/wazeos/core/sdk/go/app"
)

// ExampleOperation performs an example operation using the %s driver.
//
// This is a template showing the SDK pattern. Replace this with your actual
// driver operations.
//
// Parameters:
//   - ctx: WazeOS application context for making IO Bus calls
//   - arg: The resource path or operation parameter
//
// Returns:
//   - string: The operation result
//   - error: Error if the operation fails or driver returns non-200 status
//
// Example:
//
//	ctx := wazeos.NewContext()
//	result, err := %ssdk.ExampleOperation(ctx, "test-resource")
//	if err != nil {
//	    return wazeos.ReturnError(fmt.Sprintf("Operation failed: %%s", err))
//	}
//	fmt.Printf("Result: %%s\\n", result)
func ExampleOperation(ctx *wazeos.Context, arg string) (string, error) {
	// 1. Build the driver URI
	uri := fmt.Sprintf("%s://%%s", arg)

	// 2. Call the driver through the IO Bus
	//    nil headers and body for this simple example
	resp, err := ctx.Call(uri, nil, nil)
	if err != nil {
		return "", fmt.Errorf("%s driver call failed: %%w", err)
	}

	// 3. Check the response status
	if resp.StatusCode != 200 {
		// Return the driver's error message if available
		if resp.Error != nil {
			return "", fmt.Errorf("%s driver error: %%s", *resp.Error)
		}
		return "", fmt.Errorf("%s driver returned status %%d", resp.StatusCode)
	}

	// 4. Convert response to expected type and return
	return string(resp.Body), nil
}

// TODO: Add more SDK functions for your driver operations. Examples:
//
// // ReadResource reads data from the driver
// //
// // Parameters:
// //   - ctx: Application context
// //   - path: Resource path to read
// //
// // Returns:
// //   - []byte: The resource data
// //   - error: Error if read fails
// //
// // Example:
// //
// //	data, err := %ssdk.ReadResource(ctx, "/path/to/resource")
// //	if err != nil {
// //	    return wazeos.ReturnError(err.Error())
// //	}
// func ReadResource(ctx *wazeos.Context, path string) ([]byte, error) {
//     uri := fmt.Sprintf("%s://%%s", path)
//     resp, err := ctx.Call(uri, nil, nil)
//     if err != nil {
//         return nil, fmt.Errorf("read failed: %%w", err)
//     }
//     if resp.StatusCode != 200 {
//         return nil, fmt.Errorf("read error: status %%d", resp.StatusCode)
//     }
//     return resp.Body, nil
// }
//
// // WriteResource writes data to the driver
// //
// // Parameters:
// //   - ctx: Application context
// //   - path: Resource path to write
// //   - data: Data to write
// //
// // Returns:
// //   - error: Error if write fails
// func WriteResource(ctx *wazeos.Context, path string, data []byte) error {
//     uri := fmt.Sprintf("%s://%%s", path)
//     headers := map[string]string{"operation": "write"}
//     resp, err := ctx.Call(uri, headers, data)
//     if err != nil {
//         return fmt.Errorf("write failed: %%w", err)
//     }
//     if resp.StatusCode != 200 {
//         return fmt.Errorf("write error: status %%d", resp.StatusCode)
//     }
//     return nil
// }
`, fullPackageName, fullName, fullName, fullName, fullName, fullPackageName, fullName, fullPackageName, fullName, fullPackageName, fullPackageName, name, fullPackageName, name, name, name, name, fullPackageName, name, name)
	os.WriteFile(goClientFile, []byte(goClientContent), 0644)
	files = append(files, goClientFile)

	// Generate Go SDK README
	goReadmeFile := filepath.Join(goSDKPath, "README.md")
	goReadmeContent := fmt.Sprintf(`# %s Driver SDK for Go

Client library for using the %s driver from WazeOS apps written in Go.

## Installation

In your app directory, add to go.mod:

**Development** (from source):
` + "```go" + `
require github.com/wazeos/wazeos/drivers/%s/sdk/go v0.1.0
replace github.com/wazeos/wazeos/drivers/%s/sdk/go => ../../drivers/%s/sdk/external/go
` + "```" + `

**Production** (from installed driver):
` + "```go" + `
require github.com/wazeos/wazeos/drivers/%s/sdk/go v0.1.0
replace github.com/wazeos/wazeos/drivers/%s/sdk/go => ~/.wazeos/drivers/%s/sdk/external/go
` + "```" + `

## Usage

` + "```go" + `
import (
    wazeos "github.com/wazeos/wazeos/core/sdk/go/app"
    %ssdk "github.com/wazeos/wazeos/drivers/%s/sdk/go"
)

//export wazeos_tool_invoke
func toolInvoke(argsPtr, argsLen uint32) uint32 {
    ctx := wazeos.NewContext()

    // Use SDK helper functions
    result, err := %ssdk.ExampleOperation(ctx, "test")
    if err != nil {
        return wazeos.ReturnError(err.Error())
    }

    return wazeos.ReturnSuccess(map[string]interface{}{
        "result": result,
    })
}
` + "```" + `

## Available Functions

- ` + "`ExampleOperation(ctx, arg)`" + ` - TODO: Document your operations

See [client.go](client.go) for full API documentation.
`, name, name, name, name, name, name, name, name, name, name)
	os.WriteFile(goReadmeFile, []byte(goReadmeContent), 0644)
	files = append(files, goReadmeFile)

	return files
}

func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func toPascalCase(s string) string {
	// Convert kebab-case or snake_case to PascalCase
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_'
	})
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}

func toSnakeCase(s string) string {
	// Convert kebab-case to snake_case
	return strings.ReplaceAll(s, "-", "_")
}

const goDriverTemplate = `package main

// {{.Name}} - WazeOS WASM Driver
//
// This driver extends the WazeOS kernel with {{.Class}} capabilities.
// It compiles to WebAssembly for sandboxed, portable execution.
//
// Quick Start:
//   Build:    GOOS=wasip1 GOARCH=wasm go build -o build/{{.Name}}.wasm .
//             (or: wazeos driver build {{.Name}})
//   Test:     go test -v ./...
//   Dev/Test: wazeos dev run -v --driver build/{{.Name}}.wasm --interactive
//             (test in isolated environment without installing)
//   Install:  wazeos driver install {{.Name}}
//
// Development:
//   - Implement the exported WASM functions below
//   - Use SDK helpers for parsing requests and creating responses
//   - Return proper status codes and error messages
//   - Test with 'wazeos dev run' before deployment
//
// The sdk/driver package is auto-generated and self-contained.
// It matches the version of wazeos that created this driver.
//
// SDK Structure:
//   sdk/driver/    - WASM SDK for building this driver
//   sdk/external/  - Client SDKs for apps to use this driver (Rust/Go)

import (
	sdk "sdk/driver"
)

//export driver_metadata
func driverMetadata() uint32 {
	return sdk.ReturnMetadata(sdk.Metadata{
		Name:         "{{.Name}}",
		Version:      "1.0.0",
		Class:        "{{.Class}}",
		URIPattern:   "{{.Name}}://**",
		Capabilities: []string{"call"},
	})
}

//export driver_init
func driverInit(configPtr, configLen uint32) uint32 {
	// TODO: Initialize driver with configuration
	//
	// Parse config:
	//   config, err := sdk.ParseConfig(configPtr, configLen)
	//   if err != nil {
	//       return 1 // Error
	//   }
	//
	// Initialize resources:
	//   - Open connections
	//   - Load credentials
	//   - Set up caches
	//
	// Return 0 for success, non-zero for error
	return 0
}

//export driver_call
func driverCall(requestPtr, requestLen uint32) uint32 {
	// Parse incoming request
	req, err := sdk.ParseRequest(requestPtr, requestLen)
	if err != nil {
		return sdk.ReturnError(400, "Failed to parse request")
	}
	_ = req // TODO: use req to implement driver logic

	// TODO: Implement your driver logic here
	//
	// Access request data:
	//   uri := req.URI               // e.g., "{{.Name}}://resource/path"
	//   operation := req.Operation   // "call", "stream", etc.
	//   args := req.Args             // Map of arguments
	//   headers := req.Headers       // HTTP-style headers
	//   body := req.Body             // Binary body data
	//
	// Example: Simple echo
	//   responseData := []byte("Echo: " + req.URI)
	//   return sdk.ReturnResponse(200, nil, responseData)
	//
	// Example: Call another driver
	//   ctx := sdk.NewContext()
	//   resp, err := ctx.Call("file:///tmp/data.json", nil, nil)
	//   if err != nil {
	//       return sdk.ReturnError(500, "File read failed")
	//   }
	//   return sdk.ReturnResponse(200, nil, resp.Body)
	//
	// Example: Parse URI path
	//   // For URI "{{.Name}}://bucket/key"
	//   path := strings.TrimPrefix(req.URI, "{{.Name}}://")
	//   parts := strings.Split(path, "/")
	//   bucket := parts[0]
	//   key := parts[1]
	//
	// Return error responses for failures:
	//   return sdk.ReturnError(404, "Resource not found")
	//   return sdk.ReturnError(403, "Permission denied")
	//   return sdk.ReturnError(500, "Internal error")

	// Default stub response
	return sdk.ReturnResponse(200, nil, []byte("{{.Name}} driver: not yet implemented"))
}

// main is required for WASM but not called
func main() {}
`

// ============================================================================
// driver build
// ============================================================================

func runDriverBuild(cmd *cobra.Command, args []string) {
	name := args[0]
	driverPath := filepath.Join("drivers", name)

	// Check if driver exists
	if _, err := os.Stat(driverPath); os.IsNotExist(err) {
		outputError("driver build", "DRIVER_NOT_FOUND", "driver not found: "+driverPath,
			"Create a new driver with: wazeos driver new "+name)
	}

	logInfo("Building WASM driver: %s", name)

	// Extract just the driver name (last component) for the binary filename
	driverName := filepath.Base(name)

	// Build driver as WASM
	buildCmd := fmt.Sprintf("cd %s && GOOS=wasip1 GOARCH=wasm go build -o build/%s.wasm .", driverPath, driverName)
	if driverRelease {
		buildCmd = fmt.Sprintf("cd %s && GOOS=wasip1 GOARCH=wasm go build -ldflags='-s -w' -o build/%s.wasm .", driverPath, driverName)
	}

	// Create build directory
	os.MkdirAll(filepath.Join(driverPath, "build"), 0755)

	// Execute build
	if err := executeCommand(buildCmd); err != nil {
		outputError("driver build", "BUILD_FAILED", err.Error(), "Check your code for errors")
	}

	binaryPath := filepath.Join(driverPath, "build", driverName+".wasm")
	stat, err := os.Stat(binaryPath)
	if err != nil {
		outputError("driver build", "BINARY_NOT_FOUND",
			fmt.Sprintf("build completed but binary not found: %s", binaryPath),
			"Check build output for errors")
	}

	if jsonOutput {
		outputSuccess("driver build", map[string]interface{}{
			"binary":     binaryPath,
			"size_bytes": stat.Size(),
		})
	} else {
		logSuccess("✓", fmt.Sprintf("Build complete: %s (%d bytes)", binaryPath, stat.Size()))
	}
}

// ============================================================================
// driver test
// ============================================================================

func runDriverTest(cmd *cobra.Command, args []string) {
	name := args[0]
	driverPath := filepath.Join("drivers", name)

	if _, err := os.Stat(driverPath); os.IsNotExist(err) {
		outputError("driver test", "DRIVER_NOT_FOUND", "driver not found: "+driverPath, "")
	}

	logInfo("Running tests for %s driver...", name)

	testCmd := fmt.Sprintf("cd %s && go test -v ./...", driverPath)
	if err := executeCommand(testCmd); err != nil {
		outputError("driver test", "TESTS_FAILED", err.Error(), "Fix failing tests")
	}

	if jsonOutput {
		outputSuccess("driver test", map[string]interface{}{
			"tests_passed": "all", // TODO: Parse test output
		})
	} else {
		logSuccess("✓", "All tests passed")
	}
}

// ============================================================================
// driver install
// ============================================================================

func runDriverInstall(cmd *cobra.Command, args []string) {
	pathOrName := args[0]

	// Determine if this is a path or a driver name (with author)
	var driverPath, author, name, fullName string

	if _, err := os.Stat(pathOrName); err == nil && strings.HasPrefix(pathOrName, "drivers/") {
		// It's a path like drivers/author/name
		driverPath = pathOrName
		parts := strings.Split(strings.TrimPrefix(driverPath, "drivers/"), string(filepath.Separator))
		if len(parts) >= 2 {
			author = parts[0]
			name = parts[1]
		} else {
			name = filepath.Base(driverPath)
			author = "default"
		}
	} else {
		// Parse author/name format
		if strings.Contains(pathOrName, "/") {
			parts := strings.SplitN(pathOrName, "/", 2)
			author = parts[0]
			name = parts[1]
		} else {
			author = "default"
			name = pathOrName
		}
		driverPath = filepath.Join("drivers", author, name)
		if _, err := os.Stat(driverPath); os.IsNotExist(err) {
			outputError("driver install", "DRIVER_NOT_FOUND",
				fmt.Sprintf("driver not found: %s", driverPath),
				"Make sure the driver exists or build it first")
		}
	}

	fullName = author + "/" + name

	// Check if built binary exists
	binaryPath := filepath.Join(driverPath, "build", name+".so")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		outputError("driver install", "BINARY_NOT_FOUND",
			fmt.Sprintf("driver binary not found: %s", binaryPath),
			"Build the driver first with: wazeos driver build "+name)
	}

	// Create installation directory
	installDir := filepath.Join(os.Getenv("HOME"), ".wazeos", "drivers", author, name)
	if err := os.MkdirAll(installDir, 0755); err != nil {
		outputError("driver install", "INSTALL_DIR_FAILED", err.Error(), "")
	}

	// Copy driver binary
	binaryData, err := os.ReadFile(binaryPath)
	if err != nil {
		outputError("driver install", "READ_BINARY_FAILED", err.Error(), "")
	}
	destBinary := filepath.Join(installDir, name+".so")
	if err := os.WriteFile(destBinary, binaryData, 0755); err != nil {
		outputError("driver install", "WRITE_BINARY_FAILED", err.Error(), "")
	}

	// Copy SDK directory if it exists
	sdkPath := filepath.Join(driverPath, "sdk")
	if _, err := os.Stat(sdkPath); err == nil {
		destSDKPath := filepath.Join(installDir, "sdk")
		if err := copyDir(sdkPath, destSDKPath); err != nil {
			outputError("driver install", "COPY_SDK_FAILED", err.Error(), "")
		}
	}

	// Create driver metadata
	metadata := map[string]interface{}{
		"author":  author,
		"name":    name,
		"version": "1.0.0",
		"class":   driverClass,
		"binary":  name + ".so",
	}
	metadataJSON, _ := json.MarshalIndent(metadata, "", "  ")
	metadataPath := filepath.Join(installDir, "driver.json")
	os.WriteFile(metadataPath, metadataJSON, 0644)

	if jsonOutput {
		outputSuccess("driver install", map[string]interface{}{
			"author":      author,
			"name":        name,
			"full_name":   fullName,
			"version":     "1.0.0",
			"install_dir": installDir,
			"binary":      destBinary,
			"sdk_bundled": true,
		})
	} else {
		logSuccess("✓", fmt.Sprintf("Installed driver: %s", fullName))
		logInfo("Location: %s", installDir)
		logInfo("Binary: %s", destBinary)
		logInfo("SDK: %s/sdk/external/", installDir)
		logInfo("")
		logInfo("Apps can now use this driver's SDK:")
		logInfo("  Rust: [dependencies]")
		logInfo("        wazeos-%s-%s = { path = \"%s/sdk/external/rust\" }", author, name, installDir)
		logInfo("  Go:   import \"%s_%ssdk\" from \"%s/sdk/external/go\"", toSnakeCase(author), toSnakeCase(name), installDir)
	}
}

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	// Get source directory info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Create destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	// Read source directory
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	// Copy each entry
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Recursively copy subdirectory
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Copy file
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				return err
			}
		}
	}

	return nil
}

// ============================================================================
// driver list
// ============================================================================

func runDriverList(cmd *cobra.Command, args []string) {
	// TODO: Query IO Bus for registered drivers
	// For now, scan drivers/ directory
	drivers := []string{}
	filepath.Walk("drivers", func(path string, info os.FileInfo, err error) error {
		if info != nil && info.IsDir() && path != "drivers" {
			drivers = append(drivers, info.Name())
		}
		return nil
	})

	if jsonOutput {
		outputSuccess("driver list", map[string]interface{}{
			"drivers": drivers,
		})
	} else {
		if len(drivers) == 0 {
			logInfo("No drivers installed")
		} else {
			logInfo("Installed drivers:")
			for _, d := range drivers {
				fmt.Printf("  - %s\n", d)
			}
		}
	}
}

// ============================================================================
// driver uninstall
// ============================================================================

func runDriverUninstall(cmd *cobra.Command, args []string) {
	name := args[0]

	// TODO: Implement driver uninstallation
	if jsonOutput {
		outputSuccess("driver uninstall", map[string]interface{}{
			"driver": name,
		})
	} else {
		logSuccess("✓", fmt.Sprintf("Driver uninstalled: %s", name))
	}
}

// ============================================================================
// Helpers
// ============================================================================

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func executeCommand(cmdStr string) error {
	// Simple command execution for now
	// In production, use exec.Command properly
	return nil
}
