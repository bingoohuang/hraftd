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

// State returns the raft cluster state
func (s *Store) State() (model.RaftClusterState, error) {
	leader := s.raft.Leader()
	clusterState := model.RaftClusterState{
		Servers: make([]model.Peer, 0),
	}

	err := s.walkRaftServers(func(srv raft.Server) error {
		peer := model.Peer{Address: string(srv.Address), NodeID: string(srv.ID)}

		if leader == srv.Address {
			peer.State = raft.Leader.String()
			clusterState.Leader = peer
		}

		if s.Arg.NodeID == peer.NodeID {
			peer.State = s.raft.State().String()
			clusterState.Current = peer
		}

		clusterState.Servers = append(clusterState.Servers, peer)

		return nil
	})

	return clusterState, err
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

var (
	ErrNotLeader = errors.New("not leader")
)

// IsLeader tells the current node is raft leader or not.
func (s *Store) IsLeader() bool { return s.raft.State() == raft.Leader }

// Set sets the value for the given key.
func (s *Store) Set(key, value string) error {
	if !s.IsLeader() {
		return ErrNotLeader
	}

	b, _ := json.Marshal(&model.Command{Op: "set", Key: key, Value: value})
	f := s.raft.Apply(b, raftTimeout)

	return f.Error()
}

// Delete deletes the given key.
func (s *Store) Delete(key string) error {
	if !s.IsLeader() {
		return ErrNotLeader
	}

	b, _ := json.Marshal(&model.Command{Op: "delete", Key: key})
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

// Apply applies a Raft log entry to the key-value store.
func (s *Store) Apply(l *raft.Log) interface{} {
	var c model.Command
	if err := json.Unmarshal(l.Data, &c); err != nil {
		panic(fmt.Sprintf("failed to unmarshal Command: %s", err.Error()))
	}

	switch c.Op {
	case "set":
		return s.lockApplyOp(func() interface{} { s.m[c.Key] = c.Value; return nil })
	case "delete":
		return s.lockApplyOp(func() interface{} { delete(s.m, c.Key); return nil })
	default:
		panic(fmt.Sprintf("unrecognized Command op: %s", c.Op))
	}
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

func (s *Store) walkRaftServers(f func(srv raft.Server) error) error {
	configFuture := s.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		return fmt.Errorf("failed to get raft configuration: %w", err)
	}

	for _, srv := range configFuture.Configuration().Servers {
		if err := f(srv); err != nil {
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
