package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"

	"github.com/hashicorp/raft"

	"github.com/bingoohuang/hraftd/model"

	"github.com/bingoohuang/hraftd/httpd"
)

func main() {
	arg := model.DefineFlags()

	flag.Parse()

	model.FixRaftArg(arg)

	argJSON, _ := json.Marshal(arg)
	log.Printf("Args:%s\n", argJSON)

	arg.ApplyInterceptor = func(l *raft.Log, cmd model.Command) bool {
		fmt.Printf("received command %+v\n", cmd)

		return false
	}

	h := httpd.Create(arg)

	go func() {
		for leader := range h.Store.LeaderCh() {
			if leader {
				log.Println("hraftd became leader")
			} else {
				log.Println("hraftd lost leader")
			}
		}
	}()

	if err := h.Start(); err != nil {
		log.Fatalf("failed to start HTTP service: %s", err.Error())
	}

	log.Println("hraftd started successfully")

	model.WaitInterrupt()
	log.Println("hraftd exiting")
}
