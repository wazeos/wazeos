package types

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStandardErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{
			name: "not found",
			err:  ErrNotFound,
			msg:  "resource not found",
		},
		{
			name: "permission denied",
			err:  ErrPermissionDenied,
			msg:  "permission denied",
		},
		{
			name: "invalid request",
			err:  ErrInvalidRequest,
			msg:  "invalid request",
		},
		{
			name: "timeout",
			err:  ErrTimeout,
			msg:  "operation timed out",
		},
		{
			name: "internal",
			err:  ErrInternal,
			msg:  "internal error",
		},
		{
			name: "abstain",
			err:  ErrAbstain,
			msg:  "authn: driver abstains",
		},
		{
			name: "already exists",
			err:  ErrAlreadyExists,
			msg:  "resource already exists",
		},
		{
			name: "dependency not found",
			err:  ErrDependencyNotFound,
			msg:  "dependency not found",
		},
		{
			name: "cycle detected",
			err:  ErrCycleDetected,
			msg:  "cycle detected",
		},
		{
			name: "max depth exceeded",
			err:  ErrMaxDepthExceeded,
			msg:  "maximum call depth exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Error(t, tt.err)
			assert.Equal(t, tt.msg, tt.err.Error())
		})
	}
}

func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *AppError
		want string
	}{
		{
			name: "with underlying error",
			err: NewAppError("NOT_FOUND", "app not found", ErrNotFound),
			want: "NOT_FOUND: app not found: resource not found",
		},
		{
			name: "without underlying error",
			err: NewAppError("VALIDATION_ERROR", "invalid input", nil),
			want: "VALIDATION_ERROR: invalid input",
		},
		{
			name: "with custom underlying error",
			err: NewAppError("IO_ERROR", "failed to read file", errors.New("file not readable")),
			want: "IO_ERROR: failed to read file: file not readable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAppError_Unwrap(t *testing.T) {
	underlying := errors.New("underlying error")
	appErr := NewAppError("TEST", "test error", underlying)

	unwrapped := appErr.Unwrap()
	assert.Equal(t, underlying, unwrapped)
}

func TestAppError_UnwrapNil(t *testing.T) {
	appErr := NewAppError("TEST", "test error", nil)

	unwrapped := appErr.Unwrap()
	assert.Nil(t, unwrapped)
}

func TestNewAppError(t *testing.T) {
	tests := []struct {
		name        string
		code        string
		message     string
		err         error
		wantCode    string
		wantMessage string
		wantErr     error
	}{
		{
			name:        "complete error",
			code:        "NOT_FOUND",
			message:     "app not found",
			err:         ErrNotFound,
			wantCode:    "NOT_FOUND",
			wantMessage: "app not found",
			wantErr:     ErrNotFound,
		},
		{
			name:        "without underlying error",
			code:        "VALIDATION",
			message:     "invalid data",
			err:         nil,
			wantCode:    "VALIDATION",
			wantMessage: "invalid data",
			wantErr:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewAppError(tt.code, tt.message, tt.err)

			assert.Equal(t, tt.wantCode, got.Code)
			assert.Equal(t, tt.wantMessage, got.Message)
			assert.Equal(t, tt.wantErr, got.Err)
		})
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "direct error",
			err:  ErrNotFound,
			want: true,
		},
		{
			name: "wrapped in AppError",
			err:  NewAppError("NOT_FOUND", "test", ErrNotFound),
			want: true,
		},
		{
			name: "different error",
			err:  ErrPermissionDenied,
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsNotFound(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsPermissionDenied(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "direct error",
			err:  ErrPermissionDenied,
			want: true,
		},
		{
			name: "wrapped in AppError",
			err:  NewAppError("FORBIDDEN", "test", ErrPermissionDenied),
			want: true,
		},
		{
			name: "different error",
			err:  ErrNotFound,
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPermissionDenied(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsInvalidRequest(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "direct error",
			err:  ErrInvalidRequest,
			want: true,
		},
		{
			name: "wrapped in AppError",
			err:  NewAppError("BAD_REQUEST", "test", ErrInvalidRequest),
			want: true,
		},
		{
			name: "different error",
			err:  ErrNotFound,
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsInvalidRequest(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsTimeout(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "direct error",
			err:  ErrTimeout,
			want: true,
		},
		{
			name: "wrapped in AppError",
			err:  NewAppError("TIMEOUT", "test", ErrTimeout),
			want: true,
		},
		{
			name: "different error",
			err:  ErrNotFound,
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsTimeout(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsAbstain(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "direct error",
			err:  ErrAbstain,
			want: true,
		},
		{
			name: "wrapped in AppError",
			err:  NewAppError("ABSTAIN", "test", ErrAbstain),
			want: true,
		},
		{
			name: "different error",
			err:  ErrNotFound,
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAbstain(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestErrorWrapping(t *testing.T) {
	// Create a wrapped error chain
	baseErr := errors.New("io error")
	appErr := NewAppError("IO_ERROR", "failed to read", baseErr)

	// Test that errors.Is works correctly
	assert.True(t, errors.Is(appErr, baseErr))

	// Test that unwrapping works
	unwrapped := errors.Unwrap(appErr)
	assert.Equal(t, baseErr, unwrapped)
}
