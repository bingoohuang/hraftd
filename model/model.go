package model

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/bingoohuang/hraftd/util"
	"github.com/hashicorp/raft"
)

const StateLeader = "Leader"

// Peer defines the peers information
type Peer struct {
	Address  string `json:"address"`
	ID       NodeID `json:"id"`
	State    string `json:"state"`
	Suffrage string `json:"suffrage"`
}

func (p Peer) DispatchJob(path string, req interface{}, rsp interface{}) {
	jobURL := p.ID.URL("/job" + path)
	log.Printf("dispatch job %+v to %s\n", req, jobURL)

	stateCode, resp, err := util.PostJSON(jobURL, req, rsp)
	log.Printf("job response %d %s\n", stateCode, resp)

	if err != nil {
		log.Printf("joined error %s\n", err.Error())
	} else {
		log.Printf("statecode:%d, rsp:%+v\n", stateCode, rsp)
	}
}

// RaftCluster is raft cluster
type RaftCluster struct {
	Current Peer   `json:"current"`
	Leader  Peer   `json:"leader"`
	Servers []Peer `json:"servers"`
}

// JoinRequest defines the Raft join request
type JoinRequest struct {
	Addr   string `json:"addr"`
	NodeID NodeID `json:"id"`
}

// Fix fixes the join request's host
func (r *JoinRequest) Fix(remoteAddr string) {
	remoteHost, _, _ := net.SplitHostPort(remoteAddr)
	host := util.EqualsThen(remoteHost, "127.0.0.1", "")

	_, port, _ := net.SplitHostPort(r.Addr)
	r.Addr = fmt.Sprintf("%s:%s", host, port)
	r.NodeID = r.NodeID.Fix(host)
}

// Rsp defines the Raft join response
type Rsp struct {
	OK   bool        `json:"ok"`
	Msg  string      `json:"msg,omitempty"`
	Data interface{} `json:"data,omitempty"`
}

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

// ApplyInterceptor defines the raft log apply interceptor prototype.
type ApplyInterceptor func(l *raft.Log, cmd Command) bool
