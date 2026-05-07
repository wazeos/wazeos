package base

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorResponse(t *testing.T) {
	result := ErrorResponse(400, "test error: %s", "details")

	assert.Equal(t, 400, result.StatusCode)
	assert.Equal(t, "test error: details", result.Error)
	assert.NotNil(t, result.Headers)

	var body map[string]string
	err := json.Unmarshal(result.Body, &body)
	assert.NoError(t, err)
	assert.Equal(t, "test error: details", body["error"])
}

func TestErrorResponseWithDetails(t *testing.T) {
	details := map[string]interface{}{
		"field": "username",
		"value": "invalid",
	}
	result := ErrorResponseWithDetails(400, "validation error", details)

	assert.Equal(t, 400, result.StatusCode)
	assert.Equal(t, "validation error", result.Error)

	var body map[string]interface{}
	err := json.Unmarshal(result.Body, &body)
	assert.NoError(t, err)
	assert.Equal(t, "validation error", body["error"])

	responseDetails := body["details"].(map[string]interface{})
	assert.Equal(t, "username", responseDetails["field"])
	assert.Equal(t, "invalid", responseDetails["value"])
}

func TestBadRequest(t *testing.T) {
	result := BadRequest("invalid input")

	assert.Equal(t, 400, result.StatusCode)
	assert.Equal(t, "invalid input", result.Error)
}

func TestUnauthorized(t *testing.T) {
	result := Unauthorized("authentication required")

	assert.Equal(t, 401, result.StatusCode)
	assert.Equal(t, "authentication required", result.Error)
}

func TestForbidden(t *testing.T) {
	result := Forbidden("access denied")

	assert.Equal(t, 403, result.StatusCode)
	assert.Equal(t, "access denied", result.Error)
}

func TestNotFound(t *testing.T) {
	result := NotFound("resource not found")

	assert.Equal(t, 404, result.StatusCode)
	assert.Equal(t, "resource not found", result.Error)
}

func TestTimeout(t *testing.T) {
	result := Timeout("request timeout")

	assert.Equal(t, 504, result.StatusCode)
	assert.Equal(t, "request timeout", result.Error)
}

func TestInternalError(t *testing.T) {
	result := InternalError("internal server error")

	assert.Equal(t, 500, result.StatusCode)
	assert.Equal(t, "internal server error", result.Error)
}

func TestSuccessResponse(t *testing.T) {
	body := []byte(`{"status":"ok"}`)
	result := SuccessResponse(body)

	assert.Equal(t, 200, result.StatusCode)
	assert.Equal(t, body, result.Body)
	assert.NotNil(t, result.Headers)
	assert.Empty(t, result.Error)
}

func TestSuccessResponseWithHeaders(t *testing.T) {
	body := []byte(`{"status":"ok"}`)
	headers := map[string]string{
		"Content-Type": "application/json",
		"X-Custom":     "value",
	}
	result := SuccessResponseWithHeaders(body, headers)

	assert.Equal(t, 200, result.StatusCode)
	assert.Equal(t, body, result.Body)
	assert.Equal(t, headers, result.Headers)
	assert.Empty(t, result.Error)
}

func TestErrorResponse_FormatArgs(t *testing.T) {
	result := ErrorResponse(404, "user %s not found in %s", "alice", "database")

	assert.Equal(t, 404, result.StatusCode)
	assert.Equal(t, "user alice not found in database", result.Error)

	var body map[string]string
	err := json.Unmarshal(result.Body, &body)
	assert.NoError(t, err)
	assert.Equal(t, "user alice not found in database", body["error"])
}
