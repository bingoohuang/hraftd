package main

import (
	"encoding/json"
	"flag"
	"log"

	"github.com/bingoohuang/hraftd/model"

	"github.com/bingoohuang/hraftd/httpd"
)

func main() {
	arg := model.DefineFlags()

	flag.Parse()

	model.FixRaftArg(arg)

	argJSON, _ := json.Marshal(arg)
	log.Printf("Args:%s\n", argJSON)

	h := httpd.Create(arg)
	if err := h.Start(); err != nil {
		log.Fatalf("failed to start HTTP service: %s", err.Error())
	}

	log.Println("hraftd started successfully")

	model.WaitInterrupt()
	log.Println("hraftd exiting")
}
