package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var fileCmd = &cobra.Command{
	Use:   "file",
	Short: "File utilities (for agents)",
	Long:  `Helper commands for file operations in agent workflows.`,
}

var fileExistsCmd = &cobra.Command{
	Use:   "exists <path>",
	Short: "Check if file exists",
	Long:  `Returns file existence status.`,
	Args:  cobra.ExactArgs(1),
	Run:   runFileExists,
}

var fileReadCmd = &cobra.Command{
	Use:   "read <path>",
	Short: "Read file contents",
	Long:  `Output file contents.`,
	Args:  cobra.ExactArgs(1),
	Run:   runFileRead,
}

var fileWriteCmd = &cobra.Command{
	Use:   "write <path> <content>",
	Short: "Write file",
	Long:  `Write content to file.`,
	Args:  cobra.ExactArgs(2),
	Run:   runFileWrite,
}

func init() {
	fileCmd.AddCommand(fileExistsCmd)
	fileCmd.AddCommand(fileReadCmd)
	fileCmd.AddCommand(fileWriteCmd)
}

func runFileExists(cmd *cobra.Command, args []string) {
	path := args[0]

	stat, err := os.Stat(path)
	exists := err == nil

	var size int64
	if exists {
		size = stat.Size()
	}

	if jsonOutput {
		outputSuccess("file exists", map[string]interface{}{
			"exists":     exists,
			"size_bytes": size,
		})
	} else {
		if exists {
			fmt.Printf("File exists: %s (%d bytes)\n", path, size)
		} else {
			fmt.Printf("File does not exist: %s\n", path)
		}
	}
}

func runFileRead(cmd *cobra.Command, args []string) {
	path := args[0]

	data, err := os.ReadFile(path)
	if err != nil {
		outputError("file read", "READ_FAILED", err.Error(), "")
	}

	if jsonOutput {
		outputSuccess("file read", map[string]interface{}{
			"content":    string(data),
			"size_bytes": len(data),
		})
	} else {
		fmt.Print(string(data))
	}
}

func runFileWrite(cmd *cobra.Command, args []string) {
	path := args[0]
	content := args[1]

	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		outputError("file write", "WRITE_FAILED", err.Error(), "")
	}

	if jsonOutput {
		outputSuccess("file write", map[string]interface{}{
			"success":    true,
			"size_bytes": len(content),
		})
	} else {
		logSuccess("✓", fmt.Sprintf("Wrote %d bytes to %s", len(content), path))
	}
}
