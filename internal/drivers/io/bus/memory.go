package bus

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/wazeos/wazeos/internal/types"
)

// MemoryIOBus implements io.bus interface using in-memory state
// This is a stateful, native Go implementation for routing resource calls
type MemoryIOBus struct {
	drivers      map[string]types.ResourceDriver // scheme -> driver
	secretsStore types.ResourceDriver             // special case for secrets
	mu           sync.RWMutex
}

// NewMemoryIOBus creates a new in-memory IO bus
func NewMemoryIOBus(secretsStore types.ResourceDriver) *MemoryIOBus {
	return &MemoryIOBus{
		drivers:      make(map[string]types.ResourceDriver),
		secretsStore: secretsStore,
	}
}

// Call routes resource calls to appropriate drivers with full audit logging
func (b *MemoryIOBus) Call(ctx context.Context, call *types.ResourceCall) (*types.ResourceResult, error) {
	// Audit log: incoming request
	fmt.Fprintf(os.Stderr,"\n[IOBUS] ════════════════════════════════════════════════════════\n")
	fmt.Fprintf(os.Stderr,"[IOBUS] io.bus.memory\n")
	fmt.Fprintf(os.Stderr,"[IOBUS] Incoming Request\n")
	fmt.Fprintf(os.Stderr,"[IOBUS]   URI: %s\n", call.URI)
	fmt.Fprintf(os.Stderr,"[IOBUS]   Permissions: %v\n", call.Permissions)

	if len(call.Headers) > 0 {
		fmt.Fprintf(os.Stderr,"[IOBUS]   Headers:\n")
		for k, v := range call.Headers {
			if k == "X-WazeOS-Credentials" {
				// Parse to show credential keys without values
				fmt.Fprintf(os.Stderr,"[IOBUS]     %s: [INJECTED] (%d bytes)\n", k, len(v))
			} else {
				fmt.Fprintf(os.Stderr,"[IOBUS]     %s: %s\n", k, v)
			}
		}
	}

	if len(call.Body) > 0 {
		fmt.Fprintf(os.Stderr,"[IOBUS]   Body: %d bytes\n", len(call.Body))
	}

	// Route to secrets store for secret:// URIs
	if strings.HasPrefix(call.URI, "secret://") {
		fmt.Fprintf(os.Stderr,"[IOBUS]   → Routing to: kernel.security.secrets\n")
		result, err := b.secretsStore.HandleCall(ctx, call)
		b.logResult(result, err)
		return result, err
	}

	// Extract scheme from URI
	schemeEnd := strings.Index(call.URI, "://")
	if schemeEnd == -1 {
		fmt.Fprintf(os.Stderr,"[IOBUS]   ❌ Invalid URI: missing scheme\n")
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
		fmt.Fprintf(os.Stderr,"[IOBUS]   ❌ No driver registered for scheme: %s\n", scheme)
		result := &types.ResourceResult{
			StatusCode: 404,
			Body:       []byte(fmt.Sprintf(`{"error":"no driver for scheme: %s"}`, scheme)),
		}
		b.logResult(result, nil)
		return result, nil
	}

	fmt.Fprintf(os.Stderr,"[IOBUS]   → Routing to: %s (scheme: %s)\n", driver.Name(), scheme)

	// Call driver
	result, err := driver.HandleCall(ctx, call)
	b.logResult(result, err)
	return result, err
}

// logResult logs the response from a driver call
func (b *MemoryIOBus) logResult(result *types.ResourceResult, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr,"[IOBUS]   ❌ Driver Error: %v\n", err)
	}

	if result != nil {
		statusIcon := "✓"
		if result.StatusCode >= 400 {
			statusIcon = "❌"
		} else if result.StatusCode >= 300 {
			statusIcon = "⚠️"
		}

		fmt.Fprintf(os.Stderr,"[IOBUS] %s Response\n", statusIcon)
		fmt.Fprintf(os.Stderr,"[IOBUS]   Status: %d\n", result.StatusCode)

		if len(result.Body) > 0 {
			if result.StatusCode >= 200 && result.StatusCode < 300 {
				fmt.Fprintf(os.Stderr,"[IOBUS]   Body: %d bytes\n", len(result.Body))
			} else {
				// Show error bodies
				bodyStr := string(result.Body)
				if len(bodyStr) > 200 {
					bodyStr = bodyStr[:200] + "..."
				}
				fmt.Fprintf(os.Stderr,"[IOBUS]   Body: %s\n", bodyStr)
			}
		}

		if len(result.Headers) > 0 {
			fmt.Fprintf(os.Stderr,"[IOBUS]   Response Headers: %d\n", len(result.Headers))
		}
	}

	fmt.Fprintf(os.Stderr,"[IOBUS] ════════════════════════════════════════════════════════\n\n")
}

// RegisterDriver registers a driver for a URI scheme
func (b *MemoryIOBus) RegisterDriver(driver types.ResourceDriver) error {
	// Extract schemes from driver patterns
	patterns := driver.Patterns()
	if len(patterns) == 0 {
		return fmt.Errorf("driver %s has no patterns", driver.Name())
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Register driver for each scheme in patterns
	for _, pattern := range patterns {
		// Extract scheme from pattern (e.g., "fn://**" -> "fn")
		schemeEnd := strings.Index(pattern, "://")
		if schemeEnd == -1 {
			return fmt.Errorf("invalid pattern %s: missing scheme", pattern)
		}
		scheme := pattern[:schemeEnd]

		// Register driver for this scheme
		b.drivers[scheme] = driver
		fmt.Fprintf(os.Stderr,"[IOBUS] ✓ Registered driver: %s → scheme '%s'\n", driver.Name(), scheme)
	}

	return nil
}
