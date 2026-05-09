# Using WazeOS with Claude Desktop

This guide shows you how to expose your WazeOS apps as MCP tools in Claude Desktop.

## Overview

WazeOS includes an MCP (Model Context Protocol) server that exposes all installed apps as tools that Claude Desktop can discover and use. This allows you to create custom WASM-based tools that Claude can invoke during conversations.

## Setup

### Quick Setup (Recommended)

The easiest way to configure Claude Desktop:

```bash
# Install WazeOS into Claude Desktop configuration
wazeos mcp install

# Restart Claude Desktop
```

That's it! WazeOS is now ready to use.

### Manual Setup

If you prefer to configure manually:

#### 1. Install WazeOS Apps

First, create and install your WazeOS apps:

```bash
# Create a new app
wazeos app new my-tool

# Build it
cd apps/my-tool
cargo build --target wasm32-wasip1 --release

# Install it
cd ../..
wazeos app install my-tool
```

#### 2. Configure Claude Desktop

Add WazeOS to your Claude Desktop MCP configuration:

**Location:** `~/.config/claude/mcp_servers.json` (create if it doesn't exist)

```json
{
  "wazeos": {
    "command": "/path/to/wazeos",
    "args": ["mcp", "server"]
  }
}
```

Replace `/path/to/wazeos` with the full path to your `wazeos` binary.

To find the path:
```bash
which wazeos
# or if running from the project directory:
realpath wazeos
```

#### 3. Restart Claude Desktop

After updating the configuration, restart Claude Desktop. It will automatically connect to the WazeOS MCP server and discover all installed tools.

## How It Works

### MCP Server

When you start Claude Desktop, it launches the WazeOS MCP server which:

1. **Discovers Apps**: Scans `~/.wazeos/apps/` for installed apps
2. **Exposes Tools**: Publishes each app's MCP tool schema to Claude
3. **Executes WASM**: When Claude calls a tool, loads and runs the WASM binary
4. **Returns Results**: Sends the tool output back to Claude

### App Execution Flow

```
Claude Desktop
    ↓ (JSON-RPC request)
WazeOS MCP Server
    ↓ (load WASM)
IO Bus → WASM Runtime
    ↓ (execute)
Your WASM App (wazeos_tool_invoke)
    ↓ (result)
Claude Desktop
```

### Security

Apps run with the permissions declared in their `wazeos.toml` manifest:

```toml
[permissions]
file = ["read"]           # Can read files
http = ["example.com"]    # Can access example.com
shell = ["ls", "cat"]     # Can run ls and cat commands
```

The IO Bus enforces these permissions at runtime.

## Example: Testing the Integration

### 1. Check Server Logs

The MCP server logs to `/tmp/wazeos-mcp.log`:

```bash
tail -f /tmp/wazeos-mcp.log
```

### 2. Manual Testing

You can test the MCP server manually using JSON-RPC:

```bash
# List available tools
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | wazeos mcp server

# Call a tool
echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"test-tool","arguments":{"input":"Hello"}}}' | wazeos mcp server
```

### 3. Using in Claude Desktop

Once configured, simply ask Claude to use your tools:

**You:** "Use the test-tool to say hello to Bob"

**Claude:** *[calls test-tool with input "Bob"]*
```
Tool result:
{
  "message": "Hello, Bob!",
  "status": "success",
  "tool": "test-tool"
}
```

## Creating MCP Tools

### App Structure

Every WazeOS app must include:

1. **wazeos.toml** - Package manifest with tool schema
2. **src/lib.rs** - Rust source with tool logic
3. **Cargo.toml** - Rust build configuration

### Example: JSON Formatter Tool

```bash
wazeos app new json-formatter
cd apps/json-formatter
```

**wazeos.toml:**
```toml
[package]
name = "json-formatter"
version = "0.1.0"
description = "Formats and validates JSON"

[tool]
name = "format-json"
description = "Format and validate JSON strings"

[tool.input_schema]
type = "object"
properties = { json = { type = "string", description = "JSON to format" } }
required = ["json"]

[permissions]
# No external permissions needed
```

**src/lib.rs:**
```rust
use serde_json::{json, Value};
use wazeos_app::{AppContext, AppResult, register_tool};

#[no_mangle]
pub extern "C" fn tool_main(_ctx: &AppContext, args: Value) -> AppResult {
    let json_str = args["json"].as_str().unwrap_or("{}");

    // Parse and re-format
    match serde_json::from_str::<Value>(json_str) {
        Ok(parsed) => {
            let formatted = serde_json::to_string_pretty(&parsed)
                .unwrap_or_else(|_| "{}".to_string());

            Ok(json!({
                "formatted": formatted,
                "valid": true
            }))
        }
        Err(e) => {
            Err(format!("Invalid JSON: {}", e))
        }
    }
}

register_tool!(tool_main);
```

Build and install:
```bash
cargo build --target wasm32-wasip1 --release
cd ../..
wazeos app install json-formatter
```

Restart Claude Desktop, and you can now ask Claude to format JSON!

## Troubleshooting

### Tools Not Appearing

1. Check that apps are installed: `wazeos app list`
2. Verify MCP config path: `~/.config/claude/mcp_servers.json`
3. Check server logs: `/tmp/wazeos-mcp.log`
4. Restart Claude Desktop

### Tool Execution Errors

1. Check app permissions in `wazeos.toml`
2. Verify WASM binary exists in `~/.wazeos/apps/<name>/`
3. Test tool manually: `echo '...' | wazeos mcp-server`

### Permission Denied

Add required permissions to `wazeos.toml`:

```toml
[permissions]
file = ["**"]              # All files
http = ["**"]              # All HTTP
shell = ["**"]             # All commands
env = ["PATH", "HOME"]     # Environment variables
```

## Next Steps

- **Create Useful Tools**: Build tools that enhance Claude's capabilities
- **Share Your Tools**: Package apps with `wazeos app package` (coming soon)
- **Explore Examples**: Check out community tools in the registry

## MCP Install Command

The `wazeos mcp install` command automatically configures Claude Desktop:

### Basic Usage

```bash
# Install with default settings
wazeos mcp install

# Update existing configuration
wazeos mcp install --force

# Use a custom server name
wazeos mcp install --name my-wazeos-tools
```

### What It Does

1. **Locates the wazeos binary** - Finds the full path automatically
2. **Creates config directory** - Makes `~/.config/claude/` if needed
3. **Updates mcp_servers.json** - Adds or updates the WazeOS entry
4. **Preserves other servers** - Keeps existing MCP server configurations
5. **Shows next steps** - Displays helpful instructions

### Flags

- `--name` - Custom name for the MCP server entry (default: "wazeos")
- `--force` - Overwrite existing WazeOS configuration
- `--help` - Show detailed help

### Example Output

```
Found wazeos binary: /usr/local/bin/wazeos
✓ WazeOS installed to Claude Desktop

Configuration written to: /Users/you/.config/claude/mcp_servers.json
Server name: wazeos

📦 3 WazeOS app(s) installed and ready to use

→ Next steps:
  1. Restart Claude Desktop to enable WazeOS tools
  2. Check server logs at: /tmp/wazeos-mcp.log
  3. View installed apps: wazeos app list
```

## Advanced Configuration

### Custom Binary Path

If you've installed WazeOS to a custom location:

```json
{
  "wazeos": {
    "command": "/opt/wazeos/bin/wazeos",
    "args": ["mcp", "server"],
    "env": {
      "WAZEOS_HOME": "/custom/path/.wazeos"
    }
  }
}
```

### Multiple Tool Sets

You can run multiple MCP servers with different app directories:

```json
{
  "wazeos-dev": {
    "command": "/path/to/wazeos",
    "args": ["mcp", "server"],
    "env": {
      "WAZEOS_HOME": "/path/to/dev/apps"
    }
  },
  "wazeos-prod": {
    "command": "/path/to/wazeos",
    "args": ["mcp", "server"],
    "env": {
      "WAZEOS_HOME": "/path/to/prod/apps"
    }
  }
}
```

## Resources

- **MCP Protocol**: https://modelcontextprotocol.io/
- **WazeOS Documentation**: See README.md
- **Example Apps**: Coming soon!
