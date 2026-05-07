package base

import (
	"fmt"
	"sync"
	"time"

	"github.com/wazeos/wazeos/internal/types"
)

// DriverConfig holds common configuration for drivers
type DriverConfig struct {
	Name         string
	Patterns     []string
	Timeout      time.Duration
	MaxRetries   int
	RetryBackoff time.Duration
	LogLevel     LogLevel
}

// BaseDriver provides common functionality for all drivers
type BaseDriver struct {
	name     string
	patterns []string
	config   DriverConfig
	logger   Logger

	// State management
	mu      sync.RWMutex
	started bool

	// For request drivers that need invocation handlers
	invoker types.InvocationHandler
}

// NewBaseDriver creates a new base driver with the given configuration
func NewBaseDriver(config DriverConfig) *BaseDriver {
	logger := NewLogger(config.Name, config.LogLevel)

	return &BaseDriver{
		name:     config.Name,
		patterns: config.Patterns,
		config:   config,
		logger:   logger,
		started:  false,
	}
}

// Name returns the driver name (implements types.ResourceDriver)
func (b *BaseDriver) Name() string {
	return b.name
}

// Patterns returns URI patterns this driver handles (implements types.ResourceDriver)
func (b *BaseDriver) Patterns() []string {
	return b.patterns
}

// Logger returns the driver's logger
func (b *BaseDriver) Logger() Logger {
	return b.logger
}

// Config returns the driver's configuration
func (b *BaseDriver) Config() DriverConfig {
	return b.config
}

// SetInvoker sets the invocation handler (for request drivers)
func (b *BaseDriver) SetInvoker(invoker types.InvocationHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.invoker = invoker
	b.logger.Debug("invoker set")
}

// GetInvoker returns the current invocation handler
func (b *BaseDriver) GetInvoker() types.InvocationHandler {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.invoker
}

// MarkStarted marks the driver as started and validates preconditions
func (b *BaseDriver) MarkStarted() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.started {
		return fmt.Errorf("driver %s already started", b.name)
	}

	b.started = true
	b.logger.Info("driver started")
	return nil
}

// MarkStopped marks the driver as stopped
func (b *BaseDriver) MarkStopped() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.started = false
	b.logger.Info("driver stopped")
}

// IsStarted returns whether the driver is currently started
func (b *BaseDriver) IsStarted() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.started
}

// ValidateInvoker checks if an invoker is set (for request drivers)
func (b *BaseDriver) ValidateInvoker() error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.invoker == nil {
		return fmt.Errorf("invoker not set for driver %s", b.name)
	}

	return nil
}
