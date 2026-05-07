package bus

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wazeos/wazeos/internal/types"
)

// Bus implements a generalized I/O bus for routing resource calls to drivers.
// It supports both simple scheme-based routing and complex URI pattern matching.
type Bus struct {
	drivers      []driverEntry
	secretsStore types.ResourceDriver // Special case for secrets://
	auditDrivers []types.AuditDriver
	logger       Logger
	mu           sync.RWMutex
}

// Logger defines the logging interface for the bus.
type Logger interface {
	LogIncomingCall(call *types.ResourceCall)
	LogRoutingDecision(driver types.ResourceDriver, uri string)
	LogResult(result *types.ResourceResult, err error)
	LogError(message string, args ...interface{})
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

// Config configures the I/O bus.
type Config struct {
	Logger       Logger                   // Optional logger (defaults to no-op)
	SecretsStore types.ResourceDriver     // Optional secrets store for secret:// URIs
}

// New creates a new I/O bus.
func New(cfg *Config) *Bus {
	if cfg == nil {
		cfg = &Config{}
	}

	logger := cfg.Logger
	if logger == nil {
		logger = &noopLogger{}
	}

	return &Bus{
		drivers:      make([]driverEntry, 0),
		secretsStore: cfg.SecretsStore,
		logger:       logger,
	}
}

// RegisterDriver adds a resource driver to the bus.
// The driver's Patterns() method defines which URIs it handles.
func (b *Bus) RegisterDriver(driver types.ResourceDriver) error {
	if driver == nil {
		return fmt.Errorf("driver cannot be nil")
	}

	patterns := driver.Patterns()
	if len(patterns) == 0 {
		return fmt.Errorf("driver %q has no patterns", driver.Name())
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

	b.mu.Lock()
	defer b.mu.Unlock()

	b.drivers = append(b.drivers, driverEntry{
		driver:   driver,
		patterns: parsedPatterns,
	})

	return nil
}

// Call routes a resource call to the appropriate driver.
func (b *Bus) Call(ctx context.Context, call *types.ResourceCall) (*types.ResourceResult, error) {
	if call == nil {
		return nil, types.ErrInvalidRequest
	}

	// Log incoming request
	b.logger.LogIncomingCall(call)

	// Special case: secrets store
	if b.secretsStore != nil && strings.HasPrefix(call.URI, "secret://") {
		b.logger.LogRoutingDecision(b.secretsStore, call.URI)
		result, err := b.secretsStore.HandleCall(ctx, call)
		b.logger.LogResult(result, err)
		return result, err
	}

	// Find matching driver with highest specificity
	driver := b.findDriver(call.URI)
	if driver == nil {
		errMsg := fmt.Sprintf("no driver found for URI: %s", call.URI)
		result := &types.ResourceResult{
			StatusCode: 404,
			Headers:    make(map[string]string),
			Body:       []byte(errMsg),
			Error:      errMsg,
		}
		b.logger.LogError("No driver found for URI: %s", call.URI)
		b.logger.LogResult(result, nil)
		return result, types.ErrNotFound
	}

	b.logger.LogRoutingDecision(driver, call.URI)

	// Route to driver and measure duration
	startTime := time.Now()
	result, err := driver.HandleCall(ctx, call)
	duration := time.Since(startTime).Nanoseconds()

	// Emit audit event
	b.emitResourceCallAudit(call, result, driver.Name(), duration)

	b.logger.LogResult(result, err)
	return result, err
}

// findDriver finds the best matching driver for a URI.
// Returns the driver with the highest specificity score.
func (b *Bus) findDriver(uri string) types.ResourceDriver {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var bestDriver types.ResourceDriver
	var bestScore int

	for _, entry := range b.drivers {
		for _, pattern := range entry.patterns {
			if score := matchScore(uri, pattern); score > bestScore {
				bestScore = score
				bestDriver = entry.driver
			}
		}
	}

	return bestDriver
}

// matchScore calculates how well a URI matches a pattern.
// Higher scores indicate better matches.
// Returns 0 if there's no match.
func matchScore(uri string, pattern uriPattern) int {
	// Parse URI
	parsedURI, err := url.Parse(uri)
	if err != nil {
		return 0
	}

	score := 0

	// Scheme matching (required)
	if pattern.scheme != "*" && parsedURI.Scheme != pattern.scheme {
		return 0
	}
	if parsedURI.Scheme == pattern.scheme {
		score += 1000
	}

	// Authority matching
	if pattern.authority != "" && pattern.authority != "*" {
		if parsedURI.Host != pattern.authority {
			return 0
		}
		score += 500
	}

	// Path matching
	if pattern.path != "" {
		pathScore := calculatePathSpecificity(parsedURI.Path, pattern.path)
		if pathScore == 0 {
			return 0
		}
		score += pathScore
	}

	return score
}

// calculatePathSpecificity calculates specificity for path matching.
func calculatePathSpecificity(uriPath, patternPath string) int {
	// Exact match
	if uriPath == patternPath {
		return 100
	}

	// Wildcard matching
	if strings.HasSuffix(patternPath, "/*") {
		prefix := strings.TrimSuffix(patternPath, "/*")
		if strings.HasPrefix(uriPath, prefix) {
			// More specific prefixes get higher scores
			return 50 + len(prefix)
		}
	}

	// Glob pattern matching
	if matchGlob(uriPath, patternPath) {
		return 25
	}

	return 0
}

// matchGlob performs simple glob matching.
func matchGlob(s, pattern string) bool {
	if pattern == "*" {
		return true
	}

	if !strings.Contains(pattern, "*") {
		return s == pattern
	}

	parts := strings.Split(pattern, "*")
	if len(parts) == 2 {
		return strings.HasPrefix(s, parts[0]) && strings.HasSuffix(s, parts[1])
	}

	// More complex patterns
	pos := 0
	for i, part := range parts {
		if part == "" {
			continue
		}
		idx := strings.Index(s[pos:], part)
		if idx == -1 {
			return false
		}
		if i == 0 && idx != 0 {
			return false
		}
		pos += idx + len(part)
	}

	if parts[len(parts)-1] != "" && !strings.HasSuffix(s, parts[len(parts)-1]) {
		return false
	}

	return true
}

// parsePattern parses a URI pattern into components.
func parsePattern(pattern string) (uriPattern, error) {
	// Handle wildcard pattern
	if pattern == "*" {
		return uriPattern{
			original:  pattern,
			scheme:    "*",
			authority: "*",
			path:      "*",
		}, nil
	}

	// Parse as URL
	u, err := url.Parse(pattern)
	if err != nil {
		return uriPattern{}, fmt.Errorf("failed to parse pattern: %w", err)
	}

	return uriPattern{
		original:  pattern,
		scheme:    u.Scheme,
		authority: u.Host,
		path:      u.Path,
	}, nil
}

// noopLogger is a no-op logger implementation.
type noopLogger struct{}

func (l *noopLogger) LogIncomingCall(call *types.ResourceCall)                   {}
func (l *noopLogger) LogRoutingDecision(driver types.ResourceDriver, uri string) {}
func (l *noopLogger) LogResult(result *types.ResourceResult, err error)          {}
func (l *noopLogger) LogError(message string, args ...interface{})               {}

// StderrLogger logs to stderr with configurable prefix.
type StderrLogger struct {
	Prefix string
	Writer io.Writer
}

func (l *StderrLogger) LogIncomingCall(call *types.ResourceCall) {
	prefix := l.Prefix
	if prefix == "" {
		prefix = "[IOBUS]"
	}
	w := l.Writer
	if w == nil {
		w = io.Discard
	}

	fmt.Fprintf(w, "\n%s ════════════════════════════════════════════════════════\n", prefix)
	fmt.Fprintf(w, "%s Incoming Request\n", prefix)
	fmt.Fprintf(w, "%s   URI: %s\n", prefix, call.URI)
	fmt.Fprintf(w, "%s   Permissions: %v\n", prefix, call.Permissions)

	if len(call.Headers) > 0 {
		fmt.Fprintf(w, "%s   Headers:\n", prefix)
		for k, v := range call.Headers {
			if k == "X-WazeOS-Credentials" {
				fmt.Fprintf(w, "%s     %s: [INJECTED] (%d bytes)\n", prefix, k, len(v))
			} else {
				fmt.Fprintf(w, "%s     %s: %s\n", prefix, k, v)
			}
		}
	}

	if len(call.Body) > 0 {
		fmt.Fprintf(w, "%s   Body: %d bytes\n", prefix, len(call.Body))
	}
}

func (l *StderrLogger) LogRoutingDecision(driver types.ResourceDriver, uri string) {
	prefix := l.Prefix
	if prefix == "" {
		prefix = "[IOBUS]"
	}
	w := l.Writer
	if w == nil {
		w = io.Discard
	}

	fmt.Fprintf(w, "%s   → Routing to: %s\n", prefix, driver.Name())
}

func (l *StderrLogger) LogResult(result *types.ResourceResult, err error) {
	prefix := l.Prefix
	if prefix == "" {
		prefix = "[IOBUS]"
	}
	w := l.Writer
	if w == nil {
		w = io.Discard
	}

	if err != nil {
		fmt.Fprintf(w, "%s   ✗ Error: %v\n", prefix, err)
		return
	}

	if result == nil {
		fmt.Fprintf(w, "%s   ⚠ No result\n", prefix)
		return
	}

	symbol := "✓"
	if result.StatusCode >= 400 {
		symbol = "✗"
	}

	fmt.Fprintf(w, "%s   %s Status: %d\n", prefix, symbol, result.StatusCode)
	if len(result.Body) > 0 {
		fmt.Fprintf(w, "%s   Response: %d bytes\n", prefix, len(result.Body))
	}
}

func (l *StderrLogger) LogError(message string, args ...interface{}) {
	prefix := l.Prefix
	if prefix == "" {
		prefix = "[IOBUS]"
	}
	w := l.Writer
	if w == nil {
		w = io.Discard
	}

	fmt.Fprintf(w, "%s   ✗ ", prefix)
	fmt.Fprintf(w, message, args...)
	fmt.Fprintf(w, "\n")
}

// RegisterAuditDriver adds an audit driver to receive audit events.
func (b *Bus) RegisterAuditDriver(driver types.AuditDriver) {
	if driver == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.auditDrivers = append(b.auditDrivers, driver)
}

// emitResourceCallAudit emits an audit event for a resource call.
func (b *Bus) emitResourceCallAudit(call *types.ResourceCall, result *types.ResourceResult, driverName string, duration int64) {
	b.mu.RLock()
	drivers := b.auditDrivers
	b.mu.RUnlock()

	if len(drivers) == 0 {
		return
	}

	// Build audit event
	success := result != nil && result.Error == ""
	event := &types.ResourceCallAuditEvent{
		AuditEvent: types.AuditEvent{
			ID:        uuid.New().String(),
			Timestamp: time.Now(),
			Type:      types.AuditEventResourceCall,
			Success:   success,
		},
		URI:         call.URI,
		Permissions: call.Permissions,
		Driver:      driverName,
		Duration:    duration,
	}

	if call.Context != nil {
		event.Principal = call.Context.Principal
		event.RequestID = call.Context.RequestID
		event.TraceID = call.Context.TraceID
	}

	if result != nil {
		event.StatusCode = result.StatusCode
		event.Error = result.Error
	}

	// Emit to all audit drivers asynchronously
	for _, auditDriver := range drivers {
		go func(driver types.AuditDriver) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = driver.RecordResourceCall(ctx, event)
		}(auditDriver)
	}
}
