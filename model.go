package hraftd

import (
	"fmt"
	"net"

	"github.com/hashicorp/raft"
)

// StateLeader is the state states string for leader.
const StateLeader = "Leader"

// Peer defines the peers information
type Peer struct {
	Address  string `json:"address"`
	ID       NodeID `json:"id"`
	State    string `json:"state"`
	Suffrage string `json:"suffrage"`
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
	host := EqualsThen(remoteHost, "127.0.0.1", "")

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

// Command defines raft log value's structure
type Command struct {
	Op    string `json:"op,omitempty"`
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
	Time  string `json:"time"`
}

// ApplyInterceptor defines the raft log apply interceptor prototype.
type ApplyInterceptor func(l *raft.Log, cmd Command) bool
