package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/bingoohuang/hraftd"
	"github.com/hashicorp/raft"
)

func main() {
	arg := hraftd.DefineFlags(flag.CommandLine)

	flag.Parse()
	arg.Fix()
	log.Printf("Args:%s\n", hraftd.Hjson(arg))

	arg.ApplyInterceptor = func(_ *raft.Log, cmd hraftd.Command) bool {
		fmt.Printf("received command %+v\n", cmd)

		return false
	}

	h := hraftd.Create(arg)
	if err := h.RegisterJobDealer("/myjob", myJob, (*JobReq)(nil)); err != nil {
		log.Fatalf("failed to register /myjob, error %v\n", err)
	}

	go leaderChanging(h)

	if err := h.StartAll(); err != nil {
		log.Fatalf("failed to start HTTP service: %s", err.Error())
	}

	log.Println("hraftd started successfully")
	hraftd.WaitInterrupt()
	log.Println("hraftd exiting")
}

func leaderChanging(h *hraftd.Service) {
	tik := hraftd.NewTicker(10*time.Second, func() { // nolint gomnd
		cluster, err := h.RaftCluster()
		if err != nil {
			fmt.Printf("h.Store.Cluster error %v\n", err)
		} else {
			tick(cluster)
		}
	})

	for leader := range h.LeaderCh {
		if leader {
			tik.StartAsync()
		} else {
			tik.StopAsync()
		}
	}
}

func tick(c hraftd.RaftCluster) {
	availableServers := make([]hraftd.Peer, 0, len(c.Servers))

	for _, server := range c.Servers {
		if server.State == hraftd.StateLeader || server.State == "Follower" {
			availableServers = append(availableServers, server)
		}
	}

	serverLen := len(availableServers)

	// demo 10 jobs
	for i := 0; i < 10; i++ {
		req := JobReq{ID: fmt.Sprintf("ID：%d", i)}

		if serverLen > 0 {
			jobIndex := i % serverLen
			peer := availableServers[jobIndex]
			r := &JobRsp{}
			err := peer.DistributeJob("/myjob", req, r)

			if err != nil {
				fmt.Printf("DistributeJob error %v\n", err)
			} else {
				fmt.Printf("DistributeJob successfully, rsp :%+v\n", r)
			}
		} else {
			rsp, err := myJob(&req)
			if err != nil {
				fmt.Printf("process locally error %v\n", err)
			} else {
				fmt.Printf("process locally successfully, rsp :%+v\n", rsp.(JobRsp))
			}
		}
	}
}

type JobReq struct {
	ID string `json:"id"`
}

type JobRsp struct {
	OK  string `json:"ok"`
	Msg string `json:"msg"`
}

func myJob(req interface{}) (interface{}, error) {
	r := req.(*JobReq)

	return JobRsp{
		OK:  "OK",
		Msg: r.ID + " is processed",
	}, nil
}
