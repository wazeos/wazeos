package types

import (
	"errors"
	"fmt"
)

// Standard errors used throughout the system.
var (
	// ErrNotFound indicates a resource was not found.
	ErrNotFound = errors.New("resource not found")

	// ErrPermissionDenied indicates access was denied due to insufficient permissions.
	ErrPermissionDenied = errors.New("permission denied")

	// ErrInvalidRequest indicates the request was malformed or invalid.
	ErrInvalidRequest = errors.New("invalid request")

	// ErrTimeout indicates an operation timed out.
	ErrTimeout = errors.New("operation timed out")

	// ErrInternal indicates an internal system error.
	ErrInternal = errors.New("internal error")

	// ErrAbstain is returned by authn drivers when they don't recognize credentials.
	ErrAbstain = errors.New("authn: driver abstains")

	// ErrAlreadyExists indicates a resource already exists.
	ErrAlreadyExists = errors.New("resource already exists")

	// ErrNotSupported indicates an operation is not supported.
	ErrNotSupported = errors.New("operation not supported")

	// ErrDependencyNotFound indicates a required dependency is missing.
	ErrDependencyNotFound = errors.New("dependency not found")

	// ErrCycleDetected indicates a circular dependency or call chain.
	ErrCycleDetected = errors.New("cycle detected")

	// ErrMaxDepthExceeded indicates maximum call depth was exceeded.
	ErrMaxDepthExceeded = errors.New("maximum call depth exceeded")
)

// AppError wraps an error with additional context.
type AppError struct {
	Code    string // Error code for programmatic handling
	Message string // Human-readable message
	Err     error  // Underlying error
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap implements error unwrapping.
func (e *AppError) Unwrap() error {
	return e.Err
}

// NewAppError creates a new AppError.
func NewAppError(code, message string, err error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// IsNotFound checks if an error is or wraps ErrNotFound.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsPermissionDenied checks if an error is or wraps ErrPermissionDenied.
func IsPermissionDenied(err error) bool {
	return errors.Is(err, ErrPermissionDenied)
}

// IsInvalidRequest checks if an error is or wraps ErrInvalidRequest.
func IsInvalidRequest(err error) bool {
	return errors.Is(err, ErrInvalidRequest)
}

// IsTimeout checks if an error is or wraps ErrTimeout.
func IsTimeout(err error) bool {
	return errors.Is(err, ErrTimeout)
}

// IsAbstain checks if an error is or wraps ErrAbstain.
func IsAbstain(err error) bool {
	return errors.Is(err, ErrAbstain)
}

// IsInternal checks if an error is or wraps ErrInternal.
func IsInternal(err error) bool {
	return errors.Is(err, ErrInternal)
}

// IsMaxDepthExceeded checks if an error is or wraps ErrMaxDepthExceeded.
func IsMaxDepthExceeded(err error) bool {
	return errors.Is(err, ErrMaxDepthExceeded)
}

// IsAlreadyExists checks if an error is or wraps ErrAlreadyExists.
func IsAlreadyExists(err error) bool {
	return errors.Is(err, ErrAlreadyExists)
}

// IsDependencyNotFound checks if an error is or wraps ErrDependencyNotFound.
func IsDependencyNotFound(err error) bool {
	return errors.Is(err, ErrDependencyNotFound)
}

// IsCycleDetected checks if an error is or wraps ErrCycleDetected.
func IsCycleDetected(err error) bool {
	return errors.Is(err, ErrCycleDetected)
}
