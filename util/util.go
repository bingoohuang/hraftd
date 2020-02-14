package util

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	hjson "github.com/hjson/hjson-go"
)

// EmptyThen returns t if s is empty
func EmptyThen(s, t string) string {
	if s == "" {
		return t
	}

	return s
}

// EqualsThen returns t if s equals to e
func EqualsThen(s, e, t string) string {
	if s == e {
		return t
	}

	return s
}

// If returns s if i else t
func If(i bool, s, t string) string {
	if i {
		return s
	}

	return t
}

// CloneMap clones a map
func CloneMap(m map[string]string) map[string]string {
	o := make(map[string]string)

	for k, v := range m {
		o[k] = v
	}

	return o
}

// WriteAsText writes s as text/plain
func WriteAsText(s string, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(s))
}

// WriteAsJSON writes m as JSON
func WriteAsJSON(m interface{}, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	b, _ := json.Marshal(m)
	_, _ = w.Write(b)
}

// ReadString ...
func ReadString(object io.ReadCloser) string {
	return string(ReadBytes(object))
}

// ReadBytes ...
func ReadBytes(object io.ReadCloser) []byte {
	defer object.Close()

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(object)

	return buf.Bytes()
}

// PathExists returns true if the given path exists.
func PathExists(p string) bool {
	if _, err := os.Lstat(p); err != nil && os.IsNotExist(err) {
		return false
	}

	return true
}

// Hjson returns HJSON presentation of v
func Hjson(v interface{}) string {
	hj, _ := hjson.Marshal(v)

	return strings.ReplaceAll(string(hj), "\n", "")
}

// Ticker defines a ticker.
type Ticker struct {
	stop     chan bool
	tickerFn func()
	d        time.Duration
}

// NewTicker creates a new ticker.
func NewTicker(d time.Duration, tickerFn func()) *Ticker {
	return &Ticker{stop: make(chan bool, 1), tickerFn: tickerFn, d: d}
}

// Start starts the ticker.
func (j *Ticker) Start() {
	t := time.NewTicker(j.d)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			j.tickerFn()
		case <-j.stop:
			return
		}
	}
}

// Stop stops the ticker.
func (j *Ticker) Stop() {
	j.stop <- true
}

const dfmt = "2006-01-02 15:04:05.000"

// FormatTime format time.
// FormatTime format time.
func FormatTime(t time.Time) string {
	return t.Format(dfmt)
}

// ParseTime parses time.
func ParseTime(s string) (time.Time, error) {
	return time.Parse(dfmt, s)
}
