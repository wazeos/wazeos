package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	jsonOutput bool
	verbose    bool
)

var rootCmd = &cobra.Command{
	Use:   "wazeos",
	Short: "WazeOS v2 - Capability-based resource abstraction layer",
	Long: `WazeOS v2 is a capability-based resource abstraction layer for AI agents,
automation tools, and secure multi-tenant applications.

Features:
  - Handle-based sessions (load once, reference many times)
  - Binary streaming (no buffer limits)
  - Hierarchical driver model (io.connect, io.listen, runtime.*, kernel.*)
  - Agent-native CLI with JSON output mode

For more information, visit: https://github.com/wazeos/wazeos`,
	Version: "2.0.0-alpha",
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	// Add subcommands
	rootCmd.AddCommand(driverCmd)
	rootCmd.AddCommand(appCmd)
	rootCmd.AddCommand(invokeCmd)
	rootCmd.AddCommand(devCmd)
	rootCmd.AddCommand(fileCmd)
}

// ============================================================================
// JSON Output Helpers
// ============================================================================

type JSONResult struct {
	Status   string      `json:"status"` // "success" or "error"
	Command  string      `json:"command"`
	Result   interface{} `json:"result,omitempty"`
	Errors   []JSONError `json:"errors,omitempty"`
	Duration int64       `json:"duration_ms,omitempty"`
}

type JSONError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

func outputJSON(result JSONResult) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.Encode(result)
}

func outputSuccess(command string, result interface{}) {
	if jsonOutput {
		outputJSON(JSONResult{
			Status:  "success",
			Command: command,
			Result:  result,
		})
	} else {
		// Human-friendly output handled by caller
	}
}

func outputError(command, code, message, suggestion string) {
	if jsonOutput {
		outputJSON(JSONResult{
			Status:  "error",
			Command: command,
			Errors: []JSONError{
				{
					Code:       code,
					Message:    message,
					Suggestion: suggestion,
				},
			},
		})
	} else {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", message)
		if suggestion != "" {
			fmt.Fprintf(os.Stderr, "\n%s\n", suggestion)
		}
	}
	os.Exit(1)
}

func logInfo(format string, args ...interface{}) {
	if !jsonOutput {
		fmt.Printf(format+"\n", args...)
	}
}

func logVerbose(format string, args ...interface{}) {
	if verbose && !jsonOutput {
		fmt.Printf("[VERBOSE] "+format+"\n", args...)
	}
}

func logSuccess(icon, message string) {
	if !jsonOutput {
		fmt.Printf("%s %s\n", icon, message)
	}
}

func logNextSteps(steps ...string) {
	if !jsonOutput && len(steps) > 0 {
		fmt.Println("\n→ Next steps:")
		for i, step := range steps {
			fmt.Printf("  %d. %s\n", i+1, step)
		}
	}
}
