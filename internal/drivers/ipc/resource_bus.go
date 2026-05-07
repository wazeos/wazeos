package ipc

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/wazeos/wazeos/internal/types"
)

// IPCResourceBus implements ResourceBus as a kernel.ipc driver
// This handles all resource call routing with full audit logging
type IPCResourceBus struct {
	drivers      map[string]types.ResourceDriver // scheme -> driver
	secretsStore types.ResourceDriver             // special case for secrets
	mu           sync.RWMutex
}

// NewIPCResourceBus creates a new IPC-based resource bus
func NewIPCResourceBus(secretsStore types.ResourceDriver) *IPCResourceBus {
	return &IPCResourceBus{
		drivers:      make(map[string]types.ResourceDriver),
		secretsStore: secretsStore,
	}
}

// Call routes resource calls to appropriate drivers with full audit logging
func (b *IPCResourceBus) Call(ctx context.Context, call *types.ResourceCall) (*types.ResourceResult, error) {
	// Audit log: incoming request
	fmt.Printf("\n[IPC-BUS] ═══════════════════════════════════════════════════════\n")
	fmt.Printf("[IPC-BUS] Incoming Request\n")
	fmt.Printf("[IPC-BUS]   Method: %s\n", call.Method)
	fmt.Printf("[IPC-BUS]   URI: %s\n", call.URI)

	if len(call.Headers) > 0 {
		fmt.Printf("[IPC-BUS]   Headers:\n")
		for k, v := range call.Headers {
			if k == "X-WazeOS-Credentials" {
				// Parse to show credential keys without values
				fmt.Printf("[IPC-BUS]     %s: [INJECTED] (%d bytes)\n", k, len(v))
			} else {
				fmt.Printf("[IPC-BUS]     %s: %s\n", k, v)
			}
		}
	}

	if len(call.Body) > 0 {
		fmt.Printf("[IPC-BUS]   Body: %d bytes\n", len(call.Body))
	}

	// Route to secrets store for secret:// URIs
	if strings.HasPrefix(call.URI, "secret://") {
		fmt.Printf("[IPC-BUS]   → Routing to: kernel.security.secrets\n")
		result, err := b.secretsStore.HandleCall(ctx, call)
		b.logResult(result, err)
		return result, err
	}

	// Extract scheme from URI
	schemeEnd := strings.Index(call.URI, "://")
	if schemeEnd == -1 {
		fmt.Printf("[IPC-BUS]   ❌ Invalid URI: missing scheme\n")
		result := &types.ResourceResult{
			StatusCode: 400,
			Body:       []byte(`{"error":"invalid URI: missing scheme"}`),
		}
		b.logResult(result, nil)
		return result, nil
	}

	scheme := call.URI[:schemeEnd]

	// Find driver for scheme
	b.mu.RLock()
	driver, ok := b.drivers[scheme]
	b.mu.RUnlock()

	if !ok {
		fmt.Printf("[IPC-BUS]   ❌ No driver registered for scheme: %s\n", scheme)
		result := &types.ResourceResult{
			StatusCode: 404,
			Body:       []byte(fmt.Sprintf(`{"error":"no driver for scheme: %s"}`, scheme)),
		}
		b.logResult(result, nil)
		return result, nil
	}

	fmt.Printf("[IPC-BUS]   → Routing to: %s (scheme: %s)\n", driver.Name(), scheme)

	// Call driver
	result, err := driver.HandleCall(ctx, call)
	b.logResult(result, err)
	return result, err
}

// logResult logs the response from a driver call
func (b *IPCResourceBus) logResult(result *types.ResourceResult, err error) {
	if err != nil {
		fmt.Printf("[IPC-BUS]   ❌ Driver Error: %v\n", err)
	}

	if result != nil {
		statusIcon := "✓"
		if result.StatusCode >= 400 {
			statusIcon = "❌"
		} else if result.StatusCode >= 300 {
			statusIcon = "⚠️"
		}

		fmt.Printf("[IPC-BUS] %s Response\n", statusIcon)
		fmt.Printf("[IPC-BUS]   Status: %d\n", result.StatusCode)

		if len(result.Body) > 0 {
			if result.StatusCode >= 200 && result.StatusCode < 300 {
				fmt.Printf("[IPC-BUS]   Body: %d bytes\n", len(result.Body))
			} else {
				// Show error bodies
				bodyStr := string(result.Body)
				if len(bodyStr) > 200 {
					bodyStr = bodyStr[:200] + "..."
				}
				fmt.Printf("[IPC-BUS]   Body: %s\n", bodyStr)
			}
		}

		if len(result.Headers) > 0 {
			fmt.Printf("[IPC-BUS]   Response Headers: %d\n", len(result.Headers))
		}
	}

	fmt.Printf("[IPC-BUS] ═══════════════════════════════════════════════════════\n\n")
}

// RegisterDriver registers a driver for a URI scheme
func (b *IPCResourceBus) RegisterDriver(driver types.ResourceDriver) error {
	// Extract scheme from driver name (e.g., "io.resource.s3" -> "s3")
	scheme := driver.Name()
	if strings.Contains(scheme, ".") {
		parts := strings.Split(scheme, ".")
		scheme = parts[len(parts)-1]
	}

	b.mu.Lock()
	b.drivers[scheme] = driver
	b.mu.Unlock()

	fmt.Printf("[IPC-BUS] ✓ Registered driver: %s → scheme '%s'\n", driver.Name(), scheme)
	return nil
}
