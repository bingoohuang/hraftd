package hraftd

import "time"

// Store is the interface Raft-backed key-value stores must implement.
type Store interface {
	// Get returns the value for the given key.
	Get(key string) (string, bool)

	// IsLeader tells the current node is raft leader or not.
	IsLeader() bool

	// Set sets the value for the given key, via distributed consensus.
	Set(key, value string) error

	// Delete removes the given key, via distributed consensus.
	Delete(key string) error

	// Join joins the node, identified by nodeID and reachable at addr, to the cluster.
	Join(nodeID string, addr string) error

	// Remove removes node from the cluster
	Remove(nodeID string) error

	// RaftStats returns the raft stats
	RaftStats() map[string]interface{}

	// Cluster returns the raft cluster servers
	Cluster() (RaftCluster, error)

	// LeadServer returns the raft lead server
	LeadServer() (Peer, error)

	// WaitForLeader blocks until a leader is detected, or the timeout expires.
	WaitForLeader(timeout time.Duration) (string, error)

	// WaitForApplied waits for all Raft log entries to to be applied to the
	// underlying database.
	WaitForApplied(timeout time.Duration) error

	// LeaderCh is used to get a channel which delivers signals on
	// acquiring or losing leadership. It sends true if we become
	// the leader, and false if we lose it. The channel is not buffered,
	// and does not block on writes.
	LeaderCh() <-chan bool

	// NodeState returns the state of current node
	NodeState() string
}
