// Package store provides a simple distributed key-value store. The keys and
// associated values are changed via distributed consensus, meaning that the
// values are changed only when a majority of nodes in the cluster agree on
// the new value.
//
// Distributed consensus is provided via the Raft algorithm, specifically the
// Hashicorp implementation.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bingoohuang/hraftd/model"

	"github.com/bingoohuang/hraftd/util"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

const (
	retainSnapshotCount = 2
	raftTimeout         = 10 * time.Second
	leaderWaitDelay     = 100 * time.Millisecond
	appliedWaitDelay    = 100 * time.Millisecond
)

var (
	// ErrNotLeader is returned when a node attempts to execute a leader-only operation.
	ErrNotLeader = errors.New("not leader")
	// ErrOpenTimeout is returned when the Store does not apply its initial logs within the specified time.
	ErrOpenTimeout = errors.New("timeout waiting for initial logs application")
)

// Store is a simple key-value store, where all changes are made via Raft consensus.
type Store struct {
	mu sync.Mutex
	m  map[string]string // The key-value store for the system.

	raft *raft.Raft // The consensus mechanism

	logger *log.Logger
	Arg    *model.Arg
}

// New returns a new Store.
func New(arg *model.Arg) *Store {
	return &Store{
		m:      make(map[string]string),
		logger: log.New(os.Stderr, "[store] ", log.LstdFlags),

		Arg: arg,
	}
}

// RaftStats returns raft stats.
func (s *Store) RaftStats() map[string]string { return s.raft.Stats() }

// LeaderCh is used to get a channel which delivers signals on
// acquiring or losing leadership. It sends true if we become
// the leader, and false if we lose it. The channel is not buffered,
// and does not block on writes.
func (s *Store) LeaderCh() <-chan bool { return s.raft.LeaderCh() }

// NodeState returns the state of current node
func (s *Store) NodeState() string { return s.raft.State().String() }

// RaftServers returns the raft cluster state
func (s *Store) RaftServers() (model.RaftCluster, error) {
	leader := s.raft.Leader()
	cluster := model.RaftCluster{
		Servers: make([]model.Peer, 0),
	}

	err := s.walkRaftServers(func(srv raft.Server) error {
		peer := model.Peer{Address: string(srv.Address), NodeID: model.NodeID(srv.ID)}

		if leader == srv.Address {
			peer.State = raft.Leader.String()
			cluster.Leader = peer
		}

		if s.Arg.NodeID == peer.NodeID {
			peer.State = s.raft.State().String()
			cluster.Current = peer
		}

		if peer.State == "" {
			if r := s.getNodeState(peer); r.OK {
				peer.State = r.Msg
			}
		}

		if peer.State != "" {
			cluster.Servers = append(cluster.Servers, peer)
		}

		return nil
	})

	return cluster, err
}

func (s *Store) getNodeState(peer model.Peer) *model.Rsp {
	r := &model.Rsp{}
	u := peer.NodeID.URLRaftState()
	rsp, err := util.GetJSON(u, r)
	s.logger.Printf("invoke get node state %s rsp %v\n", u, rsp)

	if err != nil {
		s.logger.Printf("invoke %s error %v", u, err)
	}

	return r
}

// Open opens the store. If enableSingle is set, and there are no existing peers,
// then this node becomes the first node, and therefore leader, of the cluster.
// localID should be the server identifier for this node.
func (s *Store) Open() error {
	// Setup Raft configuration.
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(s.Arg.NodeID)

	logStore, stableStore, snapshots, err := s.createStores()
	if err != nil {
		return err
	}

	transport, err := s.createTransport()
	if err != nil {
		return err
	}

	r, err := raft.NewRaft(config, s, logStore, stableStore, snapshots, transport)
	if err != nil {
		return fmt.Errorf("new raft: %s", err)
	}

	s.raft = r

	if s.Arg.Bootstrap {
		s.raft.BootstrapCluster(raft.Configuration{
			Servers: []raft.Server{{ID: config.LocalID, Address: transport.LocalAddr()}}})
	}

	return nil
}

func (s *Store) createTransport() (*raft.NetworkTransport, error) {
	// Setup Raft communication.
	addr, err := net.ResolveTCPAddr("tcp", s.Arg.RaftAddr)
	if err != nil {
		return nil, err
	}

	// nolint gomnd
	return raft.NewTCPTransport(s.Arg.RaftAddr, addr, 3, 10*time.Second, os.Stderr)
}

// createStores creates the log store and stable store.
func (s *Store) createStores() (raft.LogStore, raft.StableStore, raft.SnapshotStore, error) {
	// Create the snapshot store. This allows the Raft to truncate the log.
	ss, err := raft.NewFileSnapshotStore(s.Arg.RaftDir, retainSnapshotCount, os.Stderr)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("file snapshot store: %s", err)
	}

	if s.Arg.InMem {
		return raft.NewInmemStore(), raft.NewInmemStore(), ss, nil
	}

	db, err := raftboltdb.NewBoltStore(filepath.Join(s.Arg.RaftDir, "raft.db"))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("new bolt store: %s", err)
	}

	return db, db, ss, nil
}

// Get returns the value for the given key.
func (s *Store) Get(key string) (v string, ok bool, err error) {
	s.lockApplyOp(func() interface{} { v, ok = s.m[key]; return nil })

	return
}

// IsLeader tells the current node is raft leader or not.
func (s *Store) IsLeader() bool { return s.raft.State() == raft.Leader }

// Set sets the value for the given key.
func (s *Store) Set(key, value string) error {
	if !s.IsLeader() {
		return ErrNotLeader
	}

	b, _ := json.Marshal(&model.Command{Op: "set", Key: key, Value: value,
		Time: time.Now().Format("2006-01-02 15:04:05.000")})
	f := s.raft.Apply(b, raftTimeout)

	return f.Error()
}

