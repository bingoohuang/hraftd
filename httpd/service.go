// Package httpd provides the HTTP server for accessing the distributed key-value store.
// It also provides the endpoint for other nodes to join an existing cluster.
package httpd

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/bingoohuang/hraftd/util"
)

// Store is the interface Raft-backed key-value stores must implement.
type Store interface {
	// Get returns the value for the given key.
	Get(key string) (string, bool, error)

	// Set sets the value for the given key, via distributed consensus.
	Set(key, value string) error

	// Delete removes the given key, via distributed consensus.
	Delete(key string) error

	// Join joins the node, identified by nodeID and reachable at addr, to the cluster.
	Join(nodeID string, addr string) error

	// RaftStats returns the raft stats
	RaftStats() map[string]string
}

// Service provides HTTP service.
type Service struct {
	addr string
	ln   net.Listener

	store Store
}

// New returns an uninitialized HTTP service.
func New(addr string, store Store) *Service { return &Service{addr: addr, store: store} }

// Start starts the service.
func (s *Service) Start() error {
	server := http.Server{Handler: s}

	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}

	s.ln = ln

	http.Handle("/", s)

	go func() {
		if err := server.Serve(s.ln); err != nil {
			log.Fatalf("HTTP serve: %s", err)
		}
	}()

	return nil
}

// Close closes the service.
func (s *Service) Close() error { return s.ln.Close() }

// ServeHTTP allows Service to serve HTTP requests.
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasPrefix(r.URL.Path, "/key"):
		s.handleKeyRequest(w, r)
	case r.URL.Path == "/join":
		s.handleJoin(w, r)
	case r.URL.Path == "/raft/stats":
		s.handleRaftStats(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// JoinRequest defines the Raft join request
type JoinRequest struct {
	RemoteAddr string `json:"addr"`
	NodeID     string `json:"id"`
}

type JoinResponse struct {
	OK  bool   `json:"ok"`
	Msg string `json:"msg"`
}

func (s *Service) handleJoin(w http.ResponseWriter, r *http.Request) {
	var m JoinRequest
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		util.WriteAsJSON(JoinResponse{OK: false, Msg: err.Error()}, w)
		return
	}

	if err := s.store.Join(m.NodeID, m.RemoteAddr); err != nil {
		util.WriteAsJSON(JoinResponse{OK: false, Msg: err.Error()}, w)
		return
	}

	util.WriteAsJSON(JoinResponse{OK: true, Msg: "OK"}, w)
}

func (s *Service) handleKeyRequest(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		if key, ok := getKey(r, w); ok {
			s.doGet(key, w)
		}
	case "POST":
		s.doPost(r, w)
	case "DELETE":
		if key, ok := getKey(r, w); ok {
			s.doDelete(key, w)
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func getKey(r *http.Request, w http.ResponseWriter) (string, bool) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 3 { // nolint gomnd
		w.WriteHeader(http.StatusBadRequest)
		return "", false
	}

	return parts[2], true
}

func (s *Service) doDelete(k string, w http.ResponseWriter) {
	if err := s.store.Delete(k); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (s *Service) doPost(r *http.Request, w http.ResponseWriter) {
	// Read the value from the POST body.
	m := map[string]string{}
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	for k, v := range m {
		if err := s.store.Set(k, v); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

func (s *Service) doGet(k string, w http.ResponseWriter) {
	v, ok, err := s.store.Get(k)

	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	util.WriteAsJSON(map[string]string{k: v}, w)
}

// Addr returns the address on which the Service is listening
func (s *Service) Addr() net.Addr { return s.ln.Addr() }

func (s *Service) handleRaftStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)

		return
	}

	util.WriteAsJSON(s.store.RaftStats(), w)
}
