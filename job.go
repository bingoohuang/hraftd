package hraftd

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"

	"github.com/bingoohuang/gonet"
)

func (s *Service) handleJobRequest(w http.ResponseWriter, r *http.Request) error {
	path := strings.TrimPrefix(r.URL.String(), DoJobPath)
	dealer, ok := s.Dealers[path]

	if !ok {
		return errors.New("dealer [] " + path + " not found")
	}

	body := gonet.ReadBytes(r.Body)
	rsp, err := dealer.Invoke(body)

	if err != nil {
		return err
	}

	WriteAsJSON(Rsp{OK: true, Msg: "ok", Data: rsp}, w)

	return nil
}

// DealerFn defines the job dealer function prototype.
type DealerFn func(req interface{}) (interface{}, error)

// Dealer defines the job dealer structure
type Dealer struct {
	Fn      DealerFn
	ReqType reflect.Type
}

// Invoke invokes the registered dealer function
func (d Dealer) Invoke(requestBody []byte) (interface{}, error) {
	reqType := d.ReqType

	if d.ReqType.Kind() == reflect.Ptr {
		reqType = d.ReqType.Elem()
	}

	req := reflect.New(reqType)

	if err := json.Unmarshal(requestBody, req.Interface()); err != nil {
		return nil, err
	}

	if d.ReqType.Kind() == reflect.Ptr {
		return d.Fn(req.Interface())
	}

	return d.Fn(req.Elem().Interface())
}

// RegisterJobDealer registers path dealers
func (s *Service) RegisterJobDealer(jobPath string, dealer DealerFn, req interface{}) error {
	if _, ok := s.Dealers[jobPath]; ok {
		return errors.New("dealer [] " + jobPath + " already registered")
	}

	s.Dealers[jobPath] = Dealer{Fn: dealer, ReqType: reflect.TypeOf(req)}

	return nil
}
