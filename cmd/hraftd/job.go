package main

type JobReq struct {
	ID string `json:"id"`
}

type JobRspData struct {
	OK  string `json:"ok"`
	Msg string `json:"msg"`
}

// Rsp defines the Raft join response
type JobRsp struct {
	OK   bool       `json:"ok"`
	Msg  string     `json:"msg,omitempty"`
	Data JobRspData `json:"data,omitempty"`
}

func myJob(req interface{}) (interface{}, error) {
	r := req.(*JobReq)

	return JobRspData{
		OK:  "OK",
		Msg: r.ID + " is processed",
	}, nil
}
