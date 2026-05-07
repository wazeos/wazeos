package types

import "fmt"

// ErrorResult creates a ResourceResult representing an error response.
// The error message is formatted as JSON and included in the body.
func ErrorResult(statusCode int, message string, args ...interface{}) *ResourceResult {
	errMsg := fmt.Sprintf(message, args...)
	body := []byte(fmt.Sprintf(`{"error":"%s"}`, errMsg))

	return &ResourceResult{
		StatusCode: statusCode,
		Headers:    make(map[string]string),
		Body:       body,
		Error:      errMsg,
	}
}

// SuccessResult creates a ResourceResult representing a successful response.
func SuccessResult(statusCode int, body []byte) *ResourceResult {
	return &ResourceResult{
		StatusCode: statusCode,
		Headers:    make(map[string]string),
		Body:       body,
		Error:      "",
	}
}

// SuccessResultWithHeaders creates a ResourceResult with custom headers.
func SuccessResultWithHeaders(statusCode int, body []byte, headers map[string]string) *ResourceResult {
	if headers == nil {
		headers = make(map[string]string)
	}

	return &ResourceResult{
		StatusCode: statusCode,
		Headers:    headers,
		Body:       body,
		Error:      "",
	}
}
