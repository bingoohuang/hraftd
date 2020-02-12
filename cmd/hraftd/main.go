package main

import (
	"flag"
	"fmt"
	"log"

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
	go leaderChanging(h.Store.LeaderCh())

	if err := h.Start(); err != nil {
		log.Fatalf("failed to start HTTP service: %s", err.Error())
	}

	log.Println("hraftd started successfully")
	model.WaitInterrupt()
	log.Println("hraftd exiting")
}

func leaderChanging(c <-chan bool) {
	for leader := range c {
		log.Printf("hraftd leadership changed to %v\n", leader)
	}
}
