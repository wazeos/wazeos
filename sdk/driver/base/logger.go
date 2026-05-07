package base

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// LogLevel represents logging verbosity levels
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger provides structured logging for drivers
type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})

	// WithPrefix returns a new logger with an additional prefix
	WithPrefix(prefix string) Logger
}

// StdLogger implements Logger using standard output with prefixes
type StdLogger struct {
	prefix string
	level  LogLevel
	writer io.Writer
	mu     sync.Mutex
}

// NewLogger creates a new standard logger
func NewLogger(prefix string, level LogLevel) Logger {
	return &StdLogger{
		prefix: prefix,
		level:  level,
		writer: os.Stderr,
	}
}

// NewLoggerWithWriter creates a new logger with a custom writer
func NewLoggerWithWriter(prefix string, level LogLevel, writer io.Writer) Logger {
	return &StdLogger{
		prefix: prefix,
		level:  level,
		writer: writer,
	}
}

func (l *StdLogger) log(level LogLevel, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("15:04:05")
	message := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.writer, "[%s:%s:%s] %s\n", timestamp, l.prefix, level.String(), message)
}

func (l *StdLogger) Debug(format string, args ...interface{}) {
	l.log(LogLevelDebug, format, args...)
}

func (l *StdLogger) Info(format string, args ...interface{}) {
	l.log(LogLevelInfo, format, args...)
}

func (l *StdLogger) Warn(format string, args ...interface{}) {
	l.log(LogLevelWarn, format, args...)
}

func (l *StdLogger) Error(format string, args ...interface{}) {
	l.log(LogLevelError, format, args...)
}

func (l *StdLogger) WithPrefix(prefix string) Logger {
	return &StdLogger{
		prefix: fmt.Sprintf("%s:%s", l.prefix, prefix),
		level:  l.level,
		writer: l.writer,
	}
}

// NoopLogger discards all log messages (useful for testing)
type NoopLogger struct{}

func NewNoopLogger() Logger {
	return &NoopLogger{}
}

func (l *NoopLogger) Debug(format string, args ...interface{}) {}
func (l *NoopLogger) Info(format string, args ...interface{})  {}
func (l *NoopLogger) Warn(format string, args ...interface{})  {}
func (l *NoopLogger) Error(format string, args ...interface{}) {}
func (l *NoopLogger) WithPrefix(prefix string) Logger          { return l }
