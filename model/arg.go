package model

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bingoohuang/hraftd/util"
)

// Arg Command line parameters
type Arg struct {
	Bootstrap bool

	InMem     bool
	RaftAddr  string
	RaftAdv   string
	RaftDir   string
	NodeID    NodeID
	HTTPAddr  string
	HTTPAdv   string
	JoinAddrs string

	ApplyInterceptor ApplyInterceptor `json:"-"`
	JoinAddrSlice    []string
}

// DefineFlags define raft args
func DefineFlags() *Arg {
	var app Arg

	flag.BoolVar(&app.InMem, "rmem", false, "Use in-memory storage for Raft")
	flag.StringVar(&app.HTTPAddr, "haddr", ":11000", "HTTP server bind address")
	flag.StringVar(&app.HTTPAdv, "hadv", "", "Advertised HTTP address. If not set, same as HTTP server")
	flag.StringVar(&app.RaftAddr, "raddr", "", "Raft communication bind address. If not set, same as haddr(port+1000)")
	flag.StringVar(&app.RaftAdv, "radv", "", "Advertised Raft communication address. If not set, same as Raft bind")
	flag.StringVar(&app.RaftDir, "rdir", "", "Raft data directory, default to ~/.raftdir/{id}")
	flag.StringVar(&app.JoinAddrs, "rjoin", "", "Set raft cluster join addresses separated by comma, if any")

	return &app
}

// WaitInterrupt waits on interrupt signal
func WaitInterrupt() {
	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	<-terminate
}

// FixRaftArg fixes the arg for some defaults.
func (a *Arg) FixRaftArg() {
	a.fixAddr()
	a.parseFlagRaftNodeID()
	a.parseFlagRaftDir()
	a.parseBootstrap()
}

func (a *Arg) fixAddr() {
	if a.RaftAddr == "" {
		host, port, err := net.SplitHostPort(a.HTTPAddr)
		if err != nil {
			panic(err)
		}

		por, _ := strconv.Atoi(port)

		a.RaftAddr = fmt.Sprintf("%s:%d", host, por+1000) // nolint gomnd
	}
}

// NodeID is the raft node ID
type NodeID string

// BindAddr is the address for bind
type BindAddr string

// URL returns the HTTP access URL with relative path
func (a BindAddr) URL(path string) string {
	host, port, _ := net.SplitHostPort(string(a))
	if host == "" {
		host = "127.0.0.1"
	}

	path = strings.TrimPrefix(path, "/")

	return fmt.Sprintf("http://%s:%s/%s", host, port, path)
}

// URL returns the HTTP access URL with relative path
func (r NodeID) URL(relativePath string) string { return BindAddr(r.HTTPAddr()).URL(relativePath) }

// URLRaftState is http://httpAddr/raft/join
func (r NodeID) URLRaftState() string { return r.URL("/raft/state") }

// URLRaftJoin is http://httpAddr/raft/join
func (a BindAddr) URLRaftJoin() string { return a.URL("/raft/join") }

// HTTPAddr returns the HTTP bind address in the NodeID
func (r NodeID) HTTPAddr() string { return strings.SplitN(string(r), ",", -1)[0] }

// RaftAddr returns the Raft bind addr in the NodeID
func (r NodeID) RaftAddr() string { return strings.SplitN(string(r), ",", -1)[1] }

func (a *Arg) parseFlagRaftNodeID() { a.NodeID = NodeID(a.HTTPAddr + "," + a.RaftAddr) }

// nolint gomnd
func (a *Arg) parseFlagRaftDir() {
	if a.RaftDir == "" {
		basePath := "./"
		if usr, err := user.Current(); err == nil {
			basePath = usr.HomeDir
		}

		a.RaftDir = filepath.Join(basePath, ".raftdir", string(a.NodeID))
	}

	_ = os.MkdirAll(a.RaftDir, 0700)
}

func (a *Arg) parseBootstrap() {
	a.JoinAddrSlice = make([]string, 0)

	if a.JoinAddrs == "" {
		a.Bootstrap = true

		return
	}

	for _, addr := range strings.Split(a.JoinAddrs, ",") {
		if addr != "" {
			a.JoinAddrSlice = append(a.JoinAddrSlice, addr)
		}
	}

	if len(a.JoinAddrSlice) == 0 || a.JoinAddrSlice[0] == a.HTTPAddr {
		a.Bootstrap = true
	}
}

// Join joins the current not to raft cluster
func (a *Arg) Join() error {
	addrLen := len(a.JoinAddrSlice)
	if addrLen == 0 {
		return nil
	}

	for i := 0; i < 10; i++ {
		if i > 0 {
			log.Printf("retry after 10s\n")
			time.Sleep(10 * time.Second) // nolint gomnd
		}

		joinURL := BindAddr(a.JoinAddrSlice[i%addrLen]).URLRaftJoin()
		log.Printf("joinURL %s\n", joinURL)

		r := &Rsp{}
		stateCode, resp, err := util.PostJSON(joinURL, JoinRequest{Addr: a.RaftAddr, NodeID: a.NodeID}, r)
		log.Printf("join response %d %s\n", stateCode, resp)

		if err != nil {
			log.Printf("joined error %s\n", err.Error())

			continue
		}

		if r.OK {
			return nil
		}
	}

	return fmt.Errorf("failed to join %s", a.JoinAddrs)
}
