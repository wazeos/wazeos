package types

import "context"

// IOBus is the kernel's I/O routing and orchestration layer
// Implementations handle request routing, queueing, and driver management
type IOBus interface {
	// Call routes a resource call to the appropriate driver
	Call(ctx context.Context, call *ResourceCall) (*ResourceResult, error)

	// RegisterDriver registers a driver for URI scheme routing
	RegisterDriver(driver ResourceDriver) error
}
