// Hello World - WazeOS MCP Tool Example
//
// This is a minimal example showing how to create a WazeOS tool in Go.
// It demonstrates basic argument parsing, IO Bus calls, and response handling.
//
// Build:
//   tinygo build -o hello.wasm -target=wasi -no-debug -opt=2 main.go
//
// Install:
//   wazeos app install hello.wasm
package main

import (
	"fmt"
	wazeos "github.com/wazeos/wazeos/core/sdk/go/app"
)

// toolInvoke is the main entry point for the tool
//
//export wazeos_tool_invoke
func toolInvoke(argsPtr, argsLen uint32) uint32 {
	ctx := wazeos.NewContext()

	// Parse arguments
	args, err := wazeos.ParseArgs(argsPtr, argsLen)
	if err != nil {
		return wazeos.ReturnError(fmt.Sprintf("Failed to parse args: %s", err))
	}

	// Get name from arguments (default to "World")
	name := "World"
	if nameArg, ok := args["name"].(string); ok {
		name = nameArg
	}

	// Optional: Demonstrate IO Bus call
	// This tries to read system time via shell command
	headers := map[string]string{
		"command": "date '+%H:%M:%S'",
	}
	timeResp, err := ctx.Call("shell://exec", headers, nil)

	var greeting string
	if err != nil {
		// If shell access denied, just return simple greeting
		greeting = fmt.Sprintf("Hello, %s!", name)
	} else {
		// Include timestamp in greeting
		timestamp := string(timeResp.Body)
		greeting = fmt.Sprintf("Hello, %s! (Time: %s)", name, timestamp)
	}

	// Return success response
	result := map[string]interface{}{
		"greeting": greeting,
		"name":     name,
	}
	return wazeos.ReturnSuccess(result)
}

// toolMetadata returns metadata about this tool
//
//export wazeos_tool_metadata
func toolMetadata() uint32 {
	return wazeos.ReturnMetadata("hello-go", "1.0.0")
}

// main is required but not used in WASM
func main() {}
