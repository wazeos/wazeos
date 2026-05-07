package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Handler interfaces for different app patterns.

// CLIHandler handles command-line style apps with arguments and stdout output.
type CLIHandler interface {
	// Run processes command-line arguments and returns a response.
	Run(ctx *Context, args []string) (*Response, error)
}

// RequestHandler handles HTTP-style request/response apps.
type RequestHandler interface {
	// Handle processes a request and returns a response.
	Handle(ctx *Context, req *Request) (*Response, error)
}

// StreamHandler handles line-by-line stream processing.
type StreamHandler interface {
	// ProcessLine handles a single line of input from stdin.
	ProcessLine(ctx *Context, line []byte) error

	// Finalize is called after all lines have been processed.
	Finalize(ctx *Context) (*Response, error)
}

// MessageHandler handles messages from queues or topics.
type MessageHandler interface {
	// HandleMessage processes a single message.
	HandleMessage(ctx *Context, msg *Message) error
}

// MCPToolHandler handles MCP tool invocations with JSON input/output.
// This is the simplest handler for building MCP tools.
type MCPToolHandler interface {
	// Handle processes arbitrary JSON input and returns a response.
	// The input map contains the parsed JSON arguments from the MCP call.
	// Return a map that will be serialized to JSON for the response.
	Handle(ctx *Context, input map[string]interface{}) (map[string]interface{}, error)
}

// Request represents an HTTP-style request.
type Request struct {
	Method  string
	Path    string
	Headers map[string]string
	Body    []byte
}

// Entry point functions that execute handlers.

// RunCLI executes a CLI handler with command-line arguments.
func RunCLI(handler CLIHandler) {
	ctx := buildContext()
	args := os.Args[1:]

	response, err := handler.Run(ctx, args)
	if err != nil {
		handleError(ctx, err)
		os.Exit(1)
	}

	writeResponse(response)
	os.Exit(response.ExitCode)
}

// RunMCPTool executes an MCP tool handler with JSON input from stdin.
// This is the recommended way to build MCP tools - simple JSON in, JSON out.
func RunMCPTool(handler MCPToolHandler) {
	ctx := buildContext()

	// Read JSON input from stdin
	var input map[string]interface{}
	decoder := json.NewDecoder(os.Stdin)
	if err := decoder.Decode(&input); err != nil {
		// Empty input is okay for tools that don't require parameters
		if err != io.EOF {
			handleError(ctx, WrapError(err, "INVALID_INPUT", "failed to parse JSON input", 400))
			os.Exit(1)
		}
		input = make(map[string]interface{})
	}

	// Call handler
	output, err := handler.Handle(ctx, input)
	if err != nil {
		handleError(ctx, err)
		os.Exit(1)
	}

	// Write JSON output to stdout
	if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
		handleError(ctx, WrapError(err, "ENCODING_ERROR", "failed to encode response", 500))
		os.Exit(1)
	}
}

// RunHandler executes a request handler with input from stdin.
func RunHandler(handler RequestHandler) {
	ctx := buildContext()

	// Read request from stdin as JSON
	var req Request
	decoder := json.NewDecoder(os.Stdin)
	if err := decoder.Decode(&req); err != nil {
		handleError(ctx, WrapError(err, "INVALID_REQUEST", "failed to parse request", 400))
		os.Exit(1)
	}

	response, err := handler.Handle(ctx, &req)
	if err != nil {
		handleError(ctx, err)
		os.Exit(1)
	}

	writeResponse(response)
	os.Exit(response.ExitCode)
}

// RunStream executes a stream handler that processes stdin line by line.
func RunStream(handler StreamHandler) {
	ctx := buildContext()
	scanner := bufio.NewScanner(os.Stdin)

	// Process each line
	for scanner.Scan() {
		if err := handler.ProcessLine(ctx, scanner.Bytes()); err != nil {
			handleError(ctx, err)
			os.Exit(1)
		}
	}

	if err := scanner.Err(); err != nil {
		handleError(ctx, WrapError(err, "STREAM_ERROR", "failed to read stdin", 500))
		os.Exit(1)
	}

	// Finalize and get response
	response, err := handler.Finalize(ctx)
	if err != nil {
		handleError(ctx, err)
		os.Exit(1)
	}

	writeResponse(response)
	os.Exit(response.ExitCode)
}

// RunConsumer executes a message handler that consumes messages from a topic.
func RunConsumer(topic string, handler MessageHandler, opts *ConsumeOptions) {
	ctx := buildContext()

	// Consume messages from topic
	if opts == nil {
		opts = &ConsumeOptions{MaxCount: 10, Timeout: 5}
	}

	// Use unified I/O API to consume messages
	result, err := ctx.IO(fmt.Sprintf("queue://%s", topic), []string{"consume"}).Call(map[string]interface{}{
		"maxCount": opts.MaxCount,
		"timeout":  opts.Timeout,
		"group":    opts.Group,
	})
	if err != nil {
		handleError(ctx, err)
		os.Exit(1)
	}

	// Parse messages from result
	var messages []*Message
	if messagesData, ok := result["messages"]; ok {
		if messagesJSON, err := json.Marshal(messagesData); err == nil {
			json.Unmarshal(messagesJSON, &messages)
		}
	}

	// Process each message
	successCount := 0
	errorCount := 0
	for _, msg := range messages {
		if err := handler.HandleMessage(ctx, msg); err != nil {
			ctx.Log.Error("message processing failed",
				String("messageId", msg.ID),
				ErrorField(err))
			errorCount++
		} else {
			successCount++
		}
	}

	ctx.Log.Info("message processing complete",
		Int("success", successCount),
		Int("errors", errorCount),
		Int("total", len(messages)))

	if errorCount > 0 {
		os.Exit(1)
	}
	os.Exit(0)
}

// Helper functions for error handling and output.

// handleError logs an error and writes an error response to stdout.
func handleError(ctx *Context, err error) {
	// Log the error
	ctx.Log.Error("handler error", ErrorField(err))

	// Determine status code from error type
	statusCode := 500
	exitCode := 1
	message := err.Error()

	if appErr, ok := err.(*AppError); ok {
		statusCode = appErr.Status
		message = appErr.Message
		if statusCode >= 500 {
			exitCode = 2
		}
	}

	// Write error response
	response := Error(statusCode, message)
	response.ExitCode = exitCode
	writeResponse(response)
}

// writeResponse writes a response to stdout.
func writeResponse(response *Response) {
	if response == nil {
		return
	}

	// Write body to stdout
	if len(response.Body) > 0 {
		fmt.Print(string(response.Body))
	}
}
