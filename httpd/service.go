// Package httpd provides the HTTP server for accessing the distributed key-value Store.
// It also provides the endpoint for other nodes to Join an existing cluster.
package httpd

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/bingoohuang/hraftd/store"

	"github.com/bingoohuang/hraftd/model"

	"github.com/bingoohuang/hraftd/util"
)

// Service provides HTTP service.
type Service struct {
	Store model.Store
	Ln    net.Listener
	Arg   *model.Arg
}

// Create returns an uninitialized HTTP service.
func Create(arg *model.Arg) *Service {
	s := store.New(arg)

	if err := s.Open(); err != nil {
		log.Fatalf("failed to open Store: %s", err.Error())
	}

	return &Service{Arg: arg, Store: s}
}

// Start starts the service.
func (s *Service) Start() (err error) {
	if s.Ln, err = net.Listen("tcp", s.Arg.HTTPAddr); err != nil {
		return err
	}

	go func() {
		http.Handle("/", s)

		server := http.Server{Handler: s}
		if err := server.Serve(s.Ln); err != nil {
			log.Fatalf("HTTP serve: %s\n", err)
		}
	}()

	leader, _ := s.Store.WaitForLeader(10 * time.Second) // nolint gomnd

	if leader == "" {
		if err := s.Arg.Join(); err != nil {
			log.Fatalf("failed to join at %s: %s\n", s.Arg.JoinAddrs, err.Error())
		}
	}

	return nil
}

// Close closes the service.
func (s *Service) Close() error { return s.Ln.Close() }

// Addr returns the address on which the Service is listening
func (s *Service) Addr() net.Addr { return s.Ln.Addr() }

// ServeHTTP allows Service to serve HTTP requests.
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	log.Printf("received request [%s] %s for %s\n", r.Method, path, r.RemoteAddr)

	switch {
	case strings.HasPrefix(path, "/key"):
		s.handleKeyRequest(w, r)
	case strings.HasPrefix(path, "/raft"):
		s.handleRaftRequest(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (s *Service) handleRaftRequest(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/raft/health":
		CheckMethod("GET", func(w http.ResponseWriter, _ *http.Request) {
			util.WriteAsText("OK", w)
		}, w, r)
	case "/raft/join":
		CheckMethodE("POST", s.tryForwardToLeaderFn(s.handleJoin), w, r)
	case "/raft/remove":
		CheckMethodE("DELETE", s.tryForwardToLeaderFn(s.handleRemove), w, r)
	case "/raft/stats":
		CheckMethod("GET", func(w http.ResponseWriter, _ *http.Request) {
			util.WriteAsJSON(s.Store.RaftStats(), w)
		}, w, r)
	case "/raft/state":
		CheckMethod("GET", func(w http.ResponseWriter, _ *http.Request) {
			util.WriteAsJSON(model.Rsp{OK: true, Msg: s.Store.NodeState()}, w)
		}, w, r)
	case "/raft/cluster":
		CheckMethodE("GET", s.handleCluster, w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (s *Service) handleKeyRequest(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		if key, ok := getKey(r, w); ok {
			if v, ok := s.Store.Get(key); ok {
				util.WriteAsJSON(map[string]string{key: v}, w)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}
	case "POST":
		s.tryForwardToLeader(s.doPost, w, r)
	case "DELETE":
		if key, ok := getKey(r, w); ok {
			s.tryForwardToLeader(func(w http.ResponseWriter, r *http.Request) {
				if err := s.Store.Delete(key); err != nil {
					w.WriteHeader(http.StatusInternalServerError)
				}
			}, w, r)
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Service) handleJoin(w http.ResponseWriter, r *http.Request) error {
	var m model.JoinRequest
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		return err
	}

	m.Fix(util.EmptyThen(r.Header.Get(util.XOriginRemoteAddr), r.RemoteAddr))

	log.Printf("received join request for remote node %s at %s\n", m.NodeID, m.Addr)

	if err := s.Store.Join(string(m.NodeID), m.Addr); err != nil {
		return err
	}

	log.Printf("node %s at %s joined successfully\n", m.NodeID, m.Addr)

	util.WriteAsJSON(model.Rsp{OK: true, Msg: "OK"}, w)

	return nil
}

// handleRemove handles cluster-remove requests.
func (s *Service) handleRemove(w http.ResponseWriter, r *http.Request) error {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	m := map[string]string{}
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	remoteID, ok := m["id"]
	if !ok {
		return errors.New("ID not found")
	}

	if err := s.Store.Remove(remoteID); err != nil {
		return err
	}

	util.WriteAsJSON(model.Rsp{OK: true, Msg: "OK"}, w)

	return nil
}

func getKey(r *http.Request, w http.ResponseWriter) (string, bool) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 3 { // nolint gomnd
		w.WriteHeader(http.StatusBadRequest)
		return "", false
	}

	return parts[2], true
}

func (s *Service) doPost(w http.ResponseWriter, r *http.Request) {
	// Read the value from the POST body.
	m := map[string]string{}
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	for k, v := range m {
		if err := s.Store.Set(k, v); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

// ServeHTTPFn defines ServeHTTP function prototype.
type ServeHTTPFn func(w http.ResponseWriter, r *http.Request)

// ServeHTTPFnE defines ServeHTTP function prototype.
type ServeHTTPFnE func(w http.ResponseWriter, r *http.Request) error

// CheckMethod checks the method and invoke f.
func CheckMethod(m string, f ServeHTTPFn, w http.ResponseWriter, r *http.Request) {
	if r.Method == m {
		f(w, r)
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// CheckMethodE checks the method and invoke f.
func CheckMethodE(m string, f ServeHTTPFnE, w http.ResponseWriter, r *http.Request) {
	if r.Method != m {
		w.WriteHeader(http.StatusMethodNotAllowed)

		return
	}

	if err := f(w, r); err != nil {
		util.WriteAsJSON(model.Rsp{Msg: err.Error()}, w)
	}
}

func (s *Service) tryForwardToLeaderFn(f ServeHTTPFnE) ServeHTTPFnE {
	if s.Store.IsLeader() {
		return f
	}

	return s.forwardToLeader
}

func (s *Service) tryForwardToLeader(f ServeHTTPFn, w http.ResponseWriter, r *http.Request) {
	if s.Store.IsLeader() {
		f(w, r)
	} else if err := s.forwardToLeader(w, r); err != nil {
		util.WriteAsJSON(model.Rsp{Msg: err.Error()}, w)
	}
}

func (s *Service) forwardToLeader(w http.ResponseWriter, r *http.Request) error {
	leader, err := s.Store.LeadServer()
	if err != nil {
		return err
	}

	addr := leader.NodeID.HTTPAddr()
	if addr == "" {
		return errors.New("failed to get raft leader")
	}

	log.Printf("forward %s to leader %s\n", r.URL.String(), addr)

	if xor := r.Header.Get(util.XOriginRemoteAddr); xor != "" {
		return errors.New("forward two times not allowed")
	}

	util.ReverseProxy(addr, r.URL.Path, 10*time.Second).ServeHTTP(w, r) // nolint gomnd

	return nil
}

func (s *Service) handleCluster(w http.ResponseWriter, _ *http.Request) error {
	servers, err := s.Store.Cluster()
	if err != nil {
		return err
	}

	util.WriteAsJSON(servers, w)

	return nil
}
