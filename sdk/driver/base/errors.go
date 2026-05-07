package base

import (
	"encoding/json"
	"fmt"

	"github.com/wazeos/wazeos/internal/types"
)

// ErrorResponse creates a standardized error response
func ErrorResponse(statusCode int, message string, args ...interface{}) *types.ResourceResult {
	formattedMessage := fmt.Sprintf(message, args...)
	body, _ := json.Marshal(map[string]string{
		"error": formattedMessage,
	})

	return &types.ResourceResult{
		StatusCode: statusCode,
		Headers:    make(map[string]string),
		Body:       body,
		Error:      formattedMessage,
	}
}

// ErrorResponseWithDetails creates an error response with additional details
func ErrorResponseWithDetails(statusCode int, message string, details map[string]interface{}) *types.ResourceResult {
	body, _ := json.Marshal(map[string]interface{}{
		"error":   message,
		"details": details,
	})

	return &types.ResourceResult{
		StatusCode: statusCode,
		Headers:    make(map[string]string),
		Body:       body,
		Error:      message,
	}
}

// Common error responses

// BadRequest returns a 400 error response
func BadRequest(message string, args ...interface{}) *types.ResourceResult {
	return ErrorResponse(400, message, args...)
}

// Unauthorized returns a 401 error response
func Unauthorized(message string, args ...interface{}) *types.ResourceResult {
	return ErrorResponse(401, message, args...)
}

// Forbidden returns a 403 error response
func Forbidden(message string, args ...interface{}) *types.ResourceResult {
	return ErrorResponse(403, message, args...)
}

// NotFound returns a 404 error response
func NotFound(message string, args ...interface{}) *types.ResourceResult {
	return ErrorResponse(404, message, args...)
}

// Timeout returns a 504 error response
func Timeout(message string, args ...interface{}) *types.ResourceResult {
	return ErrorResponse(504, message, args...)
}

// InternalError returns a 500 error response
func InternalError(message string, args ...interface{}) *types.ResourceResult {
	return ErrorResponse(500, message, args...)
}

// SuccessResponse creates a standardized success response
func SuccessResponse(body []byte) *types.ResourceResult {
	return &types.ResourceResult{
		StatusCode: 200,
		Headers:    make(map[string]string),
		Body:       body,
	}
}

// SuccessResponseWithHeaders creates a success response with custom headers
func SuccessResponseWithHeaders(body []byte, headers map[string]string) *types.ResourceResult {
	return &types.ResourceResult{
		StatusCode: 200,
		Headers:    headers,
		Body:       body,
	}
}
