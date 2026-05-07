package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/wazeos/wazeos/internal/types"
)

// WasmLifecycleManager delegates cache policy decisions to a WASM app
// The native Go side stores actual compiled modules
// The WASM side makes decisions about what to cache/evict
type WasmLifecycleManager struct {
	mu            sync.RWMutex
	policyRuntime wazero.Runtime
	policyModule  api.Module
	cache         map[string]types.CompiledModule // appID -> compiled module
	stats         types.LifecycleStats
}

// PolicyRequest represents a request to the policy WASM app
type PolicyRequest struct {
	Action string                 `json:"action"` // "get", "put", "remove", "clear"
	AppID  string                 `json:"appID"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// PolicyResponse represents a response from the policy WASM app
type PolicyResponse struct {
	Decision  string   `json:"decision"`  // "cache", "evict", "keep", "found", "not_found"
	EvictIDs  []string `json:"evictIDs"`  // AppIDs to evict (if decision is "evict")
	CacheHit  bool     `json:"cacheHit"`  // Whether this was a cache hit
	ErrorMsg  string   `json:"error,omitempty"`
}

// NewWasmLifecycleManager creates a lifecycle manager using a WASM policy app
func NewWasmLifecycleManager(ctx context.Context, policyWasmBinary []byte) (*WasmLifecycleManager, error) {
	// Create wazero runtime for the policy app
	rt := wazero.NewRuntime(ctx)

	// Instantiate WASI
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		rt.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate WASI: %w", err)
	}

	// Compile policy module
	compiled, err := rt.CompileModule(ctx, policyWasmBinary)
	if err != nil {
		rt.Close(ctx)
		return nil, fmt.Errorf("failed to compile policy module: %w", err)
	}

	// Instantiate policy module
	config := wazero.NewModuleConfig().
		WithName("lifecycle-policy").
		WithStdout(nil). // Policy app doesn't need stdout
		WithStderr(nil)

	mod, err := rt.InstantiateModule(ctx, compiled, config)
	if err != nil {
		rt.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate policy module: %w", err)
	}

	return &WasmLifecycleManager{
		policyRuntime: rt,
		policyModule:  mod,
		cache:         make(map[string]types.CompiledModule),
		stats: types.LifecycleStats{
			MaxSize: 100, // Default, policy app can adjust this
		},
	}, nil
}

// Name returns the lifecycle manager name
func (m *WasmLifecycleManager) Name() string {
	return "kernel.runtime.lifecycle.wasm"
}

// Get retrieves a compiled module from cache
func (m *WasmLifecycleManager) Get(ctx context.Context, appID string) (types.CompiledModule, error) {
	// Ask policy app if this is in cache
	resp, err := m.callPolicy(ctx, PolicyRequest{
		Action: "get",
		AppID:  appID,
	})
	if err != nil {
		return nil, fmt.Errorf("policy call failed: %w", err)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if resp.CacheHit {
		m.stats.Hits++
		module := m.cache[appID]
		return module, nil
	}

	m.stats.Misses++
	return nil, nil // Not in cache
}

// Put stores a compiled module in cache (or evicts based on policy)
func (m *WasmLifecycleManager) Put(ctx context.Context, appID string, module types.CompiledModule) error {
	// Ask policy app whether to cache and what to evict
	resp, err := m.callPolicy(ctx, PolicyRequest{
		Action: "put",
		AppID:  appID,
		Params: map[string]interface{}{
			"currentSize": len(m.cache),
		},
	})
	if err != nil {
		return fmt.Errorf("policy call failed: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Handle evictions first
	for _, evictID := range resp.EvictIDs {
		if cachedModule, ok := m.cache[evictID]; ok {
			if err := cachedModule.Close(ctx); err != nil {
				fmt.Printf("[LIFECYCLE] Warning: failed to close evicted module %s: %v\n", evictID, err)
			}
			delete(m.cache, evictID)
			m.stats.Evictions++
			fmt.Printf("[LIFECYCLE] Evicted module: %s\n", evictID)
		}
	}

	// Cache if policy says so
	if resp.Decision == "cache" {
		m.cache[appID] = module
		m.stats.CurrentSize = len(m.cache)
		fmt.Printf("[LIFECYCLE] Cached module: %s (cache size: %d)\n", appID, m.stats.CurrentSize)
		return nil
	}

	// Don't cache - close immediately
	if err := module.Close(ctx); err != nil {
		return fmt.Errorf("failed to close module: %w", err)
	}

	return nil
}

// Remove explicitly removes a module from cache
func (m *WasmLifecycleManager) Remove(ctx context.Context, appID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if module, ok := m.cache[appID]; ok {
		if err := module.Close(ctx); err != nil {
			return fmt.Errorf("failed to close module: %w", err)
		}
		delete(m.cache, appID)
		m.stats.CurrentSize = len(m.cache)
	}

	// Notify policy app
	_, err := m.callPolicy(ctx, PolicyRequest{
		Action: "remove",
		AppID:  appID,
	})

	return err
}

// Clear removes all cached modules
func (m *WasmLifecycleManager) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Close all modules
	for appID, module := range m.cache {
		if err := module.Close(ctx); err != nil {
			fmt.Printf("[LIFECYCLE] Warning: failed to close module %s: %v\n", appID, err)
		}
	}

	m.cache = make(map[string]types.CompiledModule)
	m.stats.CurrentSize = 0

	// Notify policy app
	_, err := m.callPolicy(ctx, PolicyRequest{
		Action: "clear",
	})

	return err
}

// Stats returns cache statistics
func (m *WasmLifecycleManager) Stats() types.LifecycleStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := m.stats
	stats.CurrentSize = len(m.cache)
	return stats
}

// Close shuts down the policy runtime
func (m *WasmLifecycleManager) Close(ctx context.Context) error {
	// Clear all cached modules first
	if err := m.Clear(ctx); err != nil {
		return err
	}

	// Close policy module
	if m.policyModule != nil {
		if err := m.policyModule.Close(ctx); err != nil {
			return err
		}
	}

	// Close runtime
	if m.policyRuntime != nil {
		return m.policyRuntime.Close(ctx)
	}

	return nil
}

// callPolicy calls the WASM policy app with a request
func (m *WasmLifecycleManager) callPolicy(ctx context.Context, req PolicyRequest) (*PolicyResponse, error) {
	// Serialize request to JSON
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create stdin/stdout buffers
	var stdout bytes.Buffer
	stdin := bytes.NewReader(reqJSON)

	// Re-instantiate module with stdin/stdout for this call
	// (wazero modules are single-use for stdin/stdout)
	config := wazero.NewModuleConfig().
		WithStdin(stdin).
		WithStdout(&stdout).
		WithName("lifecycle-policy")

	// Get the compiled module and instantiate
	// Note: We need to keep a reference to the compiled module
	// For now, this is a simplified version
	// TODO: Proper module reuse pattern

	// Parse response from stdout
	var resp PolicyResponse
	if err := json.NewDecoder(&stdout).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to parse policy response: %w", err)
	}

	if resp.ErrorMsg != "" {
		return nil, fmt.Errorf("policy error: %s", resp.ErrorMsg)
	}

	return &resp, nil
}
