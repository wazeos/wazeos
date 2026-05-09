package wasm

import (
	"context"
	"testing"

	"github.com/wazeos/wazeos/core/kernel/iobus"
)

// Simple WASM binary that exports a function returning 42
// (wat2wasm '(module (func (export "test_func") (result i32) i32.const 42))')
var simpleWASM = []byte{
	0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00, // WASM magic + version
	0x01, 0x05, 0x01, 0x60, 0x00, 0x01, 0x7f,       // Type section
	0x03, 0x02, 0x01, 0x00,                         // Function section
	0x07, 0x0d, 0x01, 0x09, 0x74, 0x65, 0x73, 0x74, // Export section
	0x5f, 0x66, 0x75, 0x6e, 0x63, 0x00, 0x00,
	0x0a, 0x06, 0x01, 0x04, 0x00, 0x41, 0x2a, 0x0b, // Code section
}

func TestDriverInit(t *testing.T) {
	driver := &Driver{}
	if driver == nil {
		t.Fatal("Driver initialization failed")
	}
}

func TestDriverCompileWASM(t *testing.T) {
	bus := iobus.GetDefaultBus()

	ctx := iobus.NewContext(
		context.Background(),
		"test-user",
		"req-test-wasm-compile",
		"trace-test",
		[]iobus.PermissionEntry{
			{URIPattern: "wasm://**", Permissions: []string{"handle"}},
		},
		bus,
	)

	req := iobus.Request{
		URI:       "wasm://load",
		Operation: iobus.OpCreateHandle,
		Args: map[string]any{
			"binary": simpleWASM,
		},
	}

	// Create handle (compile and instantiate WASM)
	resp, err := bus.Call(ctx, req)
	if err != nil {
		t.Fatalf("Failed to create WASM handle: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, resp.Error)
	}

	handleID := string(resp.Body)
	if handleID == "" {
		t.Fatal("Expected non-empty handle ID")
	}

	// Clean up
	closeReq := iobus.Request{
		URI:       handleID,
		Operation: iobus.OpCloseHandle,
	}
	_, err = bus.Call(ctx, closeReq)
	if err != nil {
		t.Errorf("Failed to close handle: %v", err)
	}
}

func TestDriverCallFunction(t *testing.T) {
	bus := iobus.GetDefaultBus()

	ctx := iobus.NewContext(
		context.Background(),
		"test-user",
		"req-test-wasm-call",
		"trace-test",
		[]iobus.PermissionEntry{
			{URIPattern: "wasm://**", Permissions: []string{"handle", "call"}},
			{URIPattern: "kernel://session/**", Permissions: []string{"call"}},
		},
		bus,
	)

	// Create handle
	createReq := iobus.Request{
		URI:       "wasm://load",
		Operation: iobus.OpCreateHandle,
		Args: map[string]any{
			"binary": simpleWASM,
		},
	}

	createResp, err := bus.Call(ctx, createReq)
	if err != nil {
		t.Fatalf("Failed to create WASM handle: %v", err)
	}

	handleID := string(createResp.Body)
	defer func() {
		closeReq := iobus.Request{
			URI:       handleID,
			Operation: iobus.OpCloseHandle,
		}
		bus.Call(ctx, closeReq)
	}()

	// Call function
	callReq := iobus.Request{
		URI:       handleID,
		Operation: iobus.OpCall,
		Args: map[string]any{
			"function": "test_func",
		},
	}

	resp, err := bus.Call(ctx, callReq)
	if err != nil {
		t.Fatalf("Failed to call WASM function: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d: %s", resp.StatusCode, resp.Error)
	}

	t.Logf("WASM function returned: %s", string(resp.Body))
}

func TestDriverInvalidWASM(t *testing.T) {
	bus := iobus.GetDefaultBus()

	ctx := iobus.NewContext(
		context.Background(),
		"test-user",
		"req-test-invalid",
		"trace-test",
		[]iobus.PermissionEntry{
			{URIPattern: "wasm://**", Permissions: []string{"handle"}},
		},
		bus,
	)

	req := iobus.Request{
		URI:       "wasm://load",
		Operation: iobus.OpCreateHandle,
		Args: map[string]any{
			"binary": []byte("invalid wasm binary"),
		},
	}

	resp, err := bus.Call(ctx, req)
	if err == nil && resp.StatusCode == 200 {
		t.Error("Expected error for invalid WASM binary")
	}
}

func TestDriverEmptyBody(t *testing.T) {
	bus := iobus.GetDefaultBus()

	ctx := iobus.NewContext(
		context.Background(),
		"test-user",
		"req-test-empty",
		"trace-test",
		[]iobus.PermissionEntry{
			{URIPattern: "wasm://**", Permissions: []string{"handle"}},
		},
		bus,
	)

	req := iobus.Request{
		URI:       "wasm://load",
		Operation: iobus.OpCreateHandle,
		Args: map[string]any{
			"binary": []byte{},
		},
	}

	resp, err := bus.Call(ctx, req)
	if err == nil && resp.StatusCode == 200 {
		t.Error("Expected error for empty binary")
	}
}

func TestDriverMissingFunction(t *testing.T) {
	bus := iobus.GetDefaultBus()

	ctx := iobus.NewContext(
		context.Background(),
		"test-user",
		"req-test-missing",
		"trace-test",
		[]iobus.PermissionEntry{
			{URIPattern: "wasm://**", Permissions: []string{"handle", "call"}},
			{URIPattern: "kernel://session/**", Permissions: []string{"call"}},
		},
		bus,
	)

	// Create handle
	createReq := iobus.Request{
		URI:       "wasm://load",
		Operation: iobus.OpCreateHandle,
		Args: map[string]any{
			"binary": simpleWASM,
		},
	}

	createResp, err := bus.Call(ctx, createReq)
	if err != nil {
		t.Fatalf("Failed to create WASM handle: %v", err)
	}

	handleID := string(createResp.Body)
	defer func() {
		closeReq := iobus.Request{
			URI:       handleID,
			Operation: iobus.OpCloseHandle,
		}
		bus.Call(ctx, closeReq)
	}()

	// Try to call non-existent function
	callReq := iobus.Request{
		URI:       handleID,
		Operation: iobus.OpCall,
		Args: map[string]any{
			"function": "nonexistent_function",
		},
	}

	resp, err := bus.Call(ctx, callReq)
	if err == nil && resp.StatusCode == 200 {
		t.Error("Expected error for missing function")
	}
}

func TestDriverMetadata(t *testing.T) {
	driver := &Driver{
		uriPattern: "wasm://**",
		class:      iobus.RuntimeDriver,
		caps:       []iobus.Capability{iobus.CapHandle},
	}

	if driver.URIPattern() != "wasm://**" {
		t.Errorf("Expected URI pattern 'wasm://**', got '%s'", driver.URIPattern())
	}

	if driver.Class() != iobus.RuntimeDriver {
		t.Errorf("Expected class 'runtime', got '%s'", driver.Class())
	}

	caps := driver.Capabilities()
	if len(caps) != 1 || caps[0] != iobus.CapHandle {
		t.Errorf("Expected capabilities [handle], got %v", caps)
	}
}
