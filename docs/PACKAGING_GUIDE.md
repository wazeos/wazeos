# WazeOS App Packaging & Distribution Guide

This guide shows you how to package your WazeOS apps for easy distribution and installation.

## Quick Start

```bash
# 1. Build your app
cd apps/my-tool
wazeos app build my-tool

# 2. Package it
wazeos app package my-tool

# 3. Share the .wazpkg file
# Output: my-tool-1.0.0.wazpkg

# 4. Others can install it
wazeos app install my-tool-1.0.0.wazpkg
```

## Why Package?

Packaging your app into a `.wazpkg` file provides:

✅ **Easy Distribution** - Single file containing everything needed
✅ **Version Management** - Package includes version metadata
✅ **Portability** - Works on any system with WazeOS installed
✅ **No Build Required** - Recipients don't need Rust/build tools
✅ **Integrity** - Package format ensures completeness

## Packaging Your App

### Prerequisites

Before packaging, ensure:
1. Your app is built: `wazeos app build my-tool`
2. The WASM binary exists in `target/wasm32-wasip1/release/`
3. Your `wazeos.toml` manifest is complete and valid

### Create a Package

From your project root:

```bash
wazeos app package my-tool
```

This creates: `my-tool-<version>.wazpkg` in the current directory.

### Custom Output Directory

```bash
wazeos app package my-tool --output ./dist/
```

Creates the package in `./dist/my-tool-<version>.wazpkg`.

### Package Contents

A `.wazpkg` file is a compressed tar archive (tar.gz) containing:

```
my-tool-1.0.0.wazpkg/
├── wazeos.toml       # App manifest
├── my-tool.wasm      # WASM binary
└── package.json      # Package metadata
```

**wazeos.toml**: Your app's manifest with permissions, tool definition, etc.

**my-tool.wasm**: The compiled WASM binary.

**package.json**: Metadata including:
- Package format version
- SDK version used
- Creation timestamp
- Build information

### View Package Contents

```bash
# List files in package
tar -tzf my-tool-1.0.0.wazpkg

# Extract package (for inspection)
tar -xzf my-tool-1.0.0.wazpkg

# View metadata
tar -xzOf my-tool-1.0.0.wazpkg package.json | jq .
```

## Distributing Your Package

### Local Distribution

Share the `.wazpkg` file directly:

```bash
# Via file sharing
cp my-tool-1.0.0.wazpkg /shared/folder/

# Via email/Slack/etc.
# Attach my-tool-1.0.0.wazpkg
```

Recipients install with:
```bash
wazeos app install path/to/my-tool-1.0.0.wazpkg
```

### Web Distribution

Host the package file on a web server:

```bash
# Upload to your server
scp my-tool-1.0.0.wazpkg user@example.com:/var/www/tools/

# Share URL with users
https://example.com/tools/my-tool-1.0.0.wazpkg
```

Recipients install with:
```bash
# Download first, then install
curl -LO https://example.com/tools/my-tool-1.0.0.wazpkg
wazeos app install my-tool-1.0.0.wazpkg
```

### GitHub Releases

1. Create a GitHub release
2. Attach your `.wazpkg` file as a release asset
3. Users can download from the releases page

```bash
# Example: Install from GitHub release
curl -LO https://github.com/user/repo/releases/download/v1.0.0/my-tool-1.0.0.wazpkg
wazeos app install my-tool-1.0.0.wazpkg
```

## Installing Packages

### From Local File

```bash
wazeos app install ./my-tool-1.0.0.wazpkg
```

### From Downloaded Package

```bash
# Download
curl -LO https://example.com/tools/my-tool-1.0.0.wazpkg

# Install
wazeos app install my-tool-1.0.0.wazpkg
```

### Installation Process

When you install a package, WazeOS:
1. ✅ Validates the package format
2. ✅ Extracts the contents
3. ✅ Verifies the manifest is valid
4. ✅ Checks the WASM binary exists
5. ✅ Installs to `~/.wazeos/apps/`
6. ✅ Makes the tool available to MCP clients

## Package Validation

WazeOS validates packages during installation:

### Valid Package
```bash
$ wazeos app install my-tool-1.0.0.wazpkg
✓ Installed my-tool v1.0.0 from package
Tool name: my-tool
Location: /Users/you/.wazeos/apps/my-tool
```

### Invalid Package
```bash
$ wazeos app install broken.wazpkg
✗ Error: INVALID_PACKAGE
  Package missing WASM binary: my-tool.wasm

  Package may be corrupted
```

## Best Practices

### Versioning

Update your `wazeos.toml` version before packaging:

```toml
[package]
name = 'my-tool'
version = '1.2.0'  # Increment for each release
```

The package filename includes the version: `my-tool-1.2.0.wazpkg`

### Permissions

Clearly document required permissions in your README:

```markdown
## Permissions

This tool requires:
- File: read/write to `/tmp/**`
- Shell: execute `date`, `echo` commands
- HTTP: access to `api.example.com`
```

### Testing Before Distribution

Always test your package before distributing:

```bash
# Create package
wazeos app package my-tool

# Test in clean environment
rm -rf ~/.wazeos/apps/my-tool
wazeos app install my-tool-1.0.0.wazpkg

# Verify it works
# Test via Claude Desktop or MCP client
```

### Release Checklist

Before releasing a package:

- [ ] Updated version in `wazeos.toml`
- [ ] Tested all functionality works
- [ ] Updated README with usage instructions
- [ ] Documented required permissions
- [ ] Built with release profile (`--release`)
- [ ] Created package file
- [ ] Tested package installation in clean environment
- [ ] Created release notes

## Example: Complete Release Workflow

```bash
# 1. Update version
vim apps/my-tool/wazeos.toml  # version = '1.1.0'

# 2. Build
wazeos app build my-tool

# 3. Test locally
wazeos app test my-tool

# 4. Create package
wazeos app package my-tool --output ./release/

# 5. Test package installation
rm -rf ~/.wazeos/apps/my-tool
wazeos app install ./release/my-tool-1.1.0.wazpkg

# 6. Verify it works
# Test with Claude Desktop

# 7. Distribute
# Upload to GitHub releases, website, etc.
```

## Advanced: Package Registry (Future)

WazeOS will support a central package registry in the future:

```bash
# Publish to registry
wazeos app publish

# Install from registry
wazeos app install my-tool

# Search registry
wazeos app search "file tools"
```

Stay tuned for registry support!

## Troubleshooting

### Package Creation Fails

**Error**: `WASM_NOT_FOUND`

**Solution**: Build your app first:
```bash
wazeos app build my-tool
```

**Error**: `INVALID_MANIFEST`

**Solution**: Check your `wazeos.toml` for syntax errors:
```bash
# Validate manifest
cat apps/my-tool/wazeos.toml
```

### Package Installation Fails

**Error**: `INVALID_PACKAGE - Package missing WASM binary`

**Cause**: Package is corrupted or incomplete

**Solution**: Re-download or re-create the package

**Error**: `NOT_AN_APP - this package is a driver`

**Solution**: Use `wazeos driver install` instead

## Need Help?

- Read the [Package Format Specification](PACKAGE_FORMAT.md)
- Check [GitHub Issues](https://github.com/wazeos/wazeos/issues)
- Join the community discussions

Happy packaging! 📦
