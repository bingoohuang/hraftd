package hraftd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

const (
	retainSnapshotCount = 2
	raftTimeout         = 10 * time.Second
	appliedWaitDelay    = 100 * time.Millisecond
)

var (
	// ErrNotLeader is returned when a node attempts to execute a leader-only operation.
	ErrNotLeader = errors.New("not leader")
	// ErrOpenTimeout is returned when the Store does not apply its initial logs within the specified time.
	ErrOpenTimeout = errors.New("timeout waiting for initial logs application")
)

// RaftStore is a simple key-value store, where all changes are made via Raft consensus.
type RaftStore struct {
	mu sync.Mutex
	m  map[string]string // The key-value store for the system.

	raft *raft.Raft // The consensus mechanism

	*Arg
}

// New returns a new Store.
func New(arg *Arg) *RaftStore {
	r := &RaftStore{
		m:   make(map[string]string),
		Arg: arg,
	}

	if r.LoggerMore == nil {
		r.LoggerMore = DefaultLogger
	}

	return r
}

// RaftStats returns raft stats.
func (s *RaftStore) RaftStats() map[string]interface{} {
	stats := s.raft.Stats()
	m := make(map[string]interface{})

	for k, v := range stats {
		if k != "latest_configuration" {
			m[k] = v
			continue
		}

		servers := s.raft.GetConfiguration().Configuration().Servers
		nodes := make([]ConfigEntry, len(servers))

		for i, server := range servers {
			nodes[i] = ConfigEntry{
				ID:       server.ID,
				Address:  server.Address,
				Suffrage: server.Suffrage.String(),
			}
		}

		m[k] = nodes
	}

	return m
}

// LeaderCh is used to get a channel which delivers signals on
// acquiring or losing leadership. It sends true if we become
// the leader, and false if we lose it. The channel is not buffered,
// and does not block on writes.
func (s *RaftStore) LeaderCh() <-chan bool { return s.raft.LeaderCh() }

// NodeState returns the state of current node
func (s *RaftStore) NodeState() string { return s.raft.State().String() }

// IsLeader tells the current node is raft leader or not.
func (s *RaftStore) IsLeader() bool { return s.raft.State() == raft.Leader }

// LeaderAddr returns the address of the current leader. Returns blank if no leader.
func (s *RaftStore) LeaderAddr() string { return string(s.raft.Leader()) }

// LeadServer returns the raft lead server
func (s *RaftStore) LeadServer() (Peer, error) {
	leader := s.raft.Leader()
	peer := Peer{}

	if leader == "" {
		return peer, errors.New("leader NA")
	}

	if err := s.walkRaftServers(func(srv raft.Server) (bool, error) {
		if leader != srv.Address {
			return true, nil
		}

		peer = Peer{
			Address: string(srv.Address),
			ID:      NodeID(srv.ID),
			State:   raft.Leader.String(),
		}

		return false, nil
	}); err != nil {
		return peer, err
	}

	if peer.ID == "" {
		return peer, errors.New("leader NA")
	}

	return peer, nil
}

// Cluster returns the raft cluster state
func (s *RaftStore) Cluster() (RaftCluster, error) {
	leaderAddress := s.raft.Leader()
	cluster := RaftCluster{
		Servers: make([]Peer, 0),
	}

	err := s.walkRaftServers(func(srv raft.Server) (bool, error) {
		peer := Peer{Address: string(srv.Address), ID: NodeID(srv.ID), Suffrage: srv.Suffrage.String()}

		if leaderAddress == srv.Address {
			peer.State = raft.Leader.String()
			cluster.Leader = peer
		}

		if s.Arg.NodeID == peer.ID {
			peer.State = s.raft.State().String()
			cluster.Current = peer
		}

		if peer.State == "" {
			if r := s.getNodeState(peer.ID); r.OK {
				peer.State = r.Msg
			}
		}

		if peer.State == "" {
			peer.State = "Lost"
		}

		cluster.Servers = append(cluster.Servers, peer)

		return true, nil
	})

	return cluster, err
}

func (s *RaftStore) getNodeState(nodeID NodeID) *Rsp {
	r := &Rsp{}
	u := nodeID.URLRaftState()
	rsp, err := GetJSON(u, r)
	s.Infof("invoke get node state %s rsp %v", u, rsp)

	if err != nil {
		s.Infof("invoke %s error %v", u, err)
	}

	return r
}

