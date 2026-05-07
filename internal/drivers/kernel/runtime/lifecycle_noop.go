package runtime

import (
	"context"

	"github.com/wazeos/wazeos/internal/types"
)

// NoopLifecycleManager is a no-op lifecycle manager that doesn't cache anything
// Every request will compile the WASM binary fresh (current behavior)
type NoopLifecycleManager struct{}

// NewNoopLifecycleManager creates a new no-op lifecycle manager
func NewNoopLifecycleManager() *NoopLifecycleManager {
	return &NoopLifecycleManager{}
}

// Name returns the lifecycle manager name
func (m *NoopLifecycleManager) Name() string {
	return "kernel.runtime.lifecycle.noop"
}

// Get always returns nil (no caching)
func (m *NoopLifecycleManager) Get(ctx context.Context, appID string) (types.CompiledModule, error) {
	return nil, nil // Not found, forces recompilation
}

// Put is a no-op (doesn't cache)
func (m *NoopLifecycleManager) Put(ctx context.Context, appID string, module types.CompiledModule) error {
	// Don't cache, but also don't close - compiled modules stay alive
	// on the shared runtime and can be reused across requests
	// The runtime manages their lifecycle
	return nil
}

// Remove is a no-op
func (m *NoopLifecycleManager) Remove(ctx context.Context, appID string) error {
	return nil
}

// Clear is a no-op
func (m *NoopLifecycleManager) Clear(ctx context.Context) error {
	return nil
}

// Stats returns empty stats
func (m *NoopLifecycleManager) Stats() types.LifecycleStats {
	return types.LifecycleStats{
		Hits:        0,
		Misses:      0,
		Evictions:   0,
		CurrentSize: 0,
		MaxSize:     0,
	}
}
