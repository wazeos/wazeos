package bus

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/wazeos/wazeos/internal/types"
)

// driverRegistration holds a driver and its URI pattern
type driverRegistration struct {
	driver  types.ResourceDriver
	pattern string
}

// parsedURI represents a URI broken into components
type parsedURI struct {
	scheme string
	host   string
	path   string
}

// MemoryIOBus implements io.bus interface using in-memory state
// This is a stateful, native Go implementation for routing resource calls
type MemoryIOBus struct {
	drivers      []driverRegistration // all registered drivers with patterns
	secretsStore types.ResourceDriver // special case for secrets
	mu           sync.RWMutex
}

// NewMemoryIOBus creates a new in-memory IO bus
func NewMemoryIOBus(secretsStore types.ResourceDriver) *MemoryIOBus {
	return &MemoryIOBus{
		drivers:      make([]driverRegistration, 0),
		secretsStore: secretsStore,
	}
}

// parseURI parses a URI into scheme, host, and path components
func parseURI(uri string) (*parsedURI, error) {
	// Extract scheme
	schemeEnd := strings.Index(uri, "://")
	if schemeEnd == -1 {
		return nil, fmt.Errorf("invalid URI: missing scheme")
	}

	scheme := uri[:schemeEnd]
	rest := uri[schemeEnd+3:] // skip "://"

	// Split host and path
	pathStart := strings.Index(rest, "/")
	var host, path string

	if pathStart == -1 {
		// No path, only host (or empty)
		host = rest
		path = "/"
	} else {
		host = rest[:pathStart]
		path = rest[pathStart:]
	}

	return &parsedURI{
		scheme: scheme,
		host:   host,
		path:   path,
	}, nil
}

// matchesPattern checks if a URI matches a pattern and returns a specificity score
// Higher score = more specific match (fewer wildcards)
// Returns -1 if no match
func matchesPattern(uri *parsedURI, pattern *parsedURI) int {
	// Scheme must match exactly
	if uri.scheme != pattern.scheme {
		return -1
	}

	score := 0

	// Host matching
	if pattern.host == "*" {
		// Full wildcard - lowest score
		score += 0
	} else if strings.HasPrefix(pattern.host, "*.") {
		// Prefix wildcard like *.example.com
		suffix := pattern.host[2:] // remove "*."
		if !strings.HasSuffix(uri.host, suffix) {
			return -1 // no match
		}
		score += 50
	} else {
		// Exact host match
		if uri.host != pattern.host {
			return -1 // no match
		}
		score += 100
	}

	// Path matching
	if pattern.path == "/*" || pattern.path == "*" {
		// Full wildcard - lowest score
		score += 0
	} else if strings.HasSuffix(pattern.path, "/*") {
		// Suffix wildcard like /path/to/*
		prefix := pattern.path[:len(pattern.path)-2] // remove "/*"
		if !strings.HasPrefix(uri.path, prefix) {
			return -1 // no match
		}
		// Score based on depth of prefix
		segments := strings.Count(prefix, "/")
		score += segments * 10
	} else {
		// Exact path match
		if uri.path != pattern.path {
			return -1 // no match
		}
		// Score based on path depth
		segments := strings.Count(pattern.path, "/")
		score += segments * 10 + 100
	}

	return score
}

// findBestDriver finds the best matching driver for a URI
func (b *MemoryIOBus) findBestDriver(uri string) (types.ResourceDriver, error) {
	parsedURI, err := parseURI(uri)
	if err != nil {
		return nil, err
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	bestScore := -1
	var bestDriver types.ResourceDriver
	var bestPattern string

	// Check all registered drivers
	for _, reg := range b.drivers {
		patternURI, err := parseURI(reg.pattern)
		if err != nil {
			continue // skip invalid patterns
		}

		score := matchesPattern(parsedURI, patternURI)
		if score > bestScore {
			bestScore = score
			bestDriver = reg.driver
			bestPattern = reg.pattern
		}
	}

	if bestDriver == nil {
		return nil, fmt.Errorf("no driver found for URI: %s", uri)
	}

	fmt.Fprintf(os.Stderr,"[IOBUS]   → Routing to: %s (pattern: %s, score: %d)\n",
		bestDriver.Name(), bestPattern, bestScore)

	return bestDriver, nil
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

	// Find best matching driver for URI
	driver, err := b.findBestDriver(call.URI)
	if err != nil {
		fmt.Fprintf(os.Stderr,"[IOBUS]   ❌ %s\n", err.Error())
		result := &types.ResourceResult{
			StatusCode: 404,
			Body:       []byte(fmt.Sprintf(`{"error":"%s"}`, err.Error())),
		}
		b.logResult(result, nil)
		return result, nil
	}

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

// RegisterDriver registers a driver with its URI patterns
func (b *MemoryIOBus) RegisterDriver(driver types.ResourceDriver) error {
	patterns := driver.Patterns()
	if len(patterns) == 0 {
		return fmt.Errorf("driver %s has no patterns", driver.Name())
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Register driver for each pattern
	for _, pattern := range patterns {
		// Validate pattern has a scheme
		schemeEnd := strings.Index(pattern, "://")
		if schemeEnd == -1 {
			return fmt.Errorf("invalid pattern %s: missing scheme", pattern)
		}

		// Add driver registration
		b.drivers = append(b.drivers, driverRegistration{
			driver:  driver,
			pattern: pattern,
		})

		fmt.Fprintf(os.Stderr,"[IOBUS] ✓ Registered driver: %s → pattern '%s'\n", driver.Name(), pattern)
	}

	return nil
}
