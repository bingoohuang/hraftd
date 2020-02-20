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
	Debugf(format string, data ...interface{})
	Infof(format string, data ...interface{})
	Warnf(format string, data ...interface{})
	Errorf(format string, data ...interface{})
	Panicf(format string, data ...interface{})
}

// LoggerAdapter ...
type LoggerAdapter struct {
	Logger
}

// Debugf prints debug
func (l LoggerAdapter) Debugf(format string, data ...interface{}) {
	l.Logf(LogLevelDebug, format, data...)
}

// Printf prints info
func (l LoggerAdapter) Printf(format string, data ...interface{}) {
	l.Logf(LogLevelInfo, format, data...)
}

// Infof prints info
func (l LoggerAdapter) Infof(format string, data ...interface{}) {
	l.Logf(LogLevelInfo, format, data...)
}

// Warnf prints warn messages
func (l LoggerAdapter) Warnf(format string, data ...interface{}) {
	l.Logf(LogLevelWarn, format, data...)
}

// Errorf prints error messages
func (l LoggerAdapter) Errorf(format string, data ...interface{}) {
	l.Logf(LogLevelError, format, data...)
}

// Logger defines logger interface
type Logger interface {
	// SetLogLevel sets the log level
	SetLogLevel(logLevel LogLevel)
	// GetLogLevel returns the log level
	GetLogLevel() LogLevel
	// GetIOWriter returns io.Writer
	GetIOWriter() io.Writer
	// Logf prints log
	Logf(level LogLevel, format string, data ...interface{})

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

// Logf prints log
func (l SLogger) Logf(level LogLevel, format string, data ...interface{}) {
	if l.Level <= level {
		l.Print("[" + level.String() + "] " + fmt.Sprintf(format, data...))
	}
}

// Panicf prints error messages and panics
func (l SLogger) Panicf(format string, data ...interface{}) {
	l.Logf(LogLevelError, format, data...)

	panic(fmt.Sprintf(format, data...))
}
