package kernel

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wazeos/wazeos/internal/types"
)

// ResourceBus implements types.ResourceBus.
type ResourceBus struct {
	drivers      []driverEntry
	auditDrivers []types.AuditDriver
}

type driverEntry struct {
	driver   types.ResourceDriver
	patterns []uriPattern
}

type uriPattern struct {
	original  string
	scheme    string
	authority string
	path      string
}

// NewResourceBus creates a new resource bus.
func NewResourceBus() *ResourceBus {
	return &ResourceBus{
		drivers: make([]driverEntry, 0),
	}
}

// RegisterDriver adds a resource driver to the bus.
func (rb *ResourceBus) RegisterDriver(driver types.ResourceDriver) error {
	if driver == nil {
		return fmt.Errorf("driver cannot be nil")
	}

	patterns := driver.Patterns()
	if len(patterns) == 0 {
		return fmt.Errorf("driver %q has no patterns", driver.Name())
	}

	// Check for duplicate driver names
	for _, existing := range rb.drivers {
		if existing.driver.Name() == driver.Name() {
			return fmt.Errorf("resource driver %q already registered", driver.Name())
		}
	}

	// Parse patterns
	parsedPatterns := make([]uriPattern, 0, len(patterns))
	for _, pattern := range patterns {
		parsed, err := parsePattern(pattern)
		if err != nil {
			return fmt.Errorf("invalid pattern %q for driver %q: %w", pattern, driver.Name(), err)
		}
		parsedPatterns = append(parsedPatterns, parsed)
	}

	rb.drivers = append(rb.drivers, driverEntry{
		driver:   driver,
		patterns: parsedPatterns,
	})

	return nil
}

// Call routes a resource call to the appropriate driver.
func (rb *ResourceBus) Call(ctx context.Context, call *types.ResourceCall) (*types.ResourceResult, error) {
	if call == nil {
		return nil, types.ErrInvalidRequest
	}

	// Find matching driver with highest specificity
	driver := rb.findDriver(call.URI)
	if driver == nil {
		result := &types.ResourceResult{
			StatusCode: 404,
			Headers:    make(map[string]string),
			Body:       []byte(fmt.Sprintf("no driver found for URI: %s", call.URI)),
			Error:      types.ErrNotFound,
		}

		// Emit audit event for failed call
		rb.emitResourceCallAudit(call, result, "", 0)

		return result, types.ErrNotFound
	}

	// Delegate to driver and measure duration
	startTime := time.Now()
	result, err := driver.HandleCall(ctx, call)
	duration := time.Since(startTime).Nanoseconds()

	// Emit audit event
	rb.emitResourceCallAudit(call, result, driver.Name(), duration)

	return result, err
}

// findDriver finds the best matching driver for a URI.
// Uses specificity precedence: authority > path > scheme.
func (rb *ResourceBus) findDriver(uri string) types.ResourceDriver {
	parsed, err := url.Parse(uri)
	if err != nil {
		return nil
	}

	var bestMatch *driverEntry
	var bestScore int

	for i := range rb.drivers {
		entry := &rb.drivers[i]
		for _, pattern := range entry.patterns {
			score := matchScore(parsed, pattern)
			if score > bestScore {
				bestScore = score
				bestMatch = entry
			}
		}
	}

	if bestMatch != nil {
		return bestMatch.driver
	}

	return nil
}

// parsePattern parses a URI pattern into components.
func parsePattern(pattern string) (uriPattern, error) {
	parsed, err := url.Parse(pattern)
	if err != nil {
		return uriPattern{}, err
	}

	return uriPattern{
		original:  pattern,
		scheme:    parsed.Scheme,
		authority: parsed.Host,
		path:      parsed.Path,
	}, nil
}

