package hraftd

import (
	"github.com/bingoohuang/goip"
	"net/http"
	"os"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
)

func (s *Service) handleRaftRequest(w http.ResponseWriter, r *http.Request) {
	switch strings.TrimPrefix(r.URL.Path, RaftPath) {
	case "/health":
		CheckMethod("GET", func(w http.ResponseWriter, _ *http.Request) {
			WriteAsText("OK", w)
		}, w, r)
	case "/join":
		CheckMethodE("POST", s.tryForwardToLeaderFn(s.handleJoin), w, r)
	case "/remove":
		CheckMethodE("DELETE", s.tryForwardToLeaderFn(s.handleRemove), w, r)
	case "/stats":
		CheckMethod("GET", func(w http.ResponseWriter, _ *http.Request) {
			WriteAsJSON(s.Store.RaftStats(), w)
		}, w, r)
	case "/state":
		CheckMethod("GET", func(w http.ResponseWriter, _ *http.Request) {
			WriteAsJSON(Rsp{OK: true, Msg: s.Store.NodeState()}, w)
		}, w, r)
	case "/cluster":
		CheckMethodE("GET", s.handleCluster, w, r)
	case "/node":
		CheckMethodE("GET", s.handleNode, w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (s *Service) handleJoin(w http.ResponseWriter, r *http.Request) error {
	var m JoinRequest
	if err := jsoniter.NewDecoder(r.Body).Decode(&m); err != nil {
		return err
	}

	m.Fix(EmptyThen(r.Header.Get(XOriginRemoteAddr), r.RemoteAddr))

	s.Printf("received join request for remote node %s at %s", m.NodeID, m.Addr)

	if err := s.Store.Join(string(m.NodeID), m.Addr); err != nil {
		return err
	}

	s.Printf("node %s at %s joined successfully", m.NodeID, m.Addr)

	s.saveCluster()

	WriteAsJSON(Rsp{OK: true, Msg: "OK"}, w)

	return nil
}

// RaftCluster returns raft cluster.
func (s *Service) RaftCluster() (RaftCluster, error) { return s.Store.Cluster() }

func (s *Service) listenLeaderCh() {
	for leaderChanged := range s.Store.LeaderCh() {
		select {
		case s.LeaderCh <- leaderChanged:
		default:
		}

		s.Printf("leader changed to %v", leaderChanged)

		if !leaderChanged {
			continue
		}
	}
}

var startupTime = time.Now()

// NodeState is raft cluster node state.
type NodeState struct {
	StartTime string `json:"startTime"`
	NodeID    string `json:"nodeID"`
	Hostname  string `json:"hostname"`
	IP        string `json:"IP"`
}

func (s *Service) handleNode(w http.ResponseWriter, _ *http.Request) error {
	hostname, _ := os.Hostname()
	mainIP, _ := goip.MainIP()

	WriteAsJSON(NodeState{
		StartTime: startupTime.Format(`2006-01-02 15:04:05.000`),
		NodeID:    string(s.NodeID),
		Hostname:  hostname,
		IP:        mainIP,
	}, w)

	return nil
}

func (s *Service) handleCluster(w http.ResponseWriter, _ *http.Request) error {
	servers, err := s.Store.Cluster()
	if err != nil {
		return err
	}

	WriteAsJSON(servers, w)

	return nil
}

func (s *Service) saveCluster() {
	cluster, err := s.Store.Cluster()
	if err != nil {
		s.Printf("s.store.Cluster failed %v", err)
		return
	}

	cv := Jsonify(cluster)
	s.Printf("try s.store.Set /raft/cluster to %v", cv)

	if err := s.Store.Set("raft/cluster", cv); err != nil {
		s.Printf("s.store.Set raft/cluster failed %v", err)
	} else {
		s.Printf("s.store.Set raft/cluster successfully")
	}
}
