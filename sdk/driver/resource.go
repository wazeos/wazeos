package driver

import (
	"encoding/json"
)

// ResourceHandler is the interface that resource drivers must implement.
type ResourceHandler interface {
	// HandleCall processes a resource call and returns a result.
	HandleCall(call *ResourceCall) (*ResourceResult, error)
}

// ServeResource is the main entry point for resource drivers.
// It handles deserialization, calls the handler, and serializes the result.
//
// Example usage in main.go:
//
//	type MyDriver struct{}
//
//	func (d *MyDriver) HandleCall(call *driver.ResourceCall) (*driver.ResourceResult, error) {
//	    // Process the call
//	    return driver.NewResourceResult(200, []byte("success")), nil
//	}
//
//	//export handle_call
//	func handleCall(callPtr, callLen uint32) uint32 {
//	    handler := &MyDriver{}
//	    return driver.ServeResource(handler, callPtr, callLen)
//	}
func ServeResource(handler ResourceHandler, callPtr, callLen uint32) uint32 {
	// Deserialize ResourceCall
	callData := CopyFromMemory(callPtr, callLen)
	var call ResourceCall
	if err := json.Unmarshal(callData, &call); err != nil {
		return encodeResourceError(400, "invalid call: "+err.Error())
	}

	// Call handler
	result, err := handler.HandleCall(&call)
	if err != nil {
		if result == nil {
			result = NewErrorResult(500, err.Error())
		}
	}

	// Serialize result
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return encodeResourceError(500, "failed to serialize result: "+err.Error())
	}

	// Copy to memory and return pointer
	return CopyToMemory(resultJSON)
}

// encodeResourceError creates an error result and returns its pointer.
func encodeResourceError(statusCode int, message string) uint32 {
	result := NewErrorResult(statusCode, message)
	resultJSON, _ := json.Marshal(result)
	return CopyToMemory(resultJSON)
}
