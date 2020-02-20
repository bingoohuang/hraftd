package hraftd

import (
	"fmt"
	"io"
	"log"
	"os"
)

// LogLevel defines log levels.
type LogLevel int

// ParseLogLevel parses log level.
func ParseLogLevel(l string) LogLevel {
	switch l {
	case "debug":
		return LogLevelDebug
	case "info":
		return LogLevelInfo
	case "warn":
		return LogLevelWarn
	case "error":
		return LogLevelError
	}

	return LogLevelInfo
}

func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "debug"
	case LogLevelInfo:
		return "info"
	case LogLevelWarn:
		return "warn"
	case LogLevelError:
		return "error"
	}

	return "unknown"
}

const (
	// LogLevelDebug debug log level
	LogLevelDebug LogLevel = iota
	// LogLevelInfo info log level
	LogLevelInfo
	// LogLevelWarn warn log level
	LogLevelWarn
	// LogLevelError error log level
	LogLevelError
)

// DefaultLogger is the default global logger
// nolint
var DefaultLogger = &SLogger{Writer: log.New(os.Stdout, "", log.LstdFlags), IOWriter: os.Stdout, Level: LogLevelInfo}

// LoggerMore ...
type LoggerMore interface {
	Printf(format string, data ...interface{})
	Debug(format string, data ...interface{})
	Info(format string, data ...interface{})
	Warn(format string, data ...interface{})
	Error(format string, data ...interface{})
	Panic(format string, data ...interface{})
}

// LoggerAdapter ...
type LoggerAdapter struct {
	Logger
}

// Debug prints debug
func (l LoggerAdapter) Debug(format string, data ...interface{}) {
	l.Log(LogLevelDebug, format, data...)
}

// Printf prints info
func (l LoggerAdapter) Printf(format string, data ...interface{}) {
	l.Log(LogLevelInfo, format, data...)
}

// Info prints info
func (l LoggerAdapter) Info(format string, data ...interface{}) {
	l.Log(LogLevelInfo, format, data...)
}

// Warn prints warn messages
func (l LoggerAdapter) Warn(format string, data ...interface{}) {
	l.Log(LogLevelWarn, format, data...)
}

// Error prints error messages
func (l LoggerAdapter) Error(format string, data ...interface{}) {
	l.Log(LogLevelError, format, data...)
}

// Logger defines logger interface
type Logger interface {
	// SetLogLevel sets the log level
	SetLogLevel(logLevel LogLevel)
	// GetLogLevel returns the log level
	GetLogLevel() LogLevel
	// GetIOWriter returns io.Writer
	GetIOWriter() io.Writer
	// Log prints log
	Log(level LogLevel, format string, data ...interface{})

	LoggerMore
}

// Writer log writer interface
type Writer interface {
	Print(...interface{})
}

// SLogger defines the simplest logger implementation of Interface
type SLogger struct {
	Writer
	LoggerAdapter
	IOWriter io.Writer
	Level    LogLevel
}

// GetIOWriter returns io.Writer
func (l *SLogger) GetIOWriter() io.Writer { return l.IOWriter }

// SetLogLevel sets the log level
func (l *SLogger) SetLogLevel(level LogLevel) { l.Level = level }

// GetLogLevel gets the log level
func (l SLogger) GetLogLevel() LogLevel { return l.Level }

// Log prints log
func (l SLogger) Log(level LogLevel, format string, data ...interface{}) {
	if l.Level <= level {
		l.Print("[" + level.String() + "] " + fmt.Sprintf(format, data...))
	}
}

// Panic prints error messages and panics
func (l SLogger) Panic(format string, data ...interface{}) {
	l.Log(LogLevelError, format, data...)

	panic(fmt.Sprintf(format, data...))
}
