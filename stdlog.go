package hraftd

import (
	"bytes"
	"strings"
)

// Provides a io.Writer to shim the data out of *log.Logger
// and back into our Logger. This is basically the only way to
// build upon *log.Logger.
type stdlogAdapter struct {
	log         Logger
	inferLevels bool
}

// Take the data, infer the levels if configured, and send it through
// a regular Logger.
func (s *stdlogAdapter) Write(data []byte) (int, error) {
	str := string(bytes.TrimRight(data, " \t\n"))

	level, str := s.pickLevel(str)
	s.log.Logf(level, str)

	return len(data), nil
}

// Detect, based on conventions, what log level this is.
func (s *stdlogAdapter) pickLevel(str string) (LogLevel, string) {
	switch {
	case strings.HasPrefix(str, "[DEBUG]"):
		return LogLevelDebug, strings.TrimSpace(str[7:])
	case strings.HasPrefix(str, "[TRACE]"):
		return LogLevelDebug, strings.TrimSpace(str[7:])
	case strings.HasPrefix(str, "[INFO]"):
		return LogLevelInfo, strings.TrimSpace(str[6:])
	case strings.HasPrefix(str, "[WARN]"):
		return LogLevelWarn, strings.TrimSpace(str[7:])
	case strings.HasPrefix(str, "[ERROR]"):
		return LogLevelError, strings.TrimSpace(str[7:])
	case strings.HasPrefix(str, "[ERR]"):
		return LogLevelError, strings.TrimSpace(str[5:])
	default:
		return LogLevelInfo, str
	}
}
