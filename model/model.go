package model

// Peer defines the peers information
type Peer struct {
	Address string `json:"address"`
	NodeID  string `json:"nodeID"`
	State   string `json:"state"`
}

// RaftClusterState is state of leader
type RaftClusterState struct {
	Current Peer   `json:"current"`
	Leader  Peer   `json:"leader"`
	Servers []Peer `json:"servers"`
}

// JoinRequest defines the Raft join request
type JoinRequest struct {
	RemoteAddr string `json:"addr"`
	NodeID     string `json:"id"`
}

// JoinResponse defines the Raft join response
type JoinResponse struct {
	OK   bool        `json:"ok"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

const ContentTypeJSON = "application-type/json"

// Store is the interface Raft-backed key-value stores must implement.
type Store interface {
	// Get returns the value for the given key.
	Get(key string) (string, bool, error)

	// Set sets the value for the given key, via distributed consensus.
	Set(key, value string) error

	// Delete removes the given key, via distributed consensus.
	Delete(key string) error

	// Join joins the node, identified by nodeID and reachable at addr, to the cluster.
	Join(nodeID string, addr string) error

	// RaftStats returns the raft stats
	RaftStats() map[string]string

	// Status returns the leader state
	Status() (RaftClusterState, error)

	// LeaderCh is used to get a channel which delivers signals on
	// acquiring or losing leadership. It sends true if we become
	// the leader, and false if we lose it. The channel is not buffered,
	// and does not block on writes.
	LeaderCh() <-chan bool
}
