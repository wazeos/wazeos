package iobus

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// WASMDriver wraps a WASM-based driver
type WASMDriver struct {
	spec       DriverSpec
	handleID   string
	bus        *IOBus
	metadata   map[string]any
	driverCtx  Context // Context used for internal handle calls
}

// NewWASMDriver creates a new WASM driver wrapper
func NewWASMDriver(spec DriverSpec, bus *IOBus) (*WASMDriver, error) {
	// Load WASM binary
	wasmBytes, err := loadWASMBinary(spec.Binary)
	if err != nil {
		return nil, fmt.Errorf("failed to load WASM binary: %w", err)
	}

	// Create context with full permissions for driver initialization
	ctx := NewContext(
		context.Background(),
		fmt.Sprintf("driver:%s", spec.Name),
		"init",
		"init",
		[]PermissionEntry{
			{URIPattern: "**", Permissions: []string{"call", "handle"}},
		},
		bus,
	)

	// Create WASM handle
	createReq := Request{
		URI:       "wasm://load",
		Operation: OpCreateHandle,
		Args: map[string]any{
			"binary": wasmBytes,
		},
	}

	createResp, err := bus.Call(ctx, createReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create WASM handle: %w", err)
	}

	if createResp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to create WASM handle: %s", createResp.Error)
	}

	handleID := string(createResp.Body)

	// Get driver metadata from WASM
	metadataReq := Request{
		URI:       handleID,
		Operation: OpCall,
		Args: map[string]any{
			"function": "driver_metadata",
		},
	}

	metadataResp, err := bus.Call(ctx, metadataReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get driver metadata: %w", err)
	}

	var metadata map[string]any
	if err := json.Unmarshal(metadataResp.Body, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse driver metadata: %w", err)
	}

	// Initialize the driver
	initReq := Request{
		URI:       handleID,
		Operation: OpCall,
		Args: map[string]any{
			"function": "driver_init",
			"config":   map[string]any{}, // Empty config for now
		},
	}

	initResp, err := bus.Call(ctx, initReq)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize driver: %w", err)
	}

	if initResp.StatusCode != 200 {
		return nil, fmt.Errorf("driver initialization failed: %s", initResp.Error)
	}

	return &WASMDriver{
		spec:       spec,
		handleID:   handleID,
		bus:        bus,
		metadata:   metadata,
		driverCtx:  ctx,
	}, nil
}

// URIPattern returns the URI pattern this driver handles
func (d *WASMDriver) URIPattern() string {
	return d.spec.URIPattern
}

// Class returns the driver class
func (d *WASMDriver) Class() DriverClass {
	return d.spec.Class
}

// Capabilities returns the capabilities this driver supports
func (d *WASMDriver) Capabilities() []Capability {
	return d.spec.Capabilities
}

// Init is a no-op for WASM drivers (already initialized in constructor)
func (d *WASMDriver) Init(ctx context.Context, config Config) error {
	return nil
}

// Close shuts down the WASM driver
func (d *WASMDriver) Close() error {
	// Create context for cleanup
	ctx := NewContext(
		context.Background(),
		fmt.Sprintf("driver:%s", d.spec.Name),
		"close",
		"close",
		[]PermissionEntry{
			{URIPattern: "**", Permissions: []string{"call", "handle"}},
		},
		d.bus,
	)

	// Close the WASM handle
	closeReq := Request{
		URI:       d.handleID,
		Operation: OpCloseHandle,
	}

	_, err := d.bus.Call(ctx, closeReq)
	return err
}

// Call executes a call on the WASM driver
func (d *WASMDriver) Call(ctx Context, req Request) (Response, error) {
	// Marshal request to JSON
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return Response{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Call the WASM driver handle using driver's context (for handle access)
	callReq := Request{
		URI:       d.handleID,
		Operation: OpCall,
		Args: map[string]any{
			"function": "driver_call",
			"request":  string(reqJSON),
		},
	}

	callResp, err := d.bus.Call(d.driverCtx, callReq)
	if err != nil {
		return Response{}, err
	}

	// Parse response
	var resp Response
	if err := json.Unmarshal(callResp.Body, &resp); err != nil {
		return Response{}, fmt.Errorf("failed to parse driver response: %w", err)
	}

	return resp, nil
}

// loadWASMBinary loads a WASM binary from a file path
func loadWASMBinary(path string) ([]byte, error) {
	// Note: We need to import "os" for this
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read WASM file %s: %w", path, err)
	}
	return data, nil
}
