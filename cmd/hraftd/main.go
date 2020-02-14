package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/bingoohuang/hraftd/util"

	"github.com/hashicorp/raft"

	"github.com/bingoohuang/hraftd/model"

	"github.com/bingoohuang/hraftd/httpd"
)

func main() {
	arg := model.DefineFlags()

	flag.Parse()
	arg.Fix()
	log.Printf("Args:%s\n", util.Hjson(arg))

	arg.ApplyInterceptor = func(_ *raft.Log, cmd model.Command) bool {
		fmt.Printf("received command %+v\n", cmd)

		return false
	}

	h := httpd.Create(arg)
	if err := h.RegisterTaskDealer("/myjob", myJob, (*JobReq)(nil)); err != nil {
		log.Fatalf("failed to register /myjob, error %v\n", err)
	}

	go leaderChanging(h)

	if err := h.Start(); err != nil {
		log.Fatalf("failed to start HTTP service: %s", err.Error())
	}

	log.Println("hraftd started successfully")
	model.WaitInterrupt()
	log.Println("hraftd exiting")
}

func leaderChanging(h *httpd.Service) {
	c := h.LeaderCh

	tik := util.NewTicker(10*time.Second, func() { // nolint gomnd
		cluster, err := h.RaftCluster()
		if err != nil {
			fmt.Printf("h.Store.Cluster error %v\n", err)
		} else {
			tick(cluster)
		}
	})

	for leader := range c {
		if leader {
			tik.Start()
		} else {
			tik.Stop()
		}
	}
}

func tick(c model.RaftCluster) {
	availableServers := make([]model.Peer, 0, len(c.Servers))

	for _, server := range c.Servers {
		if server.State == model.StateLeader || server.State == "Follower" {
			availableServers = append(availableServers, server)
		}
	}

	// demo 10 jobs
	for i := 0; i < 10; i++ {
		jobIndex := i % len(availableServers)
		peer := availableServers[jobIndex]

		r := &JobRsp{}
		peer.DispatchJob("/myjob", JobReq{ID: fmt.Sprintf("ID：%d", i)}, r)
	}
}
