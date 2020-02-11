package util

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	hjson "github.com/hjson/hjson-go"
)

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
