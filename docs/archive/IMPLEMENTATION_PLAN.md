# Implementation Plan: Complete App SDK & Packaging

This document outlines what needs to be implemented to make WazeOS apps fully functional and shareable.

## 1. Implement AppContext Methods (High Priority)

### Problem
The AppContext in `sdk/rust/wazeos-app/src/lib.rs` has stubbed methods:
```rust
pub fn read_file(&self, _path: &str) -> Result<String, String> {
    // TODO: Implement via iobus_call to file driver
    Err("read_file not yet implemented".to_string())
}
```

### Solution
Apps need to call drivers through the IO Bus using the same host functions that drivers use.

#### Add Host Function Imports to App SDK

**File:** `sdk/rust/wazeos-app/src/lib.rs`

```rust
// Add to imports
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

// Add host function binding (same as in wazeos-driver)
#[link(wasm_import_module = "env")]
extern "C" {
    fn host_iobus_call(ptr: u32, length: u32) -> u64;
}

// Add Request/Response types (same as driver SDK)
#[derive(Debug, Serialize, Deserialize)]
struct IOBusRequest {
    uri: String,
    operation: String,
    #[serde(default)]
    args: HashMap<String, serde_json::Value>,
    #[serde(default)]
    headers: HashMap<String, String>,
    #[serde(default)]
    body: Vec<u8>,
}

#[derive(Debug, Serialize, Deserialize)]
struct IOBusResponse {
    status_code: u16,
    #[serde(default)]
    headers: HashMap<String, String>,
    #[serde(default)]
    body: Vec<u8>,
    #[serde(skip_serializing_if = "Option::is_none")]
    error: Option<String>,
}

// Internal helper to call IO Bus
fn iobus_call(req: &IOBusRequest) -> Result<IOBusResponse, String> {
    unsafe {
        let req_json = serde_json::to_string(req)
            .map_err(|e| format!("Failed to serialize request: {}", e))?;

        let req_bytes = req_json.as_bytes();
        let req_ptr = req_bytes.as_ptr() as u32;
        let req_len = req_bytes.len() as u32;

        let result = host_iobus_call(req_ptr, req_len);

        if result == 0 {
            return Err("IO Bus call failed".to_string());
        }

        let resp_ptr = (result >> 32) as u32;
        let resp_len = (result & 0xFFFFFFFF) as u32;

        let resp_slice = std::slice::from_raw_parts(resp_ptr as *const u8, resp_len as usize);
        let resp: IOBusResponse = serde_json::from_slice(resp_slice)
            .map_err(|e| format!("Failed to deserialize response: {}", e))?;

        Ok(resp)
    }
}
```

#### Implement AppContext Methods

