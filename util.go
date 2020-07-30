package hraftd

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
)

// EmptyThen returns t if s is empty.
func EmptyThen(s, t string) string {
	if s == "" {
		return t
	}

	return s
}

// EqualsThen returns t if s equals to e.
func EqualsThen(s, e, t string) string {
	if s == e {
		return t
	}

	return s
}

// If returns s if i else t.
func If(i bool, s, t string) string {
	if i {
		return s
	}

	return t
}

// CloneMap clones a map.
func CloneMap(m map[string]string) map[string]string {
	o := make(map[string]string)

	for k, v := range m {
		o[k] = v
	}

	return o
}

// WriteAsText writes s as text/plain.
func WriteAsText(s string, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(s))
}

// WriteAsJSON writes m as JSON.
func WriteAsJSON(m interface{}, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	_, _ = w.Write(JsonifyBytes(m))
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

// Jsonify jsonifies v to JSON string.
func Jsonify(v interface{}) string {
	return string(JsonifyBytes(v))
}

// Jsonify4Print jsonifies v to JSON string for printing only.
func Jsonify4Print(v interface{}) string {
	all := strings.ReplaceAll(Jsonify(v), `\"`, ``)
	return strings.ReplaceAll(all, `"`, ``)
}

// JsonifyBytes jsonifies v to JSON []byte.
func JsonifyBytes(v interface{}) []byte {
	b, _ := jsoniter.Marshal(v)
	return b
}

// TryParseJSON try parse v as JSON，return parsed object or failed to source v.
func TryParseJSON(v string) interface{} {
	if jv, err := ParseJSON(v); err == nil {
		return jv
	}

	return v
}

// ParseJSON parses string to v.
func ParseJSON(v string) (m interface{}, err error) {
	err = jsoniter.Unmarshal([]byte(v), &m)

	return
}

// WaitInterrupt waits on interrupt signal.
func WaitInterrupt() {
	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	<-terminate
}
