package util

import (
	"encoding/json"
	"io"
)

// CloneMap clones a map
func CloneMap(m map[string]string) map[string]string {
	o := make(map[string]string)

	for k, v := range m {
		o[k] = v
	}

	return o
}

// WriteAsJSON writes m as JSON
func WriteAsJSON(m interface{}, w io.Writer) {
	b, _ := json.Marshal(m)
	_, _ = w.Write(b)
}
