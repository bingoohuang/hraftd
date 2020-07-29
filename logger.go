package hraftd

import (
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	"sync"
	"time"

	"github.com/spf13/viper"
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
var DefaultLogger = NewSLogger()

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
}

// LevelLogger ...
type LevelLogger interface {
	// Printf prints info
	Printf(format string, data ...interface{})
	// Debugf prints debug
	Debugf(format string, data ...interface{})
	// Infof prints info
	Infof(format string, data ...interface{})
	// Warnf prints warn messages
	Warnf(format string, data ...interface{})
	// Errorf prints error messages
	Errorf(format string, data ...interface{})
	// Panicf prints error messages and panics
	Panicf(format string, data ...interface{})
}

// LoggerMore ...
type LoggerMore interface {
	Logger
	LevelLogger
}

// LevelLoggerAdapter adapters Logger to LevelLogger
type LevelLoggerAdapter struct{ Logger }

// Debugf prints debug
func (l LevelLoggerAdapter) Debugf(format string, data ...interface{}) {
	l.Logf(LogLevelDebug, format, data...)
}

// Printf prints info
func (l LevelLoggerAdapter) Printf(format string, data ...interface{}) {
	l.Logf(LogLevelInfo, format, data...)
}

// Infof prints info
func (l LevelLoggerAdapter) Infof(format string, data ...interface{}) {
	l.Logf(LogLevelInfo, format, data...)
}

// Warnf prints warn messages
func (l LevelLoggerAdapter) Warnf(format string, data ...interface{}) {
	l.Logf(LogLevelWarn, format, data...)
}

// Errorf prints error messages
func (l LevelLoggerAdapter) Errorf(format string, data ...interface{}) {
	l.Logf(LogLevelError, format, data...)
}

// Panicf prints error messages and panic
func (l LevelLoggerAdapter) Panicf(format string, data ...interface{}) {
	l.Logf(LogLevelError, format, data...)
	panic(fmt.Sprintf(format, data...))
}

// Writer log writer interface
type Writer interface {
	Print(...interface{})
}

// SLogger defines the simplest logger implementation of Interface
type SLogger struct {
	Writer
	IOWriter io.Writer
	Level    LogLevel
	LevelLogger
}

// NewSLogger creates a new SLogger
func NewSLogger() *SLogger {
	k := &SLogger{
		Writer:   NewStdLogger(os.Stdout),
		IOWriter: os.Stdout,
		Level:    LogLevelInfo,
	}

	k.LevelLogger = &LevelLoggerAdapter{Logger: k}

	return k
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

// A StdLogger represents an active logging object that generates lines of
// output to an io.Writer. Each logging operation makes a single call to
// the Writer's Write method. A Logger can be used simultaneously from
// multiple goroutines; it guarantees to serialize access to the Writer.
type StdLogger struct {
	mu  sync.Mutex // ensures atomic writes; protects the following fields
	buf []byte     // for accumulating text to write
	Out io.Writer  // destination for output

	CallDepth       int  // used for print logger file and line number
	PrintCallerInfo bool // switcher to print caller info
}

// NewStdLogger creates a new Logger. The out variable sets the
// destination to which log data will be written.
// The prefix appears at the beginning of each generated log line.
// The flag argument defines the logging properties.
func NewStdLogger(out io.Writer) *StdLogger {
	// nolint:gomnd
	return &StdLogger{Out: out, CallDepth: 4, PrintCallerInfo: viper.GetBool("PrintCallerInfo")}
}

// Cheap integer to fixed-width decimal ASCII. Give a negative width to avoid zero-padding.
// nolint:gomnd
func itoa(buf *[]byte, i int, wid int) {
	// Assemble decimal in reverse order.
	var b [20]byte
	bp := len(b) - 1

	for i >= 10 || wid > 1 {
		wid--
		q := i / 10
		b[bp] = byte('0' + i - q*10)
		bp--
		i = q
	}
	// i < 10
	b[bp] = byte('0' + i)
	*buf = append(*buf, b[bp:]...)
}

// formatHeader writes log header to buf in following order:
//   * l.prefix (if it's not blank),
//   * date and/or time (if corresponding flags are provided),
//   * file and line number (if corresponding flags are provided).
// nolint:gomnd
func (l *StdLogger) formatHeader(buf *[]byte, t time.Time, file string, line int) {
	year, month, day := t.Date()
	itoa(buf, year, 4)
	*buf = append(*buf, '-')
	itoa(buf, int(month), 2)
	*buf = append(*buf, '-')
	itoa(buf, day, 2)
	*buf = append(*buf, ' ')

	hour, min, sec := t.Clock()
	itoa(buf, hour, 2)
	*buf = append(*buf, ':')
	itoa(buf, min, 2)
	*buf = append(*buf, ':')
	itoa(buf, sec, 2)

	*buf = append(*buf, '.')
	itoa(buf, t.Nanosecond()/1e6, 3)

	*buf = append(*buf, ' ')

	if l.PrintCallerInfo {
		short := file

		for i := len(file) - 1; i > 0; i-- {
			if file[i] == '/' {
				short = file[i+1:]
				break
			}
		}

		file = short

		*buf = append(*buf, file...)
		*buf = append(*buf, ':')
		itoa(buf, line, -1)
		*buf = append(*buf, " "...)
	}
}

// Output writes the output for a logging event. The string s contains
// the text to print after the prefix specified by the flags of the
// Logger. A newline is appended if the last character of s is not
// already a newline. CallDepth is used to recover the PC and is
// provided for generality, although at the moment on all pre-defined
// paths it will be 2.
func (l *StdLogger) Output(s string) error {
	now := time.Now() // get this early.

	file := ""
	line := 0

	if l.PrintCallerInfo {
		ok := false

		// getting caller info - it's expensive.
		_, file, line, ok = runtime.Caller(l.CallDepth)
		// funcName := "???" path.Base(runtime.FuncForPC(pc).Name())

		if ok {
			file = path.Base(file)
		} else {
			file = "???"
		}
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.buf = l.buf[:0]
	l.formatHeader(&l.buf, now, file, line)
	l.buf = append(l.buf, s...)

	if len(s) == 0 || s[len(s)-1] != '\n' {
		l.buf = append(l.buf, '\n')
	}

	_, err := l.Out.Write(l.buf)

	return err
}

// Print calls l.Output to print to the logger.
// Arguments are handled in the manner of fmt.Print.
func (l *StdLogger) Print(v ...interface{}) { _ = l.Output(fmt.Sprint(v...)) }
