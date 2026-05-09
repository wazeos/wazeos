package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var invokeCmd = &cobra.Command{
	Use:   "invoke <app>/<tool> [args...]",
	Short: "Invoke an MCP tool",
	Long: `Execute an MCP tool.

Examples:
  wazeos invoke time/get_time
  wazeos invoke random/random_article --json
  wazeos invoke transcribe/audio file=audio.wav model=whisper-tiny`,
	Args: cobra.MinimumNArgs(1),
	Run:  runInvoke,
}

var invokeTimeout int

func init() {
	invokeCmd.Flags().IntVar(&invokeTimeout, "timeout", 30, "Max execution time in seconds")
}

func runInvoke(cmd *cobra.Command, args []string) {
	toolPath := args[0]
	toolArgs := args[1:]

	logVerbose("Invoking tool: %s", toolPath)
	logVerbose("Arguments: %v", toolArgs)

	// TODO: Implement actual tool invocation
	result := map[string]interface{}{
		"message": "Tool invocation not yet implemented",
		"tool":    toolPath,
		"args":    toolArgs,
	}

	if jsonOutput {
		outputSuccess("invoke", result)
	} else {
		fmt.Printf("Result: %v\n", result)
	}
}
