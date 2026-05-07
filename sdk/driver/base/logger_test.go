package base

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLogger(t *testing.T) {
	logger := NewLogger("test", LogLevelInfo)
	assert.NotNil(t, logger)
}

func TestStdLogger_LogLevels(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter("TEST", LogLevelDebug, &buf)

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()
	assert.Contains(t, output, "DEBUG")
	assert.Contains(t, output, "INFO")
	assert.Contains(t, output, "WARN")
	assert.Contains(t, output, "ERROR")
	assert.Contains(t, output, "TEST")
}

func TestStdLogger_LogLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter("TEST", LogLevelWarn, &buf)

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()
	assert.NotContains(t, output, "DEBUG")
	assert.NotContains(t, output, "INFO")
	assert.Contains(t, output, "WARN")
	assert.Contains(t, output, "ERROR")
}

func TestStdLogger_WithPrefix(t *testing.T) {
	var buf bytes.Buffer
	baseLogger := NewLoggerWithWriter("BASE", LogLevelInfo, &buf)
	childLogger := baseLogger.WithPrefix("CHILD")

	childLogger.Info("test message")

	output := buf.String()
	assert.Contains(t, output, "BASE:CHILD")
	assert.Contains(t, output, "test message")
}

func TestStdLogger_FormatArgs(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter("TEST", LogLevelInfo, &buf)

	logger.Info("message with args: %s %d", "test", 123)

	output := buf.String()
	assert.Contains(t, output, "message with args: test 123")
}

func TestNoopLogger(t *testing.T) {
	logger := NewNoopLogger()

	// Should not panic
	logger.Debug("test")
	logger.Info("test")
	logger.Warn("test")
	logger.Error("test")

	child := logger.WithPrefix("child")
	child.Info("test")
}

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{LogLevelDebug, "DEBUG"},
		{LogLevelInfo, "INFO"},
		{LogLevelWarn, "WARN"},
		{LogLevelError, "ERROR"},
		{LogLevel(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.level.String())
		})
	}
}

func TestStdLogger_ConcurrentAccess(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter("TEST", LogLevelInfo, &buf)

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			logger.Info("concurrent message %d", id)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Equal(t, 10, len(lines), "should have 10 log lines")
}
