package hraftd

import (
	"io"

	"github.com/sirupsen/logrus"
)

// LogrusAdapter adapts the logrus to Logger.
type LogrusAdapter struct {
	Logrus *logrus.Logger
}

// Printf prints info
func (l *LogrusAdapter) Printf(format string, data ...interface{}) { l.Logrus.Printf(format, data...) }

// Debugf prints debug
func (l *LogrusAdapter) Debugf(format string, data ...interface{}) { l.Logrus.Debugf(format, data...) }

// Infof prints info
func (l *LogrusAdapter) Infof(format string, data ...interface{}) { l.Logrus.Infof(format, data...) }

// Warnf prints warn messages
func (l *LogrusAdapter) Warnf(format string, data ...interface{}) { l.Logrus.Warnf(format, data...) }

// Errorf prints error messages
func (l *LogrusAdapter) Errorf(format string, data ...interface{}) { l.Logrus.Errorf(format, data...) }

// Panicf prints error messages and panics
func (l *LogrusAdapter) Panicf(format string, data ...interface{}) { l.Logrus.Panicf(format, data...) }

// NewLogrusAdapter news a LogrusAdapter
func NewLogrusAdapter(logrus *logrus.Logger) *LogrusAdapter {
	l := &LogrusAdapter{Logrus: logrus}

	return l
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