// Open opens the store. If enableSingle is set, and there are no existing peers,
// then this node becomes the first node, and therefore leader, of the cluster.
// localID should be the server identifier for this node.
func (s *RaftStore) Open() error {
	if s.LoggerMore == nil {
		s.LoggerMore = DefaultLogger
	}

	// Setup Raft configuration.
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(s.Arg.NodeID)

	peerFile := filepath.Join(s.Arg.RaftNodeDir, "peers.json")
	peerFileExits := PathExists(peerFile)

	s.Infof("RaftNodeDir %s exists %v", s.Arg.RaftNodeDir, peerFileExits)

	logStore, stableStore, snapshots, err := s.createStores()
	if err != nil {
		return err
	}

	t, err := s.createTransport()
	if err != nil {
		return err
	}

	config.Logger = &hclogLogger{Logger: s, name: "[raft]"}

	r, err := raft.NewRaft(config, s, logStore, stableStore, snapshots, t)
	if err != nil {
		return fmt.Errorf("new raft: %s", err)
	}

	s.raft = r

	if s.Arg.Bootstrap && !peerFileExits {
		c := raft.Configuration{Servers: []raft.Server{{ID: config.LocalID, Address: t.LocalAddr()}}}
		s.raft.BootstrapCluster(c)
	}

	if peerFileExits {
		if err := s.recoverJoin(peerFile); err != nil {
			return err
		}
	}

	return nil
}

func (s *RaftStore) recoverJoin(peerFile string) error {
	c, err := ReadPeersJSON(peerFile)
	if err != nil {
		return err
	}

	if ok := s.tryJoinLeader(c); ok {
		return nil
	}

	//if err := raft.RecoverCluster(config, s, logStore, stableStore, snapshots, t, c); err != nil {
	//	return err
	//}

	s.Infof("recovered from %s successfully", peerFile)

	return nil
}

func (s *RaftStore) tryJoinLeader(c raft.Configuration) bool {
	leaderID := NodeID("")

	for _, server := range c.Servers {
		nodeID := NodeID(server.ID)
		if r := s.getNodeState(nodeID); r.OK && r.Msg == StateLeader {
			leaderID = nodeID
		}
	}

	if leaderID == "" {
		return false
	}

	if err := Join(s, leaderID.HTTPAddr(), s.Arg.RaftAddr, s.Arg.NodeID); err == nil {
		return true
	}

	return false
}

// createTransport setup Raft communication.
func (s *RaftStore) createTransport() (*raft.NetworkTransport, error) {
	addr, err := net.ResolveTCPAddr("tcp", s.Arg.RaftAddr)

	if err != nil {
		return nil, err
	}

	// nolint gomnd
	return raft.NewTCPTransport(s.Arg.RaftAddr, addr, 3, 10*time.Second, os.Stderr)
}

// createStores creates the log store and stable store.
func (s *RaftStore) createStores() (raft.LogStore, raft.StableStore, raft.SnapshotStore, error) {
	if s.Arg.InMem {
		return raft.NewInmemStore(), raft.NewInmemStore(), raft.NewInmemSnapshotStore(), nil
	}

	db, err := raftboltdb.NewBoltStore(filepath.Join(s.Arg.RaftNodeDir, "raft.db"))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("new bolt store: %s", err)
	}

	// Create the snapshot store. This allows the Raft to truncate the log.
	ss, err := raft.NewFileSnapshotStore(s.Arg.RaftNodeDir, retainSnapshotCount, os.Stderr)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("file snapshot store: %s", err)
	}

	return db, db, ss, nil
}

// Get returns the value for the given key.
func (s *RaftStore) Get(key string) (v string, ok bool) {
	s.lockApplyOp(func() interface{} { v, ok = s.m[key]; return nil })

	return
}

// Set sets the value for the given key.
func (s *RaftStore) Set(key, value string) error {
	if !s.IsLeader() {
		return ErrNotLeader
	}

	c := &Command{Op: "set", Key: key, Value: value, Time: FormatTime(time.Now())}
	f := s.raft.Apply(JsonifyBytes(c), raftTimeout)

	return f.Error()
}

// Delete deletes the given key.
func (s *RaftStore) Delete(key string) error {
	if !s.IsLeader() {
		return ErrNotLeader
	}

	c := &Command{Op: "delete", Key: key, Time: FormatTime(time.Now())}
	f := s.raft.Apply(JsonifyBytes(c), raftTimeout)

	return f.Error()
}

