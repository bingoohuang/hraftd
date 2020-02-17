package httpd

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/bingoohuang/hraftd/model"
	"github.com/bingoohuang/hraftd/util"
)

func (s *Service) handleKeyRequest(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		if key, ok := getKey(r, w); ok {
			if v, ok := s.store.Get(key); ok {
				util.WriteAsJSON(map[string]string{key: v}, w)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}
	case "POST":
		s.tryForwardToLeader(s.doKeyPost, w, r)
	case "DELETE":
		if key, ok := getKey(r, w); ok {
			s.tryForwardToLeader(func(w http.ResponseWriter, r *http.Request) {
				if err := s.store.Delete(key); err != nil {
					w.WriteHeader(http.StatusInternalServerError)
				}
			}, w, r)
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func getKey(r *http.Request, w http.ResponseWriter) (string, bool) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 3 { // nolint gomnd
		w.WriteHeader(http.StatusBadRequest)
		return "", false
	}

	return parts[2], true
}

func (s *Service) doKeyPost(w http.ResponseWriter, r *http.Request) {
	// Read the value from the POST body.
	m := map[string]string{}
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	for k, v := range m {
		if err := s.store.Set(k, v); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}

// handleRemove handles cluster-remove requests.
func (s *Service) handleRemove(w http.ResponseWriter, r *http.Request) error {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	m := map[string]string{}
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	remoteID, ok := m["id"]
	if !ok {
		return errors.New("ID not found")
	}

	if err := s.store.Remove(remoteID); err != nil {
		return err
	}

	util.WriteAsJSON(model.Rsp{OK: true, Msg: "OK"}, w)

	return nil
}
