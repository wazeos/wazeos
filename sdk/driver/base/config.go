package base

import (
	"time"
)

// ConfigBuilder provides a fluent interface for building driver configurations
type ConfigBuilder struct {
	config DriverConfig
}

// NewConfigBuilder creates a new configuration builder with sensible defaults
func NewConfigBuilder(name string) *ConfigBuilder {
	return &ConfigBuilder{
		config: DriverConfig{
			Name:         name,
			Patterns:     []string{},
			Timeout:      30 * time.Second,
			MaxRetries:   3,
			RetryBackoff: 100 * time.Millisecond,
			LogLevel:     LogLevelInfo,
		},
	}
}

// WithPatterns sets the URI patterns this driver handles
func (b *ConfigBuilder) WithPatterns(patterns ...string) *ConfigBuilder {
	b.config.Patterns = patterns
	return b
}

// WithTimeout sets the operation timeout
func (b *ConfigBuilder) WithTimeout(timeout time.Duration) *ConfigBuilder {
	b.config.Timeout = timeout
	return b
}

// WithMaxRetries sets the maximum number of retry attempts
func (b *ConfigBuilder) WithMaxRetries(maxRetries int) *ConfigBuilder {
	b.config.MaxRetries = maxRetries
	return b
}

// WithRetryBackoff sets the backoff duration between retries
func (b *ConfigBuilder) WithRetryBackoff(backoff time.Duration) *ConfigBuilder {
	b.config.RetryBackoff = backoff
	return b
}

// WithLogLevel sets the logging verbosity level
func (b *ConfigBuilder) WithLogLevel(level LogLevel) *ConfigBuilder {
	b.config.LogLevel = level
	return b
}

// Build returns the final configuration
func (b *ConfigBuilder) Build() DriverConfig {
	return b.config
}

// DefaultConfig returns a default configuration for a driver
func DefaultConfig(name string, patterns ...string) DriverConfig {
	return NewConfigBuilder(name).
		WithPatterns(patterns...).
		Build()
}
