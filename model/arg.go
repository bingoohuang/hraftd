package model

import (
	"flag"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
)

// Arg Command line parameters
type Arg struct {
	Bootstrap bool

	InMem    bool
	RaftAddr string
	RaftDir  string
	NodeID   string
	HTTPAddr string
	JoinAddr string
}

// DefineFlags define raft args
func DefineFlags() *Arg {
	var app Arg

	flag.BoolVar(&app.InMem, "rmem", false, "Use in-memory storage for Raft")
	flag.StringVar(&app.HTTPAddr, "haddr", ":11000", "HTTP bind address")
	flag.StringVar(&app.RaftAddr, "raddr", ":12000", "Raft bind address")
	flag.StringVar(&app.RaftDir, "rdir", "", "Raft data directory, default to ~/.raftdir/{id}")
	flag.StringVar(&app.JoinAddr, "rjoin", "", "Set raft cluster join address, if any")
	flag.StringVar(&app.NodeID, "rid", "", "Node ID, default to {ip}:{raddr port}")

	return &app
}

// WaitInterrupt waits on interrupt signal
func WaitInterrupt() {
	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	<-terminate
}

// FixRaftArg fixes the arg for some defaults.
func FixRaftArg(arg *Arg) {
	parseFlagRafNodeID(arg)
	parseFlagRaftDir(arg)
	parseBootstrap(arg)
}

func parseFlagRafNodeID(app *Arg) {
	if app.NodeID == "" {
		app.NodeID = app.RaftAddr
	}
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

func parseBootstrap(arg *Arg) {
	arg.Bootstrap = arg.JoinAddr == "" || arg.JoinAddr == arg.HTTPAddr
}