// Remove removes the node, with the given nodeID, from the cluster.
func (s *RaftStore) Remove(nodeID string) error {
	if s.raft.State() != raft.Leader {
		return ErrNotLeader
	}

	if f := s.raft.RemoveServer(raft.ServerID(nodeID), 0, 0); f.Error() != nil {
		return f.Error()
	}

	_ = s.writeConfigEntries()

	return nil
}

// Join joins a node, identified by nodeID and located at addr, to this store.
// The node must be ready to respond to Raft communications at that address.
func (s *RaftStore) Join(nodeID, addr string) error {
	s.Infof("received request to join node at %s", addr)

	if s.raft.State() != raft.Leader {
		return ErrNotLeader
	}

	serverID := raft.ServerID(nodeID)
	serverAddress := raft.ServerAddress(addr)
	alreadyJoined := false

	if err := s.walkRaftServers(func(srv raft.Server) (bool, error) {
		// If a node already exists with either the joining node's ID or address,
		// that node may need to be removed from the config first.
		idEquals, addrEquals := srv.ID == serverID, srv.Address == serverAddress

		// If *both* the ID and the address are the same,
		// then nothing -- not even a join operation -- is needed.
		if addrEquals && idEquals {
			alreadyJoined = true
			return false, nil // already member of cluster, ignoring join request
		}

		if idEquals || addrEquals {
			if f := s.raft.RemoveServer(srv.ID, 0, 0); f.Error() != nil {
				return false, fmt.Errorf("error removing existing node %s at %s: %w", nodeID, addr, f.Error())
			}
		}

		return true, nil
	}); err != nil {
		return err
	}

	if alreadyJoined {
		s.Infof("node %s at %s already member of cluster, ignoring join request", nodeID, addr)

		return nil
	}

	if f := s.raft.AddVoter(serverID, serverAddress, 0, 0); f.Error() != nil {
		return fmt.Errorf("node %s at %s joined error %w", nodeID, addr, f.Error())
	}

	_ = s.writeConfigEntries()

	return nil
}

// ConfigEntry is used when decoding a new-style peers.json.
type ConfigEntry struct {
	// ID is the ID of the server (a UUID, usually).
	ID raft.ServerID `json:"id"`

	// Address is the host:port of the server.
	Address raft.ServerAddress `json:"address"`

	// NonVoter controls the suffrage. We choose this sense so people
	// can leave this out and get a Voter by default.
	Suffrage string `json:"suffrage"`
}

func (s *RaftStore) writeClusterConfigEntries(cluster RaftCluster) error {
	entries := make([]ConfigEntry, len(cluster.Servers))

	for i, server := range cluster.Servers {
		entries[i] = ConfigEntry{
			ID:       raft.ServerID(server.ID),
			Address:  raft.ServerAddress(server.Address),
			Suffrage: server.Suffrage,
		}
	}

	return s.writePeersJSON(entries)
}

func (s *RaftStore) writeConfigEntries() error {
	if s.Arg.InMem {
		return nil
	}

	servers := s.raft.GetConfiguration().Configuration().Servers
	entries := make([]ConfigEntry, len(servers))

	for i, server := range servers {
		entries[i] = ConfigEntry{
			ID:       server.ID,
			Address:  server.Address,
			Suffrage: server.Suffrage.String(),
		}
	}

	return s.writePeersJSON(entries)
}

func (s *RaftStore) writePeersJSON(entries []ConfigEntry) error {
	_ = os.MkdirAll(s.Arg.RaftNodeDir, 0644)
	peerFile := filepath.Join(s.Arg.RaftNodeDir, "peers.json")

	return ioutil.WriteFile(peerFile, JsonifyBytes(entries), 0644)
}

// ReadPeersJSON consumes a legacy peers.json file in the format of the old JSON
// peer store and creates a new-style configuration structure. This can be used
// to migrate this data or perform manual recovery when running protocol versions
// that can interoperate with older, unversioned Raft servers. This should not be
// used once server IDs are in use, because the old peers.json file didn't have
// support for these, nor non-voter suffrage types.
func ReadPeersJSON(path string) (raft.Configuration, error) {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return raft.Configuration{}, err
	}

	var peers []ConfigEntry

	dec := json.NewDecoder(bytes.NewReader(buf))
	if err := dec.Decode(&peers); err != nil {
		return raft.Configuration{}, err
	}

	// Map it into the new-style configuration structure. We can only specify
	// voter roles here, and the ID has to be the same as the address.
	c := raft.Configuration{}
	for _, peer := range peers {
		c.Servers = append(c.Servers, raft.Server{
			Suffrage: ParseSuffrage(peer.Suffrage),
			ID:       peer.ID, Address: peer.Address},
		)
	}

	return c, nil
}

