package base

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewBaseDriver(t *testing.T) {
	config := DefaultConfig("test/driver", "test://*")
	driver := NewBaseDriver(config)

	assert.NotNil(t, driver)
	assert.Equal(t, "test/driver", driver.Name())
	assert.Equal(t, []string{"test://*"}, driver.Patterns())
	assert.NotNil(t, driver.Logger())
	assert.False(t, driver.IsStarted())
}

func TestBaseDriver_Name(t *testing.T) {
	config := DefaultConfig("mydriver", "scheme://*")
	driver := NewBaseDriver(config)

	assert.Equal(t, "mydriver", driver.Name())
}

func TestBaseDriver_Patterns(t *testing.T) {
	patterns := []string{"http://*", "https://*"}
	config := NewConfigBuilder("httpdriver").
		WithPatterns(patterns...).
		Build()
	driver := NewBaseDriver(config)

	assert.Equal(t, patterns, driver.Patterns())
}

func TestBaseDriver_SetInvoker(t *testing.T) {
	config := DefaultConfig("test/driver", "test://*")
	driver := NewBaseDriver(config)

	assert.Nil(t, driver.GetInvoker())

	// Set a mock invoker (nil is fine for this test)
	driver.SetInvoker(nil)
	assert.Nil(t, driver.GetInvoker())
}

func TestBaseDriver_MarkStarted(t *testing.T) {
	config := DefaultConfig("test/driver", "test://*")
	driver := NewBaseDriver(config)

	assert.False(t, driver.IsStarted())

	err := driver.MarkStarted()
	assert.NoError(t, err)
	assert.True(t, driver.IsStarted())

	// Second start should fail
	err = driver.MarkStarted()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already started")
}

func TestBaseDriver_MarkStopped(t *testing.T) {
	config := DefaultConfig("test/driver", "test://*")
	driver := NewBaseDriver(config)

	driver.MarkStarted()
	assert.True(t, driver.IsStarted())

	driver.MarkStopped()
	assert.False(t, driver.IsStarted())

	// Can start again after stop
	err := driver.MarkStarted()
	assert.NoError(t, err)
}

func TestBaseDriver_ValidateInvoker(t *testing.T) {
	config := DefaultConfig("test/driver", "test://*")
	driver := NewBaseDriver(config)

	// No invoker set
	err := driver.ValidateInvoker()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invoker not set")

	// Set invoker (nil for test purposes)
	driver.SetInvoker(nil)
	err = driver.ValidateInvoker()
	assert.Error(t, err) // Still error because invoker is nil
}

func TestBaseDriver_Config(t *testing.T) {
	config := NewConfigBuilder("test/driver").
		WithPatterns("test://*").
		WithTimeout(5 * time.Second).
		WithMaxRetries(5).
		Build()

	driver := NewBaseDriver(config)

	retrievedConfig := driver.Config()
	assert.Equal(t, "test/driver", retrievedConfig.Name)
	assert.Equal(t, 5*time.Second, retrievedConfig.Timeout)
	assert.Equal(t, 5, retrievedConfig.MaxRetries)
}

func TestBaseDriver_ConcurrentAccess(t *testing.T) {
	config := DefaultConfig("test/driver", "test://*")
	driver := NewBaseDriver(config)

	done := make(chan bool, 10)

	// Concurrent reads
	for i := 0; i < 5; i++ {
		go func() {
			_ = driver.IsStarted()
			_ = driver.Name()
			_ = driver.Patterns()
			done <- true
		}()
	}

	// Concurrent writes
	for i := 0; i < 5; i++ {
		go func() {
			driver.SetInvoker(nil)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
