package driver

import (
	"encoding/json"
)

// AuthHandler is the interface that authentication drivers must implement.
type AuthHandler interface {
	// Authenticate validates credentials and returns a principal or error.
	// Return AuthResult with Error="abstain" if the driver doesn't handle this auth type.
	Authenticate(payload *AuthPayload) (*AuthResult, error)
}

// ServeAuth is the main entry point for authentication drivers.
// It handles deserialization, calls the handler, and serializes the result.
//
// Example usage in main.go:
//
//	type MyAuthDriver struct{}
//
//	func (d *MyAuthDriver) Authenticate(payload *driver.AuthPayload) (*driver.AuthResult, error) {
//	    // Validate credentials
//	    if valid {
//	        return driver.NewAuthResult("user:alice"), nil
//	    }
//	    return driver.NewAuthError("invalid credentials"), nil
//	}
//
//	//export authenticate
//	func authenticate(payloadPtr, payloadLen uint32) uint32 {
//	    handler := &MyAuthDriver{}
//	    return driver.ServeAuth(handler, payloadPtr, payloadLen)
//	}
func ServeAuth(handler AuthHandler, payloadPtr, payloadLen uint32) uint32 {
	// Deserialize AuthPayload
	payloadData := CopyFromMemory(payloadPtr, payloadLen)
	var payload AuthPayload
	if err := json.Unmarshal(payloadData, &payload); err != nil {
		return encodeAuthError("invalid payload: " + err.Error())
	}

	// Call handler
	result, err := handler.Authenticate(&payload)
	if err != nil {
		if result == nil {
			result = NewAuthError(err.Error())
		}
	}

	// Serialize result
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return encodeAuthError("failed to serialize result: " + err.Error())
	}

	// Copy to memory and return pointer
	return CopyToMemory(resultJSON)
}

// encodeAuthError creates an error result and returns its pointer.
func encodeAuthError(message string) uint32 {
	result := NewAuthError(message)
	resultJSON, _ := json.Marshal(result)
	return CopyToMemory(resultJSON)
}

// EncodeAuthAbstain creates an abstain result and returns its pointer.
// Use this when the driver doesn't handle the provided authentication type.
func EncodeAuthAbstain() uint32 {
	result := NewAuthAbstain()
	resultJSON, _ := json.Marshal(result)
	return CopyToMemory(resultJSON)
}