```rust
impl AppContext {
    /// Read a file from the file system
    pub fn read_file(&self, path: &str) -> Result<String, String> {
        let req = IOBusRequest {
            uri: format!("file://{}", path),
            operation: "call".to_string(),
            args: HashMap::new(),
            headers: HashMap::new(),
            body: Vec::new(),
        };

        let resp = iobus_call(&req)?;

        if resp.status_code != 200 {
            return Err(resp.error.unwrap_or_else(|| "File read failed".to_string()));
        }

        String::from_utf8(resp.body)
            .map_err(|e| format!("Invalid UTF-8: {}", e))
    }

    /// Write content to a file
    pub fn write_file(&self, path: &str, content: &str) -> Result<(), String> {
        let mut args = HashMap::new();
        args.insert("content".to_string(), serde_json::json!(content));

        let req = IOBusRequest {
            uri: format!("file://{}", path),
            operation: "call".to_string(),
            args,
            headers: HashMap::new(),
            body: Vec::new(),
        };

        let resp = iobus_call(&req)?;

        if resp.status_code != 200 {
            return Err(resp.error.unwrap_or_else(|| "File write failed".to_string()));
        }

        Ok(())
    }

    /// Make an HTTP GET request
    pub fn http_get(&self, url: &str) -> Result<String, String> {
        let req = IOBusRequest {
            uri: url.to_string(),
            operation: "call".to_string(),
            args: HashMap::new(),
            headers: HashMap::new(),
            body: Vec::new(),
        };

        let resp = iobus_call(&req)?;

        if resp.status_code < 200 || resp.status_code >= 300 {
            return Err(resp.error.unwrap_or_else(|| format!("HTTP error: {}", resp.status_code)));
        }

        String::from_utf8(resp.body)
            .map_err(|e| format!("Invalid UTF-8: {}", e))
    }

    /// Make an HTTP POST request
    pub fn http_post(&self, url: &str, body: &str) -> Result<String, String> {
        let req = IOBusRequest {
            uri: url.to_string(),
            operation: "call".to_string(),
            args: HashMap::new(),
            headers: HashMap::new(),
            body: body.as_bytes().to_vec(),
        };

        let resp = iobus_call(&req)?;

        if resp.status_code < 200 || resp.status_code >= 300 {
            return Err(resp.error.unwrap_or_else(|| format!("HTTP error: {}", resp.status_code)));
        }

        String::from_utf8(resp.body)
            .map_err(|e| format!("Invalid UTF-8: {}", e))
    }

    /// Execute a shell command
    pub fn shell_exec(&self, command: &str) -> Result<String, String> {
        let mut args = HashMap::new();
        args.insert("command".to_string(), serde_json::json!(command));

        let req = IOBusRequest {
            uri: "shell://exec".to_string(),
            operation: "call".to_string(),
            args,
            headers: HashMap::new(),
            body: Vec::new(),
        };

        let resp = iobus_call(&req)?;

        if resp.status_code != 200 {
            return Err(resp.error.unwrap_or_else(|| "Shell command failed".to_string()));
        }

        String::from_utf8(resp.body)
            .map_err(|e| format!("Invalid UTF-8: {}", e))
    }
}
```

### Testing

Create a test app that uses these methods:

```rust
// apps/file-reader/src/lib.rs
use serde_json::{json, Value};
use wazeos_app::{AppContext, AppResult, register_tool};

#[no_mangle]
pub extern "C" fn tool_main(ctx: &AppContext, args: Value) -> AppResult {
    let path = args["path"].as_str().unwrap_or("/tmp/test.txt");

    // Read file using driver
    match ctx.read_file(path) {
        Ok(content) => Ok(json!({
            "path": path,
            "content": content,
            "lines": content.lines().count()
        })),
        Err(e) => Err(format!("Failed to read file: {}", e))
    }
}

register_tool!(tool_main);
```

---

## 2. App Packaging System (Medium Priority)

### Problem
No way to share apps between users. Need a packaging format and commands.

### Solution: Implement `.wazpkg` Package Format

#### Package Structure

A `.wazpkg` file is a gzipped tar archive containing:
```
app-name.wazpkg
├── wazeos.toml          # Manifest
├── app.wasm             # Compiled binary
├── README.md            # Optional documentation
└── .wazpkg.json         # Package metadata
```

#### Package Metadata Format

**File:** `.wazpkg.json`
```json
{
  "format_version": "1.0",
  "package_name": "json-formatter",
  "package_version": "0.1.0",
  "created_at": "2026-05-08T13:00:00Z",
  "created_by": "username",
  "wazeos_version": "2.0.0",
  "checksums": {
    "wazeos.toml": "sha256:...",
    "app.wasm": "sha256:..."
  },
  "dependencies": {
    "drivers": [],
    "apps": []
  }
}
```

#### Implementation: `wazeos app package`

**File:** `cmd/wazeos/app_package.go` (NEW)

```go
package main

import (
    "archive/tar"
    "compress/gzip"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "time"

    "github.com/spf13/cobra"
    "github.com/wazeos/wazeos/v2/internal/pkg"
)

var appPackageCmd = &cobra.Command{
    Use:   "package <app-name>",
    Short: "Package an app for distribution",
    Long: `Creates a .wazpkg package file that can be shared and installed by other users.

