package hraftd

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
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
			WriteAsJSON(s.store.RaftStats(), w)
		}, w, r)
	case "/state":
		CheckMethod("GET", func(w http.ResponseWriter, _ *http.Request) {
			WriteAsJSON(Rsp{OK: true, Msg: s.store.NodeState()}, w)
		}, w, r)
	case "/cluster":
		CheckMethodE("GET", s.handleCluster, w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (s *Service) handleJoin(w http.ResponseWriter, r *http.Request) error {
	var m JoinRequest
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		return err
	}

	m.Fix(EmptyThen(r.Header.Get(XOriginRemoteAddr), r.RemoteAddr))

	log.Printf("received join request for remote node %s at %s\n", m.NodeID, m.Addr)

	if err := s.store.Join(string(m.NodeID), m.Addr); err != nil {
		return err
	}

	log.Printf("node %s at %s joined successfully\n", m.NodeID, m.Addr)

	WriteAsJSON(Rsp{OK: true, Msg: "OK"}, w)

	return nil
}

// RaftCluster returns raft cluster
func (s *Service) RaftCluster() (RaftCluster, error) { return s.store.Cluster() }

func (s *Service) listenLeaderCh() {
	for leaderChanged := range s.store.LeaderCh() {
		select {
		case s.LeaderCh <- leaderChanged:
		default:
		}

		log.Printf("leaderChanged to %v\n", leaderChanged)

		if !leaderChanged {
			continue
		}

		cluster, err := s.store.Cluster()
		if err != nil {
			log.Printf("s.store.Cluster failed %v\n", err)
			continue
		}

		v, _ := json.Marshal(cluster)
		cv := string(v)
		log.Printf("try s.store.Set /raft/cluster to %v\n", cv)

		if err := s.store.Set("/raft/cluster", cv); err != nil {
			log.Printf("s.store.Set /raft/cluster failed %v\n", err)
		} else {
			log.Printf("s.store.Set /raft/cluster suucessed\n")
		}
	}
}

func (s *Service) handleCluster(w http.ResponseWriter, _ *http.Request) error {
	servers, err := s.store.Cluster()
	if err != nil {
		return err
	}

	WriteAsJSON(servers, w)

	return nil
}