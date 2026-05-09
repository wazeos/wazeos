# WazeOS Package Format (.wazpkg)

## Overview

A `.wazpkg` file is a distributable package format for WazeOS apps and drivers. It bundles the WASM binary, manifest, and metadata into a single file that can be easily shared and installed.

## Format Specification

A `.wazpkg` file is a **tar.gz archive** containing:

```
package-name.wazpkg (tar.gz)
├── wazeos.toml          # App/driver manifest (required)
├── package-name.wasm    # WASM binary (required)
└── package.json         # Package metadata (optional)
```

### File: wazeos.toml

The standard WazeOS manifest file defining the app/driver configuration, permissions, and metadata.

Example:
```toml
[package]
name = 'my-tool'
version = '1.0.0'
description = 'A useful tool'

[tool]
name = 'my-tool'
description = 'MCP tool: my-tool'

[permissions]
file = ['/tmp/**']
shell = ['echo', 'date']
```

### File: package-name.wasm

The compiled WASM binary for the app or driver.

### File: package.json (Optional)

Additional metadata about the package build and requirements.

```json
{
  "name": "my-tool",
  "version": "1.0.0",
  "package_format_version": "1.0",
  "created_at": "2026-05-08T14:30:00Z",
  "sdk_version": "0.1.0",
  "driver_dependencies": {
    "file-driver-wasm": ">=0.1.0",
    "shell-driver-wasm": ">=0.1.0"
  },
  "build": {
    "rust_version": "1.77.0",
    "target": "wasm32-wasip1",
    "profile": "release"
  }
}
```

## Package Naming Convention

Package files should follow this naming pattern:
```
<package-name>-<version>.wazpkg
```

Examples:
- `test-tool-0.1.0.wazpkg`
- `file-search-1.2.3.wazpkg`
- `git-helper-2.0.0.wazpkg`

## Creating a Package

```bash
# From the app directory
cd apps/my-tool
wazeos app package

# Creates: my-tool-1.0.0.wazpkg in current directory
```

## Installing from Package

```bash
# Install from local file
wazeos app install ./my-tool-1.0.0.wazpkg

# Install from URL (future)
wazeos app install https://registry.wazeos.dev/packages/my-tool-1.0.0.wazpkg
```

## Package Validation

When installing, WazeOS validates:
1. ✅ Archive can be extracted
2. ✅ `wazeos.toml` exists and is valid
3. ✅ WASM binary exists and matches package name
4. ✅ Package format version is supported
5. ✅ Required drivers are available (future)
6. ✅ Version conflicts with installed packages (future)

## Package Registry (Future)

Packages can be published to a central registry:

```bash
# Publish to registry
wazeos app publish

# Search registry
wazeos app search "file tool"

# Install from registry
wazeos app install file-search
```

## Security Considerations

- Packages should be signed (future: GPG signatures)
- Verify checksums before installation
- Display permissions before confirming install
- Sandboxed installation process
- Verify source authenticity for registry packages

## Implementation Notes

- Use standard Go `archive/tar` and `compress/gzip` libraries
- Store packages in `~/.wazeos/packages/` after installation
- Support both apps and drivers in the same format
- Packages are immutable once created
- Version conflicts handled by semantic versioning