The package includes:
  - WASM binary
  - wazeos.toml manifest
  - Package metadata with checksums
  - Optional README

Example:
  wazeos app package my-tool
  # Creates: my-tool-v0.1.0.wazpkg
`,
    Args: cobra.ExactArgs(1),
    RunE: runAppPackage,
}

var (
    packageOutput string
    packageReadme string
)

func init() {
    appCmd.AddCommand(appPackageCmd)

    appPackageCmd.Flags().StringVarP(&packageOutput, "output", "o", "", "Output file path (default: <name>-v<version>.wazpkg)")
    appPackageCmd.Flags().StringVar(&packageReadme, "readme", "", "Include README file")
}

type PackageMetadata struct {
    FormatVersion  string            `json:"format_version"`
    PackageName    string            `json:"package_name"`
    PackageVersion string            `json:"package_version"`
    CreatedAt      string            `json:"created_at"`
    CreatedBy      string            `json:"created_by"`
    WazeOSVersion  string            `json:"wazeos_version"`
    Checksums      map[string]string `json:"checksums"`
    Dependencies   PackageDeps       `json:"dependencies"`
}

type PackageDeps struct {
    Drivers []string `json:"drivers"`
    Apps    []string `json:"apps"`
}

func runAppPackage(cmd *cobra.Command, args []string) error {
    appName := args[0]
    appPath := filepath.Join("apps", appName)

    // Load manifest
    manifestPath := filepath.Join(appPath, "wazeos.toml")
    manifest, err := pkg.LoadManifest(manifestPath)
    if err != nil {
        return fmt.Errorf("failed to load manifest: %w", err)
    }

    if !manifest.IsApp() {
        return fmt.Errorf("not an app package")
    }

    // Find WASM binary
    cargoName := strings.ReplaceAll(manifest.Package.Name, "-", "_")
    wasmPath := filepath.Join(appPath, "target/wasm32-wasip1/release",
        fmt.Sprintf("%s.wasm", cargoName))

    if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
        return fmt.Errorf("WASM binary not found: %s\nBuild it first with: wazeos app build %s",
            wasmPath, appName)
    }

    // Determine output path
    outputPath := packageOutput
    if outputPath == "" {
        outputPath = fmt.Sprintf("%s-v%s.wazpkg",
            manifest.Package.Name, manifest.Package.Version)
    }

    logInfo("Creating package: %s", outputPath)

    // Create package
    if err := createPackage(outputPath, manifest, manifestPath, wasmPath, packageReadme); err != nil {
        return fmt.Errorf("failed to create package: %w", err)
    }

    // Get package size
    stat, _ := os.Stat(outputPath)
    sizeMB := float64(stat.Size()) / 1024 / 1024

    logSuccess("✓", "Package created successfully")
    logInfo("")
    logInfo("Package: %s", outputPath)
    logInfo("Size: %.2f MB", sizeMB)
    logInfo("Name: %s", manifest.Package.Name)
    logInfo("Version: %s", manifest.Package.Version)
    logInfo("")
    logInfo("Share this file with other users to install:")
    logInfo("  wazeos app install %s", outputPath)

    return nil
}

func createPackage(outputPath string, manifest *pkg.Manifest, manifestPath, wasmPath, readmePath string) error {
    // Create output file
    outFile, err := os.Create(outputPath)
    if err != nil {
        return err
    }
    defer outFile.Close()

    // Create gzip writer
    gzWriter := gzip.NewWriter(outFile)
    defer gzWriter.Close()

    // Create tar writer
    tarWriter := tar.NewWriter(gzWriter)
    defer tarWriter.Close()

    checksums := make(map[string]string)

    // Add wazeos.toml
    if err := addFileToTar(tarWriter, manifestPath, "wazeos.toml", checksums); err != nil {
        return err
    }

    // Add WASM binary
    if err := addFileToTar(tarWriter, wasmPath, "app.wasm", checksums); err != nil {
        return err
    }

    // Add README if provided
    if readmePath != "" {
        if err := addFileToTar(tarWriter, readmePath, "README.md", checksums); err != nil {
            return err
        }
    }

    // Create package metadata
    metadata := PackageMetadata{
        FormatVersion:  "1.0",
        PackageName:    manifest.Package.Name,
        PackageVersion: manifest.Package.Version,
        CreatedAt:      time.Now().UTC().Format(time.RFC3339),
        CreatedBy:      os.Getenv("USER"),
        WazeOSVersion:  "2.0.0",
        Checksums:      checksums,
        Dependencies: PackageDeps{
            Drivers: []string{},
            Apps:    []string{},
        },
    }

    // Add metadata
    metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
    if err != nil {
        return err
    }

    header := &tar.Header{
        Name: ".wazpkg.json",
        Mode: 0644,
        Size: int64(len(metadataJSON)),
    }

    if err := tarWriter.WriteHeader(header); err != nil {
        return err
    }

    if _, err := tarWriter.Write(metadataJSON); err != nil {
        return err
    }

    return nil
}

func addFileToTar(tarWriter *tar.Writer, srcPath, dstName string, checksums map[string]string) error {
    // Read file
    data, err := os.ReadFile(srcPath)
    if err != nil {
        return err
    }

    // Calculate checksum
    hash := sha256.Sum256(data)
    checksums[dstName] = "sha256:" + hex.EncodeToString(hash[:])

    // Create tar header
    header := &tar.Header{
        Name: dstName,
        Mode: 0644,
        Size: int64(len(data)),
    }

    if err := tarWriter.WriteHeader(header); err != nil {
        return err
    }

    if _, err := tarWriter.Write(data); err != nil {
        return err
    }

    return nil
}
```

