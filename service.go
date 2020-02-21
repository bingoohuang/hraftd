package hraftd

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"time"
)

// Service provides HTTP service.
type Service struct {
	store Store
	Ln    net.Listener
	*Arg

	LeaderCh chan bool
	DealerMap
}

// Create returns an uninitialized service.
func Create(arg *Arg) *Service {
	s := New(arg)

	if err := s.Open(); err != nil {
		s.Panicf("failed to open Store: %s", err.Error())
	}

	return &Service{Arg: arg, store: s, DealerMap: MakeDealerMap(), LeaderCh: make(chan bool, 1)}
}

// StartAll starts the http and raft service.
func (s *Service) StartAll() error {
	if err := s.GoStartHTTP(); err != nil {
		return err
	}

	return s.StartRaft()
}

// StartRaft starts the raft service.
func (s *Service) StartRaft() error {
	go s.listenLeaderCh()

	go func() {
		for {
			err := s.Arg.Join()
			if err == nil {
				s.Infof("joined at %s error %s successfully", s.Arg.JoinAddrs)

				return
			}

			s.Errorf("failed to join at %s error %s, retry after 10s", s.Arg.JoinAddrs, err.Error())
			time.Sleep(10 * time.Second) // nolint gomnd
		}
	}()

	waitTimeout := 100 * time.Second // nolint gomnd
	_, _ = s.store.WaitForLeader(waitTimeout)
	_ = s.store.WaitForApplied(waitTimeout)

	return nil
}

// GoStartHTTP starts the http server in go routine.
func (s *Service) GoStartHTTP() (err error) {
	if s.Ln, err = net.Listen("tcp", s.Arg.ConvertToZeroHost(s.Arg.HTTPAddr)); err != nil {
		return err
	}

	go func() {
		http.Handle("/", s)

		server := http.Server{Handler: s}
		if err := server.Serve(s.Ln); err != nil {
			s.Panicf("HTTP serve: %s", err)
		}
	}()

	return nil
}

// Close closes the service.
func (s *Service) Close() error { return s.Ln.Close() }

// Addr returns the address on which the Service is listening
func (s *Service) Addr() net.Addr { return s.Ln.Addr() }

// ServeHTTP allows Service to serve HTTP requests.
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	s.Printf("received request [%s] %s for %s", r.Method, path, r.RemoteAddr)

	switch {
	case strings.HasPrefix(path, KeyPath):
		s.handleKeyRequest(w, r)
	case strings.HasPrefix(path, RaftPath):
		s.handleRaftRequest(w, r)
	case strings.HasPrefix(path, DoJobPath):
		CheckMethodE("POST", s.handleJobRequest, w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
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
		WriteAsJSON(Rsp{Msg: err.Error()}, w)
	}
}

func (s *Service) tryForwardToLeaderFn(f ServeHTTPFnE) ServeHTTPFnE {
	if s.store.IsLeader() {
		return f
	}

	return s.forwardToLeader
}

func (s *Service) tryForwardToLeader(f ServeHTTPFn, w http.ResponseWriter, r *http.Request) {
	if s.store.IsLeader() {
		f(w, r)
	} else if err := s.forwardToLeader(w, r); err != nil {
		WriteAsJSON(Rsp{Msg: err.Error()}, w)
	}
}

func (s *Service) forwardToLeader(w http.ResponseWriter, r *http.Request) error {
	leader, err := s.store.LeadServer()
	if err != nil {
		return err
	}

	addr := leader.ID.HTTPAddr()
	if addr == "" {
		return errors.New("failed to get raft leader")
	}

	s.Printf("forward %s to leader %s", r.URL.String(), addr)

	if xor := r.Header.Get(XOriginRemoteAddr); xor != "" {
		return errors.New("forward two times not allowed")
	}

	ReverseProxy(addr, r.URL.Path, 10*time.Second).ServeHTTP(w, r) // nolint gomnd

	return nil
}
