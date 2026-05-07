# Multi-Language Support in WazeOS

This document explains how WazeOS supports multiple programming languages and provides a guide for adding support for new languages.

## Table of Contents

- [Overview](#overview)
- [Currently Supported Languages](#currently-supported-languages)
- [Architecture](#architecture)
- [Adding a New Language](#adding-a-new-language)
- [Language-Specific Components](#language-specific-components)
- [Testing](#testing)
- [Publishing SDKs](#publishing-sdks)

## Overview

WazeOS is designed to be language-agnostic at the runtime level. All applications and drivers compile to WebAssembly (WASM) and communicate with the kernel using a standard JSON-over-stdio protocol. This means any language that can:

1. Compile to WASM with WASI support
2. Read/write JSON from stdin/stdout
3. Implement the required handler interfaces

...can be used to build WazeOS applications and drivers.

## Currently Supported Languages

### Go (TinyGo)

- **Compiler**: [TinyGo](https://tinygo.org/)
- **Target**: `wasm32-wasi`
- **SDK Location**: `/sdk/app/`, `/sdk/driver/`
- **Features**:
  - Automatic JSON schema generation from struct tags
  - Full runtime support with garbage collection
  - Comprehensive SDK with logging, I/O, and context management

### Rust

- **Compiler**: [Cargo](https://doc.rust-lang.org/cargo/) + [rustc](https://www.rust-lang.org/)
- **Target**: `wasm32-wasi`
- **SDK Location**: `/sdk/rust/wazeos-app/`, `/sdk/rust/wazeos-driver/`
- **Features**:
  - Manual JSON schema definition
  - Memory-safe, zero-cost abstractions
  - Excellent performance and small WASM binaries
  - Full type safety and error handling

## Architecture

### The Language-Agnostic Protocol

The WazeOS kernel communicates with WASM modules through a simple protocol:

**For Applications:**
```
Kernel  →  [JSON Input via stdin]  →  App WASM
Kernel  ←  [JSON Output via stdout] ←  App WASM
```

**For Drivers:**
```
Kernel  →  [ResourceCall JSON via stdin]  →  Driver WASM
Kernel  ←  [ResourceResult JSON via stdout] ←  Driver WASM
```

This protocol is completely language-independent. The WASM module just needs to:
1. Read JSON from stdin
2. Parse it into appropriate data structures
3. Execute business logic
4. Serialize results as JSON
5. Write to stdout

### Build System Architecture

The build system detects the language and uses the appropriate compiler:

```
wazeos apps build
    ↓
Language Detection (go.mod, Cargo.toml, main.*, etc.)
    ↓
    ├─→ Go: tinygo build -target=wasi
    └─→ Rust: cargo build --target wasm32-wasi --release
    ↓
Result: app.wasm
```

## Adding a New Language

To add support for a new language (e.g., AssemblyScript, Zig, C), follow these steps:

### Step 1: Verify WASM/WASI Support

Ensure the language can:
- Compile to WASM with WASI support
- Read from stdin (file descriptor 0)
- Write to stdout (file descriptor 1)
- Parse and generate JSON

### Step 2: Create SDK Crates/Packages

Create SDK libraries for the new language in `/sdk/<language>/`:

**For Applications (`/sdk/<language>/wazeos-app`):**

Required types:
```
Context {
    request_id: String
    trace_id: String
    principal: String
    permissions: PermissionContext
    metadata: Map<String, String>
}

Response {
    status_code: Integer
    headers: Map<String, String>
    body: Bytes
    exit_code: Integer
}
```

Required trait/interface:
```
MCPToolHandler {
    handle(ctx: Context, input: JSON) -> Result<JSON>
}
```

Entry point function:
```
run_mcp_tool(handler: MCPToolHandler)
```

**For Drivers (`/sdk/<language>/wazeos-driver`):**

Required types:
```
ResourceCall {
    uri: String
    headers: Map<String, String>
    body: Bytes
    permissions: Array<String>
}

ResourceResult {
    status_code: Integer
    headers: Map<String, String>
    body: Bytes
    error: Optional<String>
}
```

Required trait/interface:
```
ResourceHandler {
    handle_call(call: ResourceCall) -> Result<ResourceResult>
}
```

Entry point function:
```
serve_resource_once(handler: ResourceHandler)
```

### Step 3: Update Language Detection

Add language detection logic to `/cmd/wazeos/commands/common/language.go`:

```go
// DetectLanguage determines the language of a project
func DetectLanguage(dir string) (Language, error) {
    // Check for new language indicators
    if _, err := os.Stat(filepath.Join(dir, "build.zig")); err == nil {
        return LanguageZig, nil
    }

    // ... existing checks ...

    return "", fmt.Errorf("unable to detect language")
}
```

Define the new language constant:
```go
const (
    LanguageGo   Language = "go"
    LanguageRust Language = "rust"
    LanguageZig  Language = "zig"  // New
)
```

### Step 4: Add Build Support

Add build logic to `/cmd/wazeos/commands/common/language.go`:

```go
// BuildWASM compiles source code to WASM based on language
func BuildWASM(lang Language, dir, outputFile string) error {
    switch lang {
    case LanguageGo:
        return buildGoWASM(dir, outputFile)
    case LanguageRust:
        return buildRustWASM(dir, outputFile)
    case LanguageZig:
        return buildZigWASM(dir, outputFile)  // New
    default:
        return fmt.Errorf("unsupported language: %s", lang)
    }
}

func buildZigWASM(dir, outputFile string) error {
    cmd := exec.Command("zig", "build-lib",
        "src/main.zig",
        "-target", "wasm32-wasi",
        "-O", "ReleaseSmall",
        "-femit-bin=" + outputFile)
    cmd.Dir = dir

    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("Zig compilation failed:\n%s", string(output))
    }

    return nil
}
```

### Step 5: Add Toolchain Verification

Add toolchain checking to `/cmd/wazeos/commands/common/language.go`:

```go
// CheckToolchain verifies the required build toolchain is installed
func CheckToolchain(lang Language) error {
    switch lang {
    case LanguageGo:
        // ... existing checks ...
    case LanguageRust:
        // ... existing checks ...
    case LanguageZig:
        cmd := exec.Command("zig", "version")
        if err := cmd.Run(); err != nil {
            return fmt.Errorf("Zig not found. Install it from: https://ziglang.org/")
        }
        // Check WASM target support if needed
        return nil
    default:
        return fmt.Errorf("unsupported language: %s", lang)
    }
}
```

### Step 6: Create Project Templates

Add template generation functions to `/cmd/wazeos/commands/apps/new.go` and `/cmd/wazeos/commands/drivers/new.go`:

**For Apps:**

```go
func generateProjectFiles(projectDir, author, appName, description, language string) error {
    var files map[string]string

    if language == "zig" {
        files = map[string]string{
            "src/main.zig": generateMainZig(),
            "build.zig":    generateBuildZig(author, appName, description),
            "metadata.json": generateMetadata(author, appName, description),
            "README.md":     generateReadmeZig(author, appName, description),
            ".gitignore":    generateGitignoreZig(),
        }
    } else if language == "rust" {
        // ... existing Rust templates ...
    } else {
        // ... existing Go templates ...
    }

    // ... write files ...
}

func generateMainZig() string {
    return `const std = @import("std");

pub fn main() !void {
    const stdin = std.io.getStdIn().reader();
    const stdout = std.io.getStdOut().writer();

    // Read JSON input from stdin
    var buffer: [4096]u8 = undefined;
    const input = try stdin.readUntilDelimiterOrEof(&buffer, '\n');

    // Process input (simplified)
    const response = "{\"status\":\"success\"}";

    // Write JSON output to stdout
    try stdout.print("{s}\n", .{response});
}
`
}
```

**For Drivers:**

```go
func generateDriverMainZig() string {
    return `const std = @import("std");

pub fn main() !void {
    const stdin = std.io.getStdIn().reader();
    const stdout = std.io.getStdOut().writer();

    // Read ResourceCall JSON from stdin
    var buffer: [4096]u8 = undefined;
    const input = try stdin.readUntilDelimiterOrEof(&buffer, '\n');

    // Handle resource call (simplified)
    const response = "{\"statusCode\":200,\"body\":[],\"headers\":{}}";

    // Write ResourceResult JSON to stdout
    try stdout.print("{s}\n", .{response});
}
`
}
```

### Step 7: Update Documentation

1. Update `/README.md` to list the new language
2. Create SDK documentation in `/sdk/<language>/README.md`
3. Add examples to `/examples/<language>/`
4. Update build command help text

### Step 8: Schema Generation (Optional)

For languages with reflection or macros, you can add automatic schema generation:

**For Go:**
- Currently implemented using Go AST parsing
- Extracts struct tags to generate JSON Schema

**For Rust (Future Enhancement):**
- Could use `syn` crate for AST parsing
- Parse derive macros and doc comments
- Generate schema automatically

**For New Language:**
- Implement schema extraction in the build command
- Update `/cmd/wazeos/commands/apps/build.go` to call your schema extractor
- Fall back to manual schema definition if extraction fails

Example:
```go
if lang == common.LanguageZig {
    fmt.Println("\n→ Schema extraction...")
    schema, err := extractSchemaFromZig(mainFile)
    if err != nil {
        fmt.Println("  ℹ Using manual schema definition from metadata.json")
    } else if schema != nil {
        fmt.Printf("  ✓ Extracted schema with %d field(s)\n", len(schema["properties"].(map[string]interface{})))
        if err := updateMetadata(metadataFile, schema); err != nil {
            // Handle error
        }
    }
}
```

## Language-Specific Components

### Required Components

Each language implementation needs:

1. **SDK Package** - Core types and traits/interfaces
2. **Build Integration** - Compiler invocation in build system
3. **Language Detection** - File patterns for auto-detection
4. **Templates** - Starter project generation
5. **Documentation** - SDK usage guide and examples
6. **Tests** - Example projects that build and run

### Optional Components

- **Schema Generation** - Automatic JSON schema extraction from code
- **Package Manager Integration** - Publishing to npm, crates.io, etc.
- **IDE Support** - Language server, syntax highlighting
- **Debugging Tools** - WASM debugging integration

### Component Locations

```
/sdk/<language>/
    wazeos-app/          # App SDK
        src/
        README.md
        Cargo.toml (or equivalent)

    wazeos-driver/       # Driver SDK
        src/
        README.md
        Cargo.toml (or equivalent)

/cmd/wazeos/commands/
    common/
        language.go       # Language detection & build
    apps/
        new.go            # App templates
        build.go          # Build command
    drivers/
        new.go            # Driver templates
        build.go          # Build command

/examples/<language>/
    hello-app/            # Example app
    hello-driver/         # Example driver

/docs/
    LANGUAGE_SUPPORT.md   # This document
    languages/
        <language>.md     # Language-specific guide
```

## Testing

### Manual Testing Checklist

For each new language:

- [ ] Create a new app project: `wazeos apps new test myapp --language <lang>`
- [ ] Build succeeds: `wazeos apps build test/myapp`
- [ ] Package succeeds: `wazeos apps package test/myapp`
- [ ] Install succeeds: `wazeos apps install test/myapp`
- [ ] Create a new driver project: `wazeos drivers new test mydriver --driver-class io.resource --language <lang>`
- [ ] Build succeeds: `wazeos drivers build test/mydriver`
- [ ] Package succeeds: `wazeos drivers package test/mydriver`
- [ ] Test with sample input: `echo '{"test":"data"}' | wasmtime test/myapp/app.wasm`
- [ ] Verify JSON output format is correct
- [ ] Check WASM binary size (should be reasonable)
- [ ] Test with WazeOS runtime

### Integration Testing

Create integration tests in `/internal/drivers/kernel/runtime/`:

```go
func TestZigAppExecution(t *testing.T) {
    // Build test Zig app
    cmd := exec.Command("wazeos", "apps", "build", "testdata/zig-app")
    if err := cmd.Run(); err != nil {
        t.Fatal(err)
    }

    // Load and execute
    ctx := context.Background()
    module, err := LoadWASMModule(ctx, "testdata/zig-app/app.wasm")
    if err != nil {
        t.Fatal(err)
    }

    // Test execution
    input := `{"message":"test"}`
    output, err := module.Execute(ctx, []byte(input))
    if err != nil {
        t.Fatal(err)
    }

    // Verify output
    var result map[string]interface{}
    if err := json.Unmarshal(output, &result); err != nil {
        t.Fatal(err)
    }

    // Assert expected fields
    // ...
}
```

## Publishing SDKs

### Publishing to Package Registries

**For Rust (crates.io):**
```bash
cd sdk/rust/wazeos-app
cargo publish

cd ../wazeos-driver
cargo publish
```

**For npm (JavaScript/AssemblyScript):**
```bash
cd sdk/javascript/wazeos-app
npm publish

cd ../wazeos-driver
npm publish
```

**For other ecosystems:**
- Go: Module path in go.mod (already accessible via GitHub)
- Zig: Zig package manager
- Python: PyPI
- etc.

### Versioning

Follow semantic versioning for SDKs:
- **Major version**: Breaking API changes
- **Minor version**: New features, backward compatible
- **Patch version**: Bug fixes

Keep SDK versions in sync with WazeOS releases when possible.

### Documentation

Each published SDK should include:
- Installation instructions
- Quick start guide
- API reference
- Examples
- Link to main WazeOS documentation

## Best Practices

### For SDK Developers

1. **Keep It Simple**: The SDK should be as thin as possible. Focus on the protocol, not fancy abstractions.
2. **Follow Conventions**: Match the idioms of the target language.
3. **Document Everything**: Provide examples for common use cases.
4. **Test Thoroughly**: Include unit tests and integration tests.
5. **Optimize for Size**: WASM binaries should be as small as possible.

### For Build System Integration

1. **Fail Fast**: Provide clear error messages for missing toolchains.
2. **Detect Automatically**: Use file patterns to auto-detect language.
3. **Support Flags**: Allow `--language` override for explicit selection.
4. **Cache Intelligently**: Don't rebuild if source hasn't changed.

### For Template Generation

1. **Include Comments**: Explain what each part does.
2. **Show Best Practices**: Demonstrate error handling, logging, etc.
3. **Keep It Minimal**: Don't include unnecessary dependencies.
4. **Make It Runnable**: Template should build and run immediately.

## Future Enhancements

### Planned Language Support

- **AssemblyScript**: TypeScript-like syntax, compiles to WASM
- **Zig**: Modern systems language with excellent WASM support
- **C/C++**: Using Emscripten or wasi-sdk
- **Python**: Using Pyodide or similar WASM Python runtime

### Build System Improvements

- **Parallel Builds**: Build multiple modules simultaneously
- **Incremental Compilation**: Only rebuild changed modules
- **Build Caching**: Cache WASM artifacts across builds
- **Cross-Compilation**: Build for different WASM feature sets

### SDK Enhancements

- **I/O Helpers**: High-level APIs for file, HTTP, database operations
- **Testing Utilities**: Mock contexts, test harnesses
- **Debugging Tools**: Better error messages, stack traces
- **Performance Profiling**: Built-in profiling support

## Conclusion

WazeOS's language-agnostic architecture makes it straightforward to add support for new languages. The key is implementing the JSON-over-stdio protocol and providing a good developer experience through SDKs and tooling.

If you're adding support for a new language, feel free to:
1. Follow this guide
2. Open a PR with your implementation
3. Ask questions in GitHub Discussions
4. Share your progress with the community

Happy coding! 🚀
