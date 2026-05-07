package app

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Logger provides structured logging to stderr with request context.
type Logger struct {
	ctx *Context
}

// Field represents a key-value pair for structured logging.
type Field struct {
	Key   string
	Value interface{}
}

// Field constructor helpers for common types.

// String creates a string field.
func String(key, val string) Field {
	return Field{Key: key, Value: val}
}

// Int creates an integer field.
func Int(key string, val int) Field {
	return Field{Key: key, Value: val}
}

// Int64 creates an int64 field.
func Int64(key string, val int64) Field {
	return Field{Key: key, Value: val}
}

// Bool creates a boolean field.
func Bool(key string, val bool) Field {
	return Field{Key: key, Value: val}
}

// Any creates a field with any value type.
func Any(key string, val interface{}) Field {
	return Field{Key: key, Value: val}
}

// ErrorField creates an error field.
func ErrorField(err error) Field {
	return Field{Key: "error", Value: err.Error()}
}

// Debug logs a debug message with optional fields.
func (l *Logger) Debug(msg string, fields ...Field) {
	l.log("DEBUG", msg, fields)
}

// Info logs an info message with optional fields.
func (l *Logger) Info(msg string, fields ...Field) {
	l.log("INFO", msg, fields)
}

// Warn logs a warning message with optional fields.
func (l *Logger) Warn(msg string, fields ...Field) {
	l.log("WARN", msg, fields)
}

// Error logs an error message with optional fields.
func (l *Logger) Error(msg string, fields ...Field) {
	l.log("ERROR", msg, fields)
}

// log writes a structured log entry to stderr in JSON format.
func (l *Logger) log(level, msg string, fields []Field) {
	// Create log entry with standard fields
	entry := make(map[string]interface{})
	entry["level"] = level
	entry["message"] = msg
	entry["timestamp"] = time.Now().Format(time.RFC3339)

	// Add context fields if available
	if l.ctx != nil {
		if l.ctx.RequestID != "" {
			entry["requestId"] = l.ctx.RequestID
		}
		if l.ctx.TraceID != "" {
			entry["traceId"] = l.ctx.TraceID
		}
		if l.ctx.Principal != "" {
			entry["principal"] = l.ctx.Principal
		}
	}

	// Add custom fields
	for _, field := range fields {
		entry[field.Key] = field.Value
	}

	// Marshal to JSON and write to stderr
	jsonData, err := json.Marshal(entry)
	if err != nil {
		// Fallback to plain text if JSON marshaling fails
		fmt.Fprintf(os.Stderr, "[%s] %s: %s (JSON marshal error: %v)\n", level, msg, err, err)
		return
	}

	fmt.Fprintf(os.Stderr, "%s\n", jsonData)
}