#### Implementation: Update `wazeos app install` to Support Packages

**File:** `cmd/wazeos/app.go` (UPDATE)

Add to `runAppInstall`:
```go
func runAppInstall(cmd *cobra.Command, args []string) error {
    pathOrName := args[0]

    // Check if it's a .wazpkg file
    if strings.HasSuffix(pathOrName, ".wazpkg") {
        return installFromPackage(pathOrName)
    }

    // Otherwise, treat as local app directory (existing logic)
    // ... existing code ...
}

func installFromPackage(packagePath string) error {
    logInfo("Installing from package: %s", packagePath)

    // Extract to temporary directory
    tempDir, err := os.MkdirTemp("", "wazeos-install-*")
    if err != nil {
        return fmt.Errorf("failed to create temp dir: %w", err)
    }
    defer os.RemoveAll(tempDir)

    // Extract package
    if err := extractPackage(packagePath, tempDir); err != nil {
        return fmt.Errorf("failed to extract package: %w", err)
    }

    // Verify checksums
    if err := verifyPackageChecksums(tempDir); err != nil {
        return fmt.Errorf("checksum verification failed: %w", err)
    }

    // Load manifest
    manifestPath := filepath.Join(tempDir, "wazeos.toml")
    manifest, err := pkg.LoadManifest(manifestPath)
    if err != nil {
        return fmt.Errorf("failed to load manifest: %w", err)
    }

    // Create installation directory
    installDir := filepath.Join(os.Getenv("HOME"), ".wazeos", "apps", manifest.Package.Name)
    if err := os.MkdirAll(installDir, 0755); err != nil {
        return fmt.Errorf("failed to create install dir: %w", err)
    }

    // Copy files
    wasmSrc := filepath.Join(tempDir, "app.wasm")
    wasmDst := filepath.Join(installDir, manifest.Package.Name+".wasm")
    if err := copyFile(wasmSrc, wasmDst); err != nil {
        return fmt.Errorf("failed to copy WASM binary: %w", err)
    }

    manifestDst := filepath.Join(installDir, "wazeos.toml")
    if err := copyFile(manifestPath, manifestDst); err != nil {
        return fmt.Errorf("failed to copy manifest: %w", err)
    }

    // Copy README if present
    readmeSrc := filepath.Join(tempDir, "README.md")
    if _, err := os.Stat(readmeSrc); err == nil {
        readmeDst := filepath.Join(installDir, "README.md")
        copyFile(readmeSrc, readmeDst)
    }

    logSuccess("✓", fmt.Sprintf("Installed %s v%s",
        manifest.Package.Name, manifest.Package.Version))

    return nil
}

func extractPackage(packagePath, destDir string) error {
    // Open package file
    file, err := os.Open(packagePath)
    if err != nil {
        return err
    }
    defer file.Close()

    // Create gzip reader
    gzReader, err := gzip.NewReader(file)
    if err != nil {
        return err
    }
    defer gzReader.Close()

    // Create tar reader
    tarReader := tar.NewReader(gzReader)

    // Extract files
    for {
        header, err := tarReader.Next()
        if err == io.EOF {
            break
        }
        if err != nil {
            return err
        }

        targetPath := filepath.Join(destDir, header.Name)

        switch header.Typeflag {
        case tar.TypeReg:
            outFile, err := os.Create(targetPath)
            if err != nil {
                return err
            }
            if _, err := io.Copy(outFile, tarReader); err != nil {
                outFile.Close()
                return err
            }
            outFile.Close()
        }
    }

    return nil
}

func verifyPackageChecksums(dir string) error {
    // Read metadata
    metadataPath := filepath.Join(dir, ".wazpkg.json")
    data, err := os.ReadFile(metadataPath)
    if err != nil {
        return err
    }

    var metadata PackageMetadata
    if err := json.Unmarshal(data, &metadata); err != nil {
        return err
    }

    // Verify each checksum
    for filename, expectedChecksum := range metadata.Checksums {
        filePath := filepath.Join(dir, filename)
        fileData, err := os.ReadFile(filePath)
        if err != nil {
            return fmt.Errorf("file not found: %s", filename)
        }

        hash := sha256.Sum256(fileData)
        actualChecksum := "sha256:" + hex.EncodeToString(hash[:])

        if actualChecksum != expectedChecksum {
            return fmt.Errorf("checksum mismatch for %s", filename)
        }
    }

    return nil
}

func copyFile(src, dst string) error {
    data, err := os.ReadFile(src)
    if err != nil {
        return err
    }
    return os.WriteFile(dst, data, 0644)
}
```

