package hraftd

import (
	"encoding/json"
	"errors"
)

// JobRsp defines the Job Response structure.
type JobRsp struct {
	OK   bool            `json:"ok"`
	Msg  string          `json:"msg,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
}

// DistributeJob distributes job to the peer node in the raft clusters.
func (p Peer) DistributeJob(logger LevelLogger, path string, req interface{}, rsp interface{}) error {
	jobURL := p.ID.URL(DoJobPath + path)
	logger.Printf("dispatch job %s to %s", Jsonify4Print(req), jobURL)

	jobRsp := JobRsp{}
	stateCode, resp, err := PostJSON(jobURL, req, &jobRsp)
	logger.Printf("dispatch job response %d %s", stateCode, resp)

	if err != nil {
		logger.Printf("fail to post job error %s", err.Error())
		return err
	}

	logger.Printf("dispatch job stateCode:%d, rsp OK: %v, Msg:%s", stateCode, jobRsp.OK, jobRsp.Msg)

	if !jobRsp.OK {
		return errors.New(jobRsp.Msg)
	}

	return json.Unmarshal(jobRsp.Data, rsp)
}
