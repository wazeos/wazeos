package app

import "fmt"

// AppError represents a structured application error with status code.
type AppError struct {
	Code    string // Error code for programmatic handling (e.g., "PERMISSION_DENIED")
	Message string // Human-readable error message
	Status  int    // HTTP status code
	Cause   error  // Underlying error (if any)
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error for error wrapping.
func (e *AppError) Unwrap() error {
	return e.Cause
}

// NewError creates a new AppError with the given code, message, and status.
func NewError(code, message string, status int) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Status:  status,
	}
}

// WrapError wraps an existing error with code, message, and status.
func WrapError(err error, code, message string, status int) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Status:  status,
		Cause:   err,
	}
}

// Common error constants for app development.
var (
	// ErrPermissionDenied indicates the operation is not permitted.
	ErrPermissionDenied = NewError("PERMISSION_DENIED", "permission denied", 403)

	// ErrNotFound indicates the requested resource was not found.
	ErrNotFound = NewError("NOT_FOUND", "resource not found", 404)

	// ErrInvalidInput indicates the input is invalid or malformed.
	ErrInvalidInput = NewError("INVALID_INPUT", "invalid input", 400)

	// ErrInternal indicates an internal server error.
	ErrInternal = NewError("INTERNAL_ERROR", "internal error", 500)

	// ErrTimeout indicates the operation timed out.
	ErrTimeout = NewError("TIMEOUT", "operation timed out", 504)

	// ErrUnavailable indicates the service is unavailable.
	ErrUnavailable = NewError("UNAVAILABLE", "service unavailable", 503)
)
