// Package app provides high-level APIs for building WazeOS applications.
//
// This SDK simplifies app development by providing:
// - Handler interfaces for different app patterns (CLI, Request, Stream, Message)
// - High-level I/O operations (file, HTTP, fn://, queue://)
// - Structured logging with request context
// - Type-safe error handling
// - Testing utilities
package app

import "encoding/json"

// Response represents the output of an app execution.
type Response struct {
	StatusCode int               `json:"statusCode"` // HTTP-style status code (200, 404, 500, etc.)
	Headers    map[string]string `json:"headers"`    // Response metadata
	Body       []byte            `json:"body"`       // Response payload
	ExitCode   int               `json:"exitCode"`   // Process exit code (0 = success)
}

// Success creates a successful response with the given body.
func Success(body []byte) *Response {
	return &Response{
		StatusCode: 200,
		Headers:    make(map[string]string),
		Body:       body,
		ExitCode:   0,
	}
}

// SuccessString creates a successful response with a string body.
func SuccessString(message string) *Response {
	return Success([]byte(message))
}

// SuccessJSON creates a successful response with a JSON-encoded body.
func SuccessJSON(v interface{}) (*Response, error) {
	body, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	resp := Success(body)
	resp.Headers["Content-Type"] = "application/json"
	return resp, nil
}

// Error creates an error response with the given status code and message.
func Error(statusCode int, message string) *Response {
	exitCode := 1
	if statusCode >= 500 {
		exitCode = 2 // Server errors get exit code 2
	}
	return &Response{
		StatusCode: statusCode,
		Headers:    make(map[string]string),
		Body:       []byte(message),
		ExitCode:   exitCode,
	}
}

// ErrorWithCode creates an error response with explicit exit code.
func ErrorWithCode(statusCode, exitCode int, message string) *Response {
	return &Response{
		StatusCode: statusCode,
		Headers:    make(map[string]string),
		Body:       []byte(message),
		ExitCode:   exitCode,
	}
}

// BadRequest creates a 400 Bad Request response.
func BadRequest(message string) *Response {
	return Error(400, message)
}

// Forbidden creates a 403 Forbidden response.
func Forbidden(message string) *Response {
	return Error(403, message)
}

// NotFound creates a 404 Not Found response.
func NotFound(message string) *Response {
	return Error(404, message)
}

// InternalError creates a 500 Internal Server Error response.
func InternalError(message string) *Response {
	return Error(500, message)
}
