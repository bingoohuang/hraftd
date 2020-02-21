package hraftd

import (
	"io"

	"github.com/sirupsen/logrus"
)

// LogrusAdapter adapts the logrus to Logger.
type LogrusAdapter struct {
	Logrus *logrus.Logger
}

// NewLogrusAdapter news a LogrusAdapter
func NewLogrusAdapter(logrus *logrus.Logger) *LevelLoggerAdapter {
	l := &LogrusAdapter{Logrus: logrus}

	return &LevelLoggerAdapter{Logger: l}
}

// SetLogLevel sets the log level
func (l *LogrusAdapter) SetLogLevel(logLevel LogLevel) {
	level, err := logrus.ParseLevel(logLevel.String())
	if err != nil {
		level = logrus.InfoLevel
	}

	l.Logrus.SetLevel(level)
}

// GetLogLevel returns the log level
func (l LogrusAdapter) GetLogLevel() LogLevel {
	return ParseLogLevel(l.Logrus.Level.String())
}

// GetIOWriter returns io.Writer
func (l LogrusAdapter) GetIOWriter() io.Writer {
	return l.Logrus.Writer()
}

// Logf prints log
func (l LogrusAdapter) Logf(logLevel LogLevel, format string, data ...interface{}) {
	level, err := logrus.ParseLevel(logLevel.String())
	if err != nil {
		level = logrus.InfoLevel
	}

	l.Logrus.Logf(level, format, data...)
}
