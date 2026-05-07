package base

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewConfigBuilder(t *testing.T) {
	builder := NewConfigBuilder("test/driver")
	assert.NotNil(t, builder)

	config := builder.Build()
	assert.Equal(t, "test/driver", config.Name)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 100*time.Millisecond, config.RetryBackoff)
	assert.Equal(t, LogLevelInfo, config.LogLevel)
}

func TestConfigBuilder_WithPatterns(t *testing.T) {
	config := NewConfigBuilder("test/driver").
		WithPatterns("http://*", "https://*").
		Build()

	assert.Equal(t, []string{"http://*", "https://*"}, config.Patterns)
}

func TestConfigBuilder_WithTimeout(t *testing.T) {
	config := NewConfigBuilder("test/driver").
		WithTimeout(10 * time.Second).
		Build()

	assert.Equal(t, 10*time.Second, config.Timeout)
}

func TestConfigBuilder_WithMaxRetries(t *testing.T) {
	config := NewConfigBuilder("test/driver").
		WithMaxRetries(5).
		Build()

	assert.Equal(t, 5, config.MaxRetries)
}

func TestConfigBuilder_WithRetryBackoff(t *testing.T) {
	config := NewConfigBuilder("test/driver").
		WithRetryBackoff(500 * time.Millisecond).
		Build()

	assert.Equal(t, 500*time.Millisecond, config.RetryBackoff)
}

func TestConfigBuilder_WithLogLevel(t *testing.T) {
	config := NewConfigBuilder("test/driver").
		WithLogLevel(LogLevelDebug).
		Build()

	assert.Equal(t, LogLevelDebug, config.LogLevel)
}

func TestConfigBuilder_Chaining(t *testing.T) {
	config := NewConfigBuilder("test/driver").
		WithPatterns("test://*").
		WithTimeout(5 * time.Second).
		WithMaxRetries(10).
		WithRetryBackoff(200 * time.Millisecond).
		WithLogLevel(LogLevelWarn).
		Build()

	assert.Equal(t, "test/driver", config.Name)
	assert.Equal(t, []string{"test://*"}, config.Patterns)
	assert.Equal(t, 5*time.Second, config.Timeout)
	assert.Equal(t, 10, config.MaxRetries)
	assert.Equal(t, 200*time.Millisecond, config.RetryBackoff)
	assert.Equal(t, LogLevelWarn, config.LogLevel)
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig("test/driver", "test://*", "test2://*")

	assert.Equal(t, "test/driver", config.Name)
	assert.Equal(t, []string{"test://*", "test2://*"}, config.Patterns)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 100*time.Millisecond, config.RetryBackoff)
	assert.Equal(t, LogLevelInfo, config.LogLevel)
}

func TestDefaultConfig_NoPatterns(t *testing.T) {
	config := DefaultConfig("test/driver")

	assert.Equal(t, "test/driver", config.Name)
	assert.Empty(t, config.Patterns)
}
