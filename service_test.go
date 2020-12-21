package hraftd_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/bingoohuang/hraftd"
)

// Test_NewServer tests that a server can perform all basic operations.
func Test_NewServer(t *testing.T) {
	store := newTestStore()
	arg := &hraftd.Arg{
		RaftAddr: ":0",
		HTTPAddr: ":0",
	}
	s := &testServer{&hraftd.Service{Arg: arg, Store: store}}

	if err := s.StartAll(); err != nil {
		t.Fatalf("failed to start HTTP service: %s", err)
	}

	b := doGet(t, s.URL(), "k1")
	if b != `{"k1":""}` {
		t.Fatalf("wrong value received for key k1: %s (expected empty string)", b)
	}

	doPost(t, s.URL(), "k1", "v1")

	b = doGet(t, s.URL(), "k1")

	if b != `{"k1":"v1"}` {
		t.Fatalf(`wrong value received for key k1: %s (expected "v1")`, b)
	}

	store.m["k2"] = "v2"
	b = doGet(t, s.URL(), "k2")

	if b != `{"k2":"v2"}` {
		t.Fatalf(`wrong value received for key k2: %s (expected "v2")`, b)
	}

	doDelete(t, s.URL(), "k2")
	b = doGet(t, s.URL(), "k2")

	if b != `{"k2":""}` {
		t.Fatalf(`wrong value received for key k2: %s (expected empty string)`, b)
	}
}

type testServer struct {
	*hraftd.Service
}

func (t *testServer) URL() string {
	port := strings.TrimLeft(t.Addr().String(), "[:]")
	return fmt.Sprintf("http://127.0.0.1:%s", port)
}

type testStore struct {
	m map[string]string
}

func newTestStore() *testStore { return &testStore{m: make(map[string]string)} }

func (t *testStore) RaftStats() map[string]interface{}             { return map[string]interface{}{} }
func (t *testStore) Cluster() (hraftd.RaftCluster, error)          { return hraftd.RaftCluster{}, nil }
func (t *testStore) LeadServer() (hraftd.Peer, error)              { return hraftd.Peer{}, nil }
func (t *testStore) WaitForLeader(_ time.Duration) (string, error) { return "", nil }
func (t *testStore) WaitForApplied(_ time.Duration) error          { return nil }
func (t *testStore) LeaderCh() <-chan bool                         { return nil }
func (t *testStore) Get(key string) (string, bool)                 { return t.m[key], true }
func (t *testStore) Set(key, value string) error                   { t.m[key] = value; return nil }
func (t *testStore) Delete(key string) error                       { delete(t.m, key); return nil }
func (t *testStore) Join(nodeID, addr string) error                { return nil }
func (t *testStore) Remove(nodeID string) error                    { return nil }
func (t *testStore) IsLeader() bool                                { return true }
func (t *testStore) NodeState() string                             { return "" }

func doGet(t *testing.T, url, key string) string {
	resp, err := http.Get(url + hraftd.KeyPath + "/" + key) // nolint:noctx
	if err != nil {
		t.Fatalf("failed to GET key: %s", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response: %s", err)
	}

	return string(body)
}

func doPost(t *testing.T, url, key, value string) {
	// nolint:noctx
	resp, err := http.Post(url+hraftd.KeyPath, hraftd.ContentTypeJSON,
		bytes.NewReader(hraftd.JsonifyBytes(map[string]string{key: value})))
	if err != nil {
		t.Fatalf("POST request failed: %s", err)
	}

	resp.Body.Close()
}

func doDelete(t *testing.T, u, key string) {
	ru, err := url.Parse(u + hraftd.KeyPath + "/" + key)
	if err != nil {
		t.Fatalf("failed to parse URL for delete: %s", err)
	}

	client := http.Client{}
	resp, err := client.Do(&http.Request{Method: "DELETE", URL: ru})
	if err != nil {
		t.Fatalf("failed to GET key: %s", err)
	}

	resp.Body.Close()
}
