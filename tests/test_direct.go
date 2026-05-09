package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/wazeos/wazeos/core/kernel"
	"github.com/wazeos/wazeos/core/kernel/iobus"
)

func main() {
	// Initialize drivers
	kernel.InitDrivers()
	bus := iobus.GetDefaultBus()

	// Create context
	ctx := iobus.NewContext(
		context.Background(),
		"test",
		"test",
		"test",
		[]iobus.PermissionEntry{
			{URIPattern: "file://**", Permissions: []string{"call"}},
			{URIPattern: "native://file/**", Permissions: []string{"call"}},
		},
		bus,
	)

	// Test file read
	req := iobus.Request{
		URI:       "file:///tmp/test-wazeos.txt",
		Operation: iobus.OpCall,
	}

	resp, err := bus.Call(ctx, req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Body: %s\n", string(resp.Body))

	// Show JSON serialization
	jsonBytes, _ := json.Marshal(resp)
	fmt.Printf("JSON: %s\n", string(jsonBytes))
}
