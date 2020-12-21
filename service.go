package hraftd

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"strings"
	"time"
)

// Service provides HTTP service.
type Service struct {
	Store Store
	Ln    net.Listener
	*Arg

	LeaderCh chan bool
	DealerMap
}

// Create returns an uninitialized service.
func Create(arg *Arg) *Service {
	if arg.LoggerMore == nil {
		arg.LoggerMore = DefaultLogger
	}

	s := New(arg)
	if s.LoggerMore == nil {
		s.LoggerMore = DefaultLogger
	}

	if err := s.Open(); err != nil {
		s.Panicf("failed to open Store: %s", err.Error())
	}

	return &Service{Arg: arg, Store: s, DealerMap: MakeDealerMap(), LeaderCh: make(chan bool, 1)}
}

// StartAll starts the http and raft service.
func (s *Service) StartAll() error {
	if s.LoggerMore == nil {
		s.LoggerMore = DefaultLogger
	}

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
				s.Infof("joined at %s successfully", s.Arg.JoinAddrs)

				return
			}

			s.Errorf("failed to join at %s error %s, retry after 10s", s.Arg.JoinAddrs, err.Error())
			time.Sleep(10 * time.Second) // nolint:gomnd
		}
	}()

	waitTimeout := 100 * time.Second // nolint:gomnd
	_, _ = s.Store.WaitForLeader(waitTimeout)
	_ = s.Store.WaitForApplied(waitTimeout)

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

// Addr returns the address on which the Service is listening.
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
	case strings.HasPrefix(path, "/debug/pprof"):
		_ = debugPprof(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func debugPprof(w http.ResponseWriter, req *http.Request) error {
	switch req.URL.Path {
	case "/debug/pprof":
		pprof.Index(w, req)
	case "/debug/pprof/cmdline":
		pprof.Cmdline(w, req)
	case "/debug/pprof/symbol":
		pprof.Symbol(w, req)
	case "/debug/pprof/profile":
		pprof.Profile(w, req)
	case "/debug/pprof/trace":
		pprof.Trace(w, req)
	case "/debug/pprof/heap":
		pprof.Handler("heap").ServeHTTP(w, req)
	case "/debug/pprof/goroutine":
		pprof.Handler("goroutine").ServeHTTP(w, req)
	case "/debug/pprof/allocs":
		pprof.Handler("allocs").ServeHTTP(w, req)
	case "/debug/pprof/block":
		pprof.Handler("block").ServeHTTP(w, req)
	case "/debug/pprof/threadcreate":
		pprof.Handler("threadcreate").ServeHTTP(w, req)
	case "/debug/pprof/mutex":
		pprof.Handler("mutex").ServeHTTP(w, req)
	default:
		return fmt.Errorf("404 %s", req.URL.Path)
	}

	return nil
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
	if s.Store.IsLeader() {
		return f
	}

	return s.forwardToLeader
}

func (s *Service) tryForwardToLeader(f ServeHTTPFn, w http.ResponseWriter, r *http.Request) {
	if s.Store.IsLeader() {
		f(w, r)
	} else if err := s.forwardToLeader(w, r); err != nil {
		WriteAsJSON(Rsp{Msg: err.Error()}, w)
	}
}

func (s *Service) forwardToLeader(w http.ResponseWriter, r *http.Request) error {
	leader, err := s.Store.LeadServer()
	if err != nil {
		return err
	}

	addr := leader.ID.HTTPAddr()
	if addr == "" {
		return errors.New("failed to get raft leader") // nolint:goerr113
	}

	s.Printf("forward %s to leader %s", r.URL.String(), addr)

	if xor := r.Header.Get(XOriginRemoteAddr); xor != "" {
		return errors.New("forward two times not allowed") // nolint:goerr113
	}

	ReverseProxy(addr, r.URL.Path, 10*time.Second).ServeHTTP(w, r) // nolint:gomnd

	return nil
}

// IsLeader tells the current node is raft leader or not.
func (s *Service) IsLeader() bool {
	return s.Store.IsLeader()
}