// Delete deletes the given key.
func (s *Store) Delete(key string) error {
	if !s.IsLeader() {
		return ErrNotLeader
	}

	b, _ := json.Marshal(&model.Command{Op: "delete", Key: key,
		Time: time.Now().Format("2006-01-02 15:04:05.000")})
	f := s.raft.Apply(b, raftTimeout)

	return f.Error()
}

// Join joins a node, identified by nodeID and located at addr, to this store.
// The node must be ready to respond to Raft communications at that address.
func (s *Store) Join(nodeID, addr string) error {
	serverID := raft.ServerID(nodeID)
	serverAddress := raft.ServerAddress(addr)

	err := s.walkRaftServers(func(srv raft.Server) error {
		// If a node already exists with either the joining node's ID or address,
		// that node may need to be removed from the config first.
		idEquals := srv.ID == serverID
		AddrEquals := srv.Address == serverAddress

		// If *both* the ID and the address are the same,
		// then nothing -- not even a join operation -- is needed.
		if AddrEquals && idEquals {
			return nil // already member of cluster, ignoring join request
		}

		if idEquals || AddrEquals {
			if f := s.raft.RemoveServer(srv.ID, 0, 0); f.Error() != nil {
				return fmt.Errorf("error removing existing node %s at %s: %w", nodeID, addr, f.Error())
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	if f := s.raft.AddVoter(serverID, serverAddress, 0, 0); f.Error() != nil {
		return fmt.Errorf("node %s at %s joined error %w", nodeID, addr, err)
	}

	return nil
}

// LeaderAddr returns the address of the current leader. Returns a
// blank string if there is no leader.
func (s *Store) LeaderAddr() string { return string(s.raft.Leader()) }

// WaitForLeader blocks until a leader is detected, or the timeout expires.
func (s *Store) WaitForLeader(timeout time.Duration) (string, error) {
	tck := time.NewTicker(leaderWaitDelay)
	tmr := time.NewTimer(timeout)

	defer tck.Stop()
	defer tmr.Stop()

	for {
		select {
		case <-tck.C:
			if l := s.LeaderAddr(); l != "" {
				return l, nil
			}
		case <-tmr.C:
			return "", fmt.Errorf("timeout expired")
		}
	}
}

// WaitForApplied waits for all Raft log entries to to be applied to the
// underlying database.
func (s *Store) WaitForApplied(timeout time.Duration) error {
	if timeout == 0 {
		return nil
	}

	s.logger.Printf("waiting for up to %s for application of initial logs", timeout)

	if err := s.WaitForAppliedIndex(s.raft.LastIndex(), timeout); err != nil {
		return ErrOpenTimeout
	}

	return nil
}

// WaitForAppliedIndex blocks until a given log index has been applied,
// or the timeout expires.
func (s *Store) WaitForAppliedIndex(idx uint64, timeout time.Duration) error {
	tck := time.NewTicker(appliedWaitDelay)
	tmr := time.NewTimer(timeout)

	defer tck.Stop()
	defer tmr.Stop()

	for {
		select {
		case <-tck.C:
			if s.raft.AppliedIndex() >= idx {
				return nil
			}
		case <-tmr.C:
			return fmt.Errorf("timeout expired")
		}
	}
}

// Apply applies a Raft log entry to the key-value store.
func (s *Store) Apply(l *raft.Log) interface{} {
	var c model.Command
	if err := json.Unmarshal(l.Data, &c); err != nil {
		panic(fmt.Sprintf("failed to unmarshal Command: %s", err.Error()))
	}

	intercepted := false
	if s.Arg.ApplyInterceptor != nil {
		intercepted = s.Arg.ApplyInterceptor(l, c)
	}

	if intercepted {
		return nil
	}

	switch c.Op {
	case "set":
		return s.lockApplyOp(func() interface{} { s.m[c.Key] = c.Value; return nil })
	case "delete":
		return s.lockApplyOp(func() interface{} { delete(s.m, c.Key); return nil })
	default:
		s.logger.Printf("unrecognized Command op: %+v", c)
	}

	return nil
}

// Snapshot returns a snapshot of the key-value store.
func (s *Store) Snapshot() (raft.FSMSnapshot, error) {
	fn := func() interface{} { return &fsmSnapshot{store: util.CloneMap(s.m)} }
	return s.lockApplyOp(fn).(raft.FSMSnapshot), nil
}

// Restore stores the key-value store to a previous state.
func (s *Store) Restore(rc io.ReadCloser) error {
	o := make(map[string]string)
	if err := json.NewDecoder(rc).Decode(&o); err != nil {
		return err
	}

	// Set the state from the snapshot, no lock required according to Hashicorp docs.
	s.m = o

	return nil
}

type fsmSnapshot struct {
	store map[string]string
}

func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	err := func() error {
		b, _ := json.Marshal(f.store)

		if _, err := sink.Write(b); err != nil {
			return err
		}

		return sink.Close()
	}()

	if err != nil {
		_ = sink.Cancel()
	}

	return err
}

func (f *fsmSnapshot) Release() {}

func (s *Store) walkRaftServers(fn func(srv raft.Server) error) error {
	f := s.raft.GetConfiguration()
	if err := f.Error(); err != nil {
		return fmt.Errorf("failed to get raft configuration: %w", err)
	}

	for _, srv := range f.Configuration().Servers {
		if err := fn(srv); err != nil {
			return fmt.Errorf("failed to invoke walk fn: %w", err)
		}
	}

	return nil
}

func (s *Store) lockApplyOp(fn func() interface{}) interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	return fn()
}
