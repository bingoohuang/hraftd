package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"time"

	"github.com/bingoohuang/gonet"
	"github.com/bingoohuang/hraftd/httpd"
	"github.com/bingoohuang/hraftd/store"
)

// Arg Command line parameters
type Arg struct {
	InMem    bool
	RaftAddr string
	RaftDir  string
	NodeID   string

	HTTPAddr string
	JoinAddr string
}

func flatParse() Arg {
	var app Arg

	flag.BoolVar(&app.InMem, "mem", false, "Use in-memory storage for Raft")
	flag.StringVar(&app.HTTPAddr, "haddr", ":11000", "HTTP bind address")
	flag.StringVar(&app.RaftAddr, "raddr", ":12000", "Raft bind address")
	flag.StringVar(&app.RaftDir, "rdir", "", "Raft data directory, default to ~/.raftdir/{id}")
	flag.StringVar(&app.JoinAddr, "join", "", "Set join address, if any")
	flag.StringVar(&app.NodeID, "id", "", "Node ID, default to {ip}:{raddr port}")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <raft-data-path> \n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	return app
}

func main() {
	arg := flatParse()
	parseFlagRafNodeID(&arg)
	parseFlagRaftDir(&arg)

	argJSON, _ := json.Marshal(arg)
	log.Printf("Args:%s\n", argJSON)

	s := store.New(arg.RaftDir, arg.RaftAddr, arg.InMem)

	bootstrap := parseBootstrap(arg)
	log.Printf("bootstrap parsed to %v\n", bootstrap)

	if err := s.Open(bootstrap, arg.NodeID); err != nil {
		log.Fatalf("failed to open store: %s", err.Error())
	}

	h := httpd.New(arg.HTTPAddr, s)
	if err := h.Start(); err != nil {
		log.Fatalf("failed to start HTTP service: %s", err.Error())
	}

	// If join was specified, make the join request.
	if !bootstrap {
		if err := join(arg.JoinAddr, arg.RaftAddr, arg.NodeID); err != nil {
			log.Fatalf("failed to join node at %s: %s", arg.JoinAddr, err.Error())
		}
	}

	log.Println("hraftd started successfully")

	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	<-terminate

	log.Println("hraftd exiting")
}

func parseBootstrap(arg Arg) bool {
	bootstrap := arg.JoinAddr == "" || arg.JoinAddr == arg.HTTPAddr
	if bootstrap {
		return true
	}

	hhost, hport, _ := net.SplitHostPort(arg.HTTPAddr)
	host, port, _ := net.SplitHostPort(arg.JoinAddr)

	if host == "" {
		host = "127.0.0.1"
	}

	if hhost == "" {
		hhost = "127.0.0.1"
	}

	isLocalHost := func(h string) bool {
		yes, _ := gonet.IsLocalAddr(h)
		return yes
	}

	if hport == port && (hhost == host || isLocalHost(hhost) && isLocalHost(host)) {
		return true
	}

	return false
}

func parseFlagRafNodeID(app *Arg) {
	if app.NodeID != "" {
		return
	}

	_, port, _ := net.SplitHostPort(app.RaftAddr)
	app.NodeID = gonet.ListIpsv4()[0] + ":" + port
}

// nolint gomnd
func parseFlagRaftDir(app *Arg) {
	if app.RaftDir == "" {
		basePath := "./"
		if usr, err := user.Current(); err == nil {
			basePath = usr.HomeDir
		}

		app.RaftDir = filepath.Join(basePath, ".raftdir", app.NodeID)
	}

	_ = os.MkdirAll(app.RaftDir, 0700)
}

func join(joinAddr, raftAddr, nodeID string) error {
	b, _ := json.Marshal(httpd.JoinRequest{RemoteAddr: raftAddr, NodeID: nodeID})
	joinURL := fmt.Sprintf("http://%s/join", joinAddr)

	for i := 0; i < 10; i++ {
		if i > 0 {
			time.Sleep(10 * time.Second) // nolint gomnd
		}

		resp, err := http.Post(joinURL, "application-type/json", bytes.NewReader(b)) // nolint gosec
		if err != nil {
			log.Printf("joined error %v, retry after 10s\n", err)

			continue
		}

		var r httpd.JoinResponse

		rs := gonet.ReadString(resp.Body)
		resp.Body.Close()
		log.Printf("json response %s\n", rs)
		_ = json.Unmarshal([]byte(rs), &r)

		if r.OK {
			return nil
		}
	}

	return fmt.Errorf("failed to join %s", joinURL)
}
