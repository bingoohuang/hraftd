package hraftd

import (
	"bytes"
	"encoding/json"
	"net/http"
)

// GetJSON does HTTP GET parse response as JSON
func GetJSON(addr string, v interface{}) (string, error) {
	resp, err := http.Get(addr) // nolint:gosec
	if err != nil {
		return "", err
	}

	return parseRSP(resp, v)
}

// PostJSON posts body as JSON and parse response as JSON
func PostJSON(addr string, body, v interface{}) (int, string, error) {
	resp, err := http.Post(addr, ContentTypeJSON, bytes.NewReader(JsonifyBytes(body))) // nolint:gosec
	if err != nil {
		return 0, "", err
	}

	rsp, err := parseRSP(resp, v)

	return resp.StatusCode, rsp, err
}

func parseRSP(resp *http.Response, v interface{}) (string, error) {
	rs := ReadString(resp.Body)
	_ = resp.Body.Close()

	if v == nil {
		return rs, nil
	}

	return rs, json.Unmarshal([]byte(rs), v)
}

// ContentTypeJSON is the JSON Content-Type.
const ContentTypeJSON = "application-type/json"
