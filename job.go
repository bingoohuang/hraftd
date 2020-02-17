package hraftd

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"

	"github.com/bingoohuang/gonet"
)

func (s *DealerMap) handleJobRequest(w http.ResponseWriter, r *http.Request) error {
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

// DealerMap keep mapping dealer path to registered dealer
type DealerMap struct {
	Dealers map[string]Dealer
}

// MakeDealerMap makes a DealerMap
func MakeDealerMap() DealerMap {
	return DealerMap{Dealers: make(map[string]Dealer)}
}

// Dealer defines the job dealer structure
type Dealer struct {
	Fn      reflect.Value
	ReqType reflect.Type
}

// Invoke invokes the registered dealer function
func (d Dealer) Invoke(requestBody []byte) (x interface{}, err error) {
	reqType := d.ReqType
	isReqTypePtr := d.ReqType.Kind() == reflect.Ptr

	if isReqTypePtr {
		reqType = d.ReqType.Elem()
	}

	req := reflect.New(reqType)

	if err := json.Unmarshal(requestBody, req.Interface()); err != nil {
		return nil, err
	}

	if !isReqTypePtr {
		req = req.Elem()
	}

	ret := d.Fn.Call([]reflect.Value{req})

	if ret[1].IsNil() {
		err = nil
	} else {
		err = ret[1].Interface().(error)
	}

	return ret[0].Interface(), err
}

// RegisterJobDealer registers path dealers
func (s *DealerMap) RegisterJobDealer(jobPath string, dealer interface{}) error {
	if _, ok := s.Dealers[jobPath]; ok {
		return errors.New("dealer [] " + jobPath + " already registered")
	}

	fn, err := checkDealerFn(dealer)
	if err != nil {
		return err
	}

	s.Dealers[jobPath] = Dealer{Fn: fn, ReqType: fn.Type().In(0)}

	return nil
}

func checkDealerFn(f interface{}) (reflect.Value, error) {
	fn := reflect.ValueOf(f)
	fnType := reflect.TypeOf(f)

	if fnType.Kind() != reflect.Func {
		return fn, errors.New("function required")
	}

	if !(fnType.NumIn() == 1 && fnType.NumOut() == 2) {
		return fn, errors.New("function must have 1 parameter and return like (x,error)")
	}

	errorInterface := reflect.TypeOf((*error)(nil)).Elem()

	if fnType.Out(1) != errorInterface { // nolint gomnd
		return fn, errors.New("function must return like（x, error)")
	}

	return fn, nil
}
