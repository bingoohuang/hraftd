package model

import "github.com/hashicorp/raft"

// Peer defines the peers information
type Peer struct {
	Address string `json:"address"`
	NodeID  NodeID `json:"nodeID"`
	State   string `json:"state"`
}

// RaftCluster is raft cluster
type RaftCluster struct {
	Current Peer   `json:"current"`
	Leader  Peer   `json:"leader"`
	Servers []Peer `json:"servers"`
}

// JoinRequest defines the Raft join request
type JoinRequest struct {
	RemoteAddr string `json:"addr"`
	NodeID     NodeID `json:"id"`
}

// Rsp defines the Raft join response
type Rsp struct {
	OK   bool        `json:"ok"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

// Store is the interface Raft-backed key-value stores must implement.
type Store interface {
	// Get returns the value for the given key.
	Get(key string) (string, bool, error)

	// IsLeader tells the current node is raft leader or not.
	IsLeader() bool

	// Set sets the value for the given key, via distributed consensus.
	Set(key, value string) error

	// Delete removes the given key, via distributed consensus.
	Delete(key string) error

	// Join joins the node, identified by nodeID and reachable at addr, to the cluster.
	Join(nodeID string, addr string) error

	// RaftStats returns the raft stats
	RaftStats() map[string]string

	// RaftServers returns the raft cluster servers
	RaftServers() (RaftCluster, error)

	// LeaderCh is used to get a channel which delivers signals on
	// acquiring or losing leadership. It sends true if we become
	// the leader, and false if we lose it. The channel is not buffered,
	// and does not block on writes.
	LeaderCh() <-chan bool

	// NodeState returns the state of current node
	NodeState() string
}

// Command defines raft log value's structure
type Command struct {
	Op    string `json:"op,omitempty"`
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
	Time  string `json:"time"`
}

type ApplyInterceptor func(l *raft.Log, cmd Command) bool
