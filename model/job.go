package model

import (
	"encoding/json"
	"errors"
	"log"

	"github.com/bingoohuang/hraftd/util"
)

// Rsp defines the JobRsp structure
type JobRsp struct {
	OK   bool            `json:"ok"`
	Msg  string          `json:"msg,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
}

const (
	HraftdRaftPath  = "/hraftd/raft"
	HraftdKeyPath   = "/hraftd/key"
	HraftdDoJobPath = "/hraftd/dojob"
)

func (p Peer) DispatchJob(path string, req interface{}, rsp interface{}) error {
	jobURL := p.ID.URL(HraftdDoJobPath + path)
	log.Printf("dispatch job %+v to %s\n", req, jobURL)

	jobRsp := JobRsp{}
	stateCode, resp, err := util.PostJSON(jobURL, req, &jobRsp)
	log.Printf("job response %d %s\n", stateCode, resp)

	if err != nil {
		log.Printf("joined error %s\n", err.Error())
		return err
	}

	log.Printf("statecode:%d, rsp:%+v\n", stateCode, jobRsp)

	if !jobRsp.OK {
		return errors.New(jobRsp.Msg)
	}

	if err := json.Unmarshal(jobRsp.Data, rsp); err != nil {
		return err
	}

	return nil
}
