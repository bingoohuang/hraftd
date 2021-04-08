package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/averagesecurityguy/random"

	"github.com/sirupsen/logrus"

	"github.com/spf13/pflag"

	"github.com/spf13/viper"

	"github.com/bingoohuang/faker"
	"github.com/bingoohuang/golog"
	"github.com/bingoohuang/hraftd"
	"github.com/hashicorp/raft"
	funk "github.com/thoas/go-funk"
)

const rigConf = "rigconf"

func main() {
	arg := hraftd.DefineFlags(flag.CommandLine)

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	_ = viper.BindPFlags(pflag.CommandLine)

	golog.SetupLogrus()

	arg.LoggerMore = hraftd.NewLogrusAdapter(logrus.StandardLogger())
	arg.Fix()
	arg.Printf("Args:%s", hraftd.Jsonify4Print(arg))

	arg.ApplyInterceptor = func(_ *raft.Log, cmd hraftd.Command) bool {
		arg.Printf("received command %s", hraftd.Jsonify4Print(cmd))

		return false
	}

	_ = arg.RegisterLogDealer(rigConf, func(items []RigConfItem) (interface{}, error) {
		meItems := funk.Filter(items, func(i RigConfItem) bool { return i.NodeID == arg.NodeID }).([]RigConfItem)
		if len(meItems) == 0 {
			return nil, nil
		}

		arg.Printf("received config items %s", hraftd.Jsonify4Print(meItems))
		return nil, nil
	})

	h := hraftd.Create(arg)
	if err := h.RegisterJobDealer("/myjob", myJob); err != nil {
		arg.Panicf("failed to register /myjob, error %v", err)
	}

	go leaderChanging(arg, h)

	if err := h.StartAll(); err != nil {
		arg.Panicf("failed to start HTTP service: %s", err.Error())
	}

	arg.Printf("hraftd started successfully")
	hraftd.WaitInterrupt()
	arg.Printf("hraftd exiting")
}

// RigConfItem defines 配置项.
type RigConfItem struct {
	Time   string        `json:"time"`
	ID     int64         `json:"id"`
	Name   string        `json:"name" faker:"len=30"`
	NodeID hraftd.NodeID `json:"node_id"`
}

func leaderChanging(logger hraftd.LevelLogger, h *hraftd.Service) {
	tik := hraftd.NewTicker(10*time.Second, true, func() { // nolint:gomnd
		cluster, err := h.RaftCluster()
		if err != nil {
			logger.Printf("h.Store.Cluster error %v", err)
		} else {
			tick(logger, h, cluster)
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

func tick(logger hraftd.LevelLogger, h *hraftd.Service, c hraftd.RaftCluster) {
	activePeers := c.ActivePeers()

	demoApplyLogs(logger, activePeers, h)
	demoDistributeJobs(logger, activePeers)
}

func demoDistributeJobs(logger hraftd.LevelLogger, activePeers []hraftd.Peer) {
	serverLen := len(activePeers)

	// demo 10 jobs
	for i := 0; i < 10; i++ {
		req := JobReq{ID: fmt.Sprintf("ID：%d", i)}

		if serverLen > 0 { // nolint:nestif
			jobIndex := i % serverLen
			peer := activePeers[jobIndex]
			r := &JobRsp{}
			err := peer.DistributeJob(logger, "/myjob", req, r)

			if err != nil {
				logger.Printf("distribute job error %v", err)
			} else {
				logger.Printf("distribute job successfully, rsp :%+v", r)
			}
		} else {
			rsp, err := myJob(&req)
			if err != nil {
				logger.Printf("process locally error %v", err)
			} else {
				logger.Printf("process locally successfully, rsp :%+v", rsp)
			}
		}
	}
}

// IntN returns a random int
func RandIntN(n uint64) int {
	i, _ := random.Uint64Range(0, n)

	return int(i)
}

func demoApplyLogs(logger hraftd.LevelLogger, activePeers []hraftd.Peer, h *hraftd.Service) {
	items := make([]RigConfItem, 0)
	// demo applying log
	for _, peer := range activePeers {
		for i := 1 + RandIntN(3); i > 0; i-- { // nolint:gomnd
			item := RigConfItem{}
			_ = faker.Fake(&item)
			item.Time = time.Now().Format(`2006-01-02 15:04:05.000`)
			item.NodeID = peer.ID

			items = append(items, item)
		}
	}

	logger.Printf("create items %s", hraftd.Jsonify4Print(items))

	if err := h.Set(rigConf, hraftd.Jsonify(items)); err != nil {
		logger.Printf("fail to set rigConf, error %v", err)
	}
}

// JobReq defines the Job Request structure.
type JobReq struct {
	ID string `json:"id"`
}

// JobRsp defines the Job Response structure.
type JobRsp struct {
	OK  string `json:"ok"`
	Msg string `json:"msg"`
}

func myJob(req *JobReq) (JobRsp, error) {
	return JobRsp{OK: "OK", Msg: req.ID + " is processed"}, nil
}
