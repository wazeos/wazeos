package driver

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// ServeResourceStdio runs a resource driver using stdin/stdout for communication.
// This is a safer alternative to pointer-based communication.
//
// Protocol:
// - Host writes JSON-encoded ResourceCall to stdin
// - Driver reads from stdin, processes the call
// - Driver writes JSON-encoded ResourceResult to stdout
// - Host reads from stdout
//
// Usage:
//
//	func main() {
//	    handler := &MyDriver{}
//	    driver.ServeResourceStdio(handler)
//	}
func ServeResourceStdio(handler ResourceHandler) {
	reader := bufio.NewReader(os.Stdin)

	for {
		// Read request from stdin (one JSON object per line)
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			writeError(fmt.Sprintf("failed to read stdin: %v", err))
			continue
		}

		// Parse ResourceCall
		var call ResourceCall
		if err := json.Unmarshal(line, &call); err != nil {
			writeError(fmt.Sprintf("failed to parse call: %v", err))
			continue
		}

		// Handle the call
		result, err := handler.HandleCall(&call)
		if err != nil && result == nil {
			result = NewErrorResult(500, err.Error())
		}

		// Write result to stdout
		resultJSON, err := json.Marshal(result)
		if err != nil {
			writeError(fmt.Sprintf("failed to marshal result: %v", err))
			continue
		}

		fmt.Fprintf(os.Stdout, "%s\n", resultJSON)
		os.Stdout.Sync() // Ensure it's written immediately
	}
}

// ServeAuthStdio runs an auth driver using stdin/stdout for communication.
//
// Protocol:
// - Host writes JSON-encoded AuthPayload to stdin
// - Driver reads from stdin, processes authentication
// - Driver writes JSON-encoded AuthResult to stdout
// - Host reads from stdout
//
// Usage:
//
//	func main() {
//	    handler := &MyAuthDriver{}
//	    driver.ServeAuthStdio(handler)
//	}
func ServeAuthStdio(handler AuthHandler) {
	reader := bufio.NewReader(os.Stdin)

	for {
		// Read request from stdin
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			writeAuthError(fmt.Sprintf("failed to read stdin: %v", err))
			continue
		}

		// Parse AuthPayload
		var payload AuthPayload
		if err := json.Unmarshal(line, &payload); err != nil {
			writeAuthError(fmt.Sprintf("failed to parse payload: %v", err))
			continue
		}

		// Authenticate
		result, err := handler.Authenticate(&payload)
		if err != nil && result == nil {
			result = NewAuthError(err.Error())
		}

		// Write result to stdout
		resultJSON, err := json.Marshal(result)
		if err != nil {
			writeAuthError(fmt.Sprintf("failed to marshal result: %v", err))
			continue
		}

		fmt.Fprintf(os.Stdout, "%s\n", resultJSON)
		os.Stdout.Sync()
	}
}

// ServeResourceOnce handles a single resource call from stdin and exits.
// Useful for request-per-invocation model.
//
// Usage:
//
//	func main() {
//	    handler := &MyDriver{}
//	    driver.ServeResourceOnce(handler)
//	}
func ServeResourceOnce(handler ResourceHandler) {
	// Read request from stdin
	var call ResourceCall
	if err := json.NewDecoder(os.Stdin).Decode(&call); err != nil {
		writeError(fmt.Sprintf("failed to parse call: %v", err))
		os.Exit(1)
	}

	// Handle the call
	result, err := handler.HandleCall(&call)
	if err != nil && result == nil {
		result = NewErrorResult(500, err.Error())
	}

	// Write result to stdout
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		writeError(fmt.Sprintf("failed to write result: %v", err))
		os.Exit(1)
	}
}

// ServeAuthOnce handles a single auth request from stdin and exits.
//
// Usage:
//
//	func main() {
//	    handler := &MyAuthDriver{}
//	    driver.ServeAuthOnce(handler)
//	}
func ServeAuthOnce(handler AuthHandler) {
	// Read request from stdin
	var payload AuthPayload
	if err := json.NewDecoder(os.Stdin).Decode(&payload); err != nil {
		writeAuthError(fmt.Sprintf("failed to parse payload: %v", err))
		os.Exit(1)
	}

	// Authenticate
	result, err := handler.Authenticate(&payload)
	if err != nil && result == nil {
		result = NewAuthError(err.Error())
	}

	// Write result to stdout
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		writeAuthError(fmt.Sprintf("failed to write result: %v", err))
		os.Exit(1)
	}
}

// writeError writes an error ResourceResult to stdout
func writeError(message string) {
	result := NewErrorResult(500, message)
	json.NewEncoder(os.Stdout).Encode(result)
	os.Stdout.Sync()
}

// writeAuthError writes an error AuthResult to stdout
func writeAuthError(message string) {
	result := NewAuthError(message)
	json.NewEncoder(os.Stdout).Encode(result)
	os.Stdout.Sync()
}