// matchScore calculates specificity score for a URI against a pattern.
// Returns 0 if no match, higher scores for more specific matches.
// Specificity: authority (1000) > path specificity (1-999) > scheme (1)
func matchScore(uri *url.URL, pattern uriPattern) int {
	score := 0

	// Scheme must match exactly (or pattern has no scheme requirement)
	if pattern.scheme != "" && pattern.scheme != "*" {
		if uri.Scheme != pattern.scheme {
			return 0
		}
		score += 1
	}

	// Authority matching - exact or glob match
	if pattern.authority != "" && pattern.authority != "*" {
		if !matchGlob(uri.Host, pattern.authority) {
			return 0
		}
		// Exact authority match is very specific
		score += 1000
	} else if pattern.authority == "*" {
		// Wildcard authority still counts but is less specific
		score += 1
	}

	// Path matching with specificity calculation
	if pattern.path != "" && pattern.path != "*" {
		if !matchGlob(uri.Path, pattern.path) {
			return 0
		}
		// Calculate path specificity: more specific paths get higher scores
		// Count non-wildcard segments and characters
		pathSpecificity := calculatePathSpecificity(pattern.path)
		score += pathSpecificity
	} else if pattern.path == "*" {
		// Wildcard path matches but adds minimal score
		score += 1
	}

	return score
}

// calculatePathSpecificity calculates how specific a path pattern is.
// More specific patterns (longer, fewer wildcards) get higher scores.
func calculatePathSpecificity(path string) int {
	if path == "*" || path == "/*" {
		return 1
	}

	// Count non-wildcard characters as specificity
	specificity := 0
	segments := strings.Split(strings.Trim(path, "/"), "/")

	for _, segment := range segments {
		if segment == "*" {
			// Wildcard segment adds minimal specificity
			specificity += 1
		} else if strings.Contains(segment, "*") {
			// Partial wildcard - medium specificity
			specificity += len(segment) * 2
		} else {
			// Exact segment - high specificity
			specificity += len(segment) * 10
		}
	}

	// Add bonus for number of segments (depth)
	specificity += len(segments) * 5

	// Cap at 999 to stay below authority score
	if specificity > 999 {
		specificity = 999
	}

	return specificity
}

// matchGlob performs simple glob matching with * wildcard.
func matchGlob(s, pattern string) bool {
	if pattern == "*" {
		return true
	}

	// No wildcard - exact match
	if !strings.Contains(pattern, "*") {
		return s == pattern
	}

	// Split pattern by *
	parts := strings.Split(pattern, "*")

	// Check prefix
	if parts[0] != "" && !strings.HasPrefix(s, parts[0]) {
		return false
	}

	// Check suffix
	if parts[len(parts)-1] != "" && !strings.HasSuffix(s, parts[len(parts)-1]) {
		return false
	}

	// Check middle parts
	pos := len(parts[0])
	for i := 1; i < len(parts)-1; i++ {
		if parts[i] == "" {
			continue
		}
		idx := strings.Index(s[pos:], parts[i])
		if idx == -1 {
			return false
		}
		pos += idx + len(parts[i])
	}

	return true
}

// RegisterAuditDriver adds an audit driver to the resource bus.
func (rb *ResourceBus) RegisterAuditDriver(driver types.AuditDriver) {
	if driver == nil {
		return
	}
	rb.auditDrivers = append(rb.auditDrivers, driver)
}

// emitResourceCallAudit emits an audit event for a resource call.
func (rb *ResourceBus) emitResourceCallAudit(call *types.ResourceCall, result *types.ResourceResult, driverName string, duration int64) {
	if len(rb.auditDrivers) == 0 {
		return
	}

	// Build audit event
	event := &types.ResourceCallAuditEvent{
		AuditEvent: types.AuditEvent{
			ID:        uuid.New().String(),
			Timestamp: time.Now(),
			Type:      types.AuditEventResourceCall,
			Success:   result != nil && result.Error == nil,
		},
		URI:        call.URI,
		Method:     call.Method,
		Driver:     driverName,
		Duration:   duration,
	}

	if call.Context != nil {
		event.Principal = call.Context.Principal
		event.RequestID = call.Context.RequestID
		event.TraceID = call.Context.TraceID
	}

	if result != nil {
		event.StatusCode = result.StatusCode
		if result.Error != nil {
			event.Error = result.Error.Error()
		}
	}

	// Emit to all audit drivers (async, don't block on errors)
	for _, auditDriver := range rb.auditDrivers {
		go func(driver types.AuditDriver) {
			ctx := context.Background()
			_ = driver.RecordResourceCall(ctx, event)
		}(auditDriver)
	}
}