// ParseSuffrage parses s to raft.ServerSuffrage.
func ParseSuffrage(s string) raft.ServerSuffrage {
	switch s {
	case "Voter":
		return raft.Voter
	case "Nonvoter":
		return raft.Nonvoter
	case "Staging":
		return raft.Staging
	default:
		return raft.Voter
	}
}

// WaitForLeader blocks until a leader is detected, or the timeout expires.
func (s *RaftStore) WaitForLeader(timeout time.Duration) (string, error) {
	tck := time.NewTicker(3 * time.Second) // nolint gomnd
	tmr := time.NewTimer(timeout)

	defer tck.Stop()
	defer tmr.Stop()

	for {
		select {
		case <-tck.C:
			if l := s.LeaderAddr(); l != "" {
				return l, nil
			}

			s.joinNodesFromLeader()
		case <-tmr.C:
			s.Infof("waitForLeader timeout %v expired", timeout)
			return "", fmt.Errorf("WaitForLeader timeout expired")
		}
	}
}

func (s *RaftStore) joinNodesFromLeader() {
	cluster, err := s.Cluster()
	if err != nil {
		return
	}

	s.tryFindAndJoinLeader(cluster)
}

func (s *RaftStore) tryFindAndJoinLeader(cluster RaftCluster) {
	leader, _ := s.findLeader(cluster)

	if leader.ID != "" {
		s.Infof("Leader found %+v", leader)
		_ = Join(s, leader.ID.HTTPAddr(), s.Arg.RaftAddr, s.Arg.NodeID)

		return
	}

	s.Infof("Leader not found")
}

func (s *RaftStore) findLeader(cluster RaftCluster) (Peer, bool) {
	for _, peer := range cluster.Servers {
		if peer.State == StateLeader {
			return peer, true
		}

		cl, err := Cluster(s, peer.ID)
		if err != nil {
			continue
		}

		if cl.Leader.State == StateLeader {
			return cl.Leader, true
		}

		for _, n := range cl.Servers {
			if n.State == StateLeader {
				return n, true
			}
		}
	}

	return Peer{}, false
}

// WaitForApplied waits for all Raft log entries to to be applied to the
// underlying database.
func (s *RaftStore) WaitForApplied(timeout time.Duration) error {
	if timeout == 0 {
		return nil
	}

	s.Infof("waiting for up to %s for application of initial logs", timeout)

	if err := s.WaitForAppliedIndex(s.raft.LastIndex(), timeout); err != nil {
		return ErrOpenTimeout
	}

	return nil
}