---

## 3. Summary

### What Exists Today
✅ MCP server exposing tools to Claude Desktop
✅ WASM app execution with sandboxing
✅ Permission system
✅ WASM drivers (file, HTTP, shell)
✅ App CLI (new, build, install, list)
✅ MCP install helper

### What Needs Implementation

1. **AppContext Methods** (Critical)
   - Add `host_iobus_call` import to app SDK
   - Implement `read_file()`, `write_file()`, `http_get()`, etc.
   - Test with real WASM drivers

2. **Package System** (Important)
   - `wazeos app package` - Create .wazpkg files
   - Update `wazeos app install` - Support .wazpkg files
   - Package verification (checksums)

3. **Future Enhancements** (Nice to Have)
   - Registry system for sharing packages
   - `wazeos app search` - Find apps in registry
   - `wazeos app publish` - Upload to registry
   - Dependency resolution
   - Driver marketplace

### Sharing Workflow (After Implementation)

**Developer:**
```bash
# Create app
wazeos app new awesome-tool
cd apps/awesome-tool
# ... write code ...
cargo build --target wasm32-wasip1 --release

# Package it
wazeos app package awesome-tool
# Creates: awesome-tool-v0.1.0.wazpkg

# Share the .wazpkg file
```

**User:**
```bash
# Install from package
wazeos app install awesome-tool-v0.1.0.wazpkg

# Configure Claude Desktop (if not already done)
wazeos mcp install

# Restart Claude Desktop - tool is now available!
```

### Migration Impact

The AppContext implementation is backward compatible - existing apps will continue to work, but will gain new capabilities once the methods are implemented.
