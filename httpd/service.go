// Package httpd provides the HTTP server for accessing the distributed key-value store.
// It also provides the endpoint for other nodes to Join an existing cluster.
package httpd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/bingoohuang/gonet"
	"github.com/bingoohuang/hraftd/store"

	"github.com/bingoohuang/hraftd/model"

	"github.com/bingoohuang/hraftd/util"
)

// Service provides HTTP service.
type Service struct {
	store model.Store

	ln net.Listener

	Arg *model.Arg
}

// Create returns an uninitialized HTTP service.
func Create(arg *model.Arg) *Service {
	s := store.New(arg)

	if err := s.Open(); err != nil {
		log.Fatalf("failed to open store: %s", err.Error())
	}

	return &Service{Arg: arg, store: s}
}

// Start starts the service.
func (s *Service) Start() error {
	server := http.Server{Handler: s}

	ln, err := net.Listen("tcp", s.Arg.HTTPAddr)
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

	// If Join was specified, make the Join request.
	if !s.Arg.Bootstrap {
		return nil
	}

	if err := Join(s.Arg.JoinAddr, s.Arg.RaftAddr, s.Arg.NodeID); err != nil {
		log.Fatalf("failed to Join node at %s: %s", s.Arg.JoinAddr, err.Error())
	}

	return nil
}

// Join joins the current not to raft cluster
func Join(joinAddr, raftAddr, nodeID string) error {
	b, _ := json.Marshal(model.JoinRequest{RemoteAddr: raftAddr, NodeID: nodeID})
	joinURL := fmt.Sprintf("http://%s/raft/join", joinAddr)

	for i := 0; i < 10; i++ {
		if i > 0 {
			time.Sleep(10 * time.Second) // nolint gomnd
		}

		resp, err := http.Post(joinURL, model.ContentTypeJSON, bytes.NewReader(b)) // nolint gosec
		if err != nil {
			log.Printf("joined error %v, retry after 10s\n", err)

			continue
		}

		var r model.JoinResponse

		rs := gonet.ReadString(resp.Body)
		resp.Body.Close()
		log.Printf("json response %s\n", rs)
		_ = json.Unmarshal([]byte(rs), &r)

		if r.OK {
			return nil
		}
	}

	return fmt.Errorf("failed to Join %s", joinURL)
}

// Close closes the service.
func (s *Service) Close() error { return s.ln.Close() }

// ServeHTTP allows Service to serve HTTP requests.
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case strings.HasPrefix(path, "/key"):
		s.handleKeyRequest(w, r)
	case path == "/raft/join":
		CheckMethod("POST", s.handleJoin, w, r)
	case path == "/raft/stats":
		CheckMethod("GET", func(w http.ResponseWriter, r *http.Request) {
			util.WriteAsJSON(s.store.RaftStats(), w)
		}, w, r)
	case path == "/raft/state":
		CheckMethod("GET", func(w http.ResponseWriter, r *http.Request) {
			util.WriteAsJSON(s.store.Status(), w)
		}, w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (s *Service) handleJoin(w http.ResponseWriter, r *http.Request) {
	var m model.JoinRequest
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		util.WriteAsJSON(model.JoinResponse{OK: false, Msg: err.Error()}, w)
		return
	}

	if err := s.store.Join(m.NodeID, m.RemoteAddr); err != nil {
		util.WriteAsJSON(model.JoinResponse{OK: false, Msg: err.Error()}, w)
		return
	}

	util.WriteAsJSON(model.JoinResponse{OK: true, Msg: "OK"}, w)
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

func CheckMethod(m string, f func(w http.ResponseWriter, r *http.Request), w http.ResponseWriter, r *http.Request) {
	if r.Method != m {
		w.WriteHeader(http.StatusMethodNotAllowed)

		return
	}

	f(w, r)
}