// WaitForAppliedIndex blocks until a given log index has been applied,
// or the timeout expires.
func (s *RaftStore) WaitForAppliedIndex(idx uint64, timeout time.Duration) error {
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
func (s *RaftStore) Apply(l *raft.Log) interface{} {
	var c Command
	if err := json.Unmarshal(l.Data, &c); err != nil {
		panic(fmt.Sprintf("failed to unmarshal Command: %s", err.Error()))
	}

	if rsp, intercepted := s.Arg.Intercept(l, c); intercepted {
		return rsp
	}

	switch c.Op {
	case "set":
		t, err := ParseTime(c.Time)
		if err != nil || t.Before(time.Now().Add(-100*time.Second)) { // nolint gomnd
			s.Infof("too old command  %+v, ignored", c)
			return nil
		}

		if c.Key == "/raft/cluster" {
			s.processSetRaftCluster(c)

			return nil
		}

		return s.lockApplyOp(func() interface{} { s.m[c.Key] = c.Value; return nil })
	case "delete":
		return s.lockApplyOp(func() interface{} { delete(s.m, c.Key); return nil })
	default:
		s.Infof("unrecognized Command op: %+v", c)
	}

	return nil
}

func (s *RaftStore) processSetRaftCluster(c Command) {
	if s.Arg.InMem {
		return
	}

	v := RaftCluster{}
	if err := json.Unmarshal([]byte(c.Value), &v); err != nil {
		s.Infof("json.Unmarshal error %+v", err)
	} else if err := s.writeClusterConfigEntries(v); err != nil {
		s.Infof("writeClusterConfigEntries error %+v", err)
	} else {
		s.Infof("writeClusterConfigEntries successfully")

		if s.LeaderAddr() == "" {
			s.tryFindAndJoinLeader(v)
		}
	}
}

// Snapshot returns a snapshot of the key-value store.
func (s *RaftStore) Snapshot() (raft.FSMSnapshot, error) {
	fn := func() interface{} { return &fsmSnapshot{store: CloneMap(s.m)} }
	return s.lockApplyOp(fn).(raft.FSMSnapshot), nil
}

// Restore stores the key-value store to a previous state.
func (s *RaftStore) Restore(rc io.ReadCloser) error {
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
		if _, err := sink.Write(JsonifyBytes(f.store)); err != nil {
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

func (s *RaftStore) walkRaftServers(fn func(srv raft.Server) (bool, error)) error {
	f := s.raft.GetConfiguration()
	if err := f.Error(); err != nil {
		return fmt.Errorf("failed to get raft configuration: %w", err)
	}

	for _, srv := range f.Configuration().Servers {
		if cont, err := fn(srv); err != nil {
			return fmt.Errorf("failed to invoke walk fn: %w", err)
		} else if !cont {
			break
		}
	}

	return nil
}

func (s *RaftStore) lockApplyOp(fn func() interface{}) interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	return fn()
}

type hclogLogger struct {
	Logger LoggerMore
	args   []interface{}
	name   string
}

func (h hclogLogger) Debug(msg string, args ...interface{}) { h.Logger.Debugf(msg, args...) }
func (h hclogLogger) Info(msg string, args ...interface{})  { h.Logger.Infof(msg, args...) }
func (h hclogLogger) Warn(msg string, args ...interface{})  { h.Logger.Warnf(msg, args...) }
func (h hclogLogger) Error(msg string, args ...interface{}) { h.Logger.Errorf(msg, args...) }
func (h hclogLogger) Log(level hclog.Level, msg string, args ...interface{}) {
	h.Logger.Logf(h.convertLevel(level), msg, args...)
}

func (h hclogLogger) ImpliedArgs() []interface{}            { return h.args }
func (h hclogLogger) Name() string                          { return h.name }
func (h hclogLogger) Trace(msg string, args ...interface{}) {}

func (h hclogLogger) IsTrace() bool { return false }
func (h hclogLogger) IsDebug() bool { return h.Logger.GetLogLevel() <= LogLevelDebug }
func (h hclogLogger) IsInfo() bool  { return h.Logger.GetLogLevel() <= LogLevelInfo }
func (h hclogLogger) IsWarn() bool  { return h.Logger.GetLogLevel() <= LogLevelWarn }
func (h hclogLogger) IsError() bool { return h.Logger.GetLogLevel() <= LogLevelError }

func (h hclogLogger) With(args ...interface{}) hclog.Logger {
	return &hclogLogger{Logger: h.Logger, args: args, name: h.name}
}

// Create a logger that will prepend the name string on the front of all messages.
// If the logger already has a name, the new value will be appended to the current
// name. That way, a major subsystem can use this to decorate all it's own logs
// without losing context.
func (h hclogLogger) Named(name string) hclog.Logger {
	return &hclogLogger{Logger: h.Logger, args: h.args, name: name + h.name}
}

// Create a logger that will prepend the name string on the front of all messages.
// This sets the name of the logger to the value directly, unlike Named which honor
// the current name as well.
func (h hclogLogger) ResetNamed(name string) hclog.Logger {
	return &hclogLogger{Logger: h.Logger, args: h.args, name: name}
}

func (h *hclogLogger) convertLevel(level hclog.Level) LogLevel {
	switch level {
	case hclog.Debug:
		return LogLevelDebug
	case hclog.Info:
		return LogLevelInfo
	case hclog.Warn:
		return LogLevelWarn
	case hclog.Error:
		return LogLevelError
	default:
		return LogLevelInfo
	}
}
func (h *hclogLogger) SetLevel(level hclog.Level) {
	h.Logger.SetLogLevel(h.convertLevel(level))
}

func (h hclogLogger) StandardLogger(opts *hclog.StandardLoggerOptions) *log.Logger {
	if opts == nil {
		opts = &hclog.StandardLoggerOptions{}
	}

	return log.New(h.StandardWriter(opts), "", 0)
}

func (h hclogLogger) StandardWriter(opts *hclog.StandardLoggerOptions) io.Writer {
	return &stdlogAdapter{log: h.Logger, inferLevels: opts.InferLevels}
}
