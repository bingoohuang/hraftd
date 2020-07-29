package hraftd

import (
	"io/ioutil"
	"os"
	"testing"
	"time"
)

// Test_StoreOpen tests that the store can be opened.
func Test_StoreOpen(t *testing.T) {
	tmpDir, _ := ioutil.TempDir("", "store_test")
	defer os.RemoveAll(tmpDir)

	s := New(&Arg{
		RaftAddr:    "127.0.0.1:0",
		RaftNodeDir: tmpDir,
		NodeID:      "node0",
	})

	if err := s.Open(); err != nil {
		t.Fatalf("failed to open store: %s", err)
	}
}

// Test_StoreOpenSingleNode tests that a Command can be applied to the log
// nolint:gomnd
func Test_StoreOpenSingleNode(t *testing.T) {
	tmpDir, _ := ioutil.TempDir("", "store_test")
	defer os.RemoveAll(tmpDir)

	s := New(&Arg{
		RaftAddr:    "127.0.0.1:0",
		RaftNodeDir: tmpDir,
		NodeID:      "node0",
		Bootstrap:   true,
	})

	if err := s.Open(); err != nil {
		t.Fatalf("failed to open store: %s", err)
	}

	// Simple way to ensure there is a leader.
	time.Sleep(3 * time.Second)

	if err := s.Set("foo", "bar"); err != nil {
		t.Fatalf("failed to set key: %s", err.Error())
	}

	// Wait for committed log entry to be applied.
	time.Sleep(500 * time.Millisecond)
	value, _ := s.Get("foo")

	if value != "bar" {
		t.Fatalf("key has wrong value: %s", value)
	}

	if err := s.Delete("foo"); err != nil {
		t.Fatalf("failed to delete key: %s", err.Error())
	}

	// Wait for committed log entry to be applied.
	time.Sleep(500 * time.Millisecond)
	value, _ = s.Get("foo")

	if value != "" {
		t.Fatalf("key has wrong value: %s", value)
	}
}

// Test_StoreInMemOpenSingleNode tests that a Command can be applied to the log
// stored in RAM.
// nolint:gomnd
func Test_StoreInMemOpenSingleNode(t *testing.T) {
	tmpDir, _ := ioutil.TempDir("", "store_test")
	defer os.RemoveAll(tmpDir)

	s := New(&Arg{
		RaftAddr:    "127.0.0.1:0",
		RaftNodeDir: tmpDir,
		NodeID:      "node0",
		Bootstrap:   true,
	})

	if err := s.Open(); err != nil {
		t.Fatalf("failed to open store: %s", err)
	}

	// Simple way to ensure there is a leader.
	time.Sleep(3 * time.Second)

	if err := s.Set("foo", "bar"); err != nil {
		t.Fatalf("failed to set key: %s", err.Error())
	}

	// Wait for committed log entry to be applied.
	time.Sleep(500 * time.Millisecond)
	value, _ := s.Get("foo")
	if value != "bar" {
		t.Fatalf("key has wrong value: %s", value)
	}

	if err := s.Delete("foo"); err != nil {
		t.Fatalf("failed to delete key: %s", err.Error())
	}

	// Wait for committed log entry to be applied.
	time.Sleep(500 * time.Millisecond)
	value, _ = s.Get("foo")

	if value != "" {
		t.Fatalf("key has wrong value: %s", value)
	}
}
