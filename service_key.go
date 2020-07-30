package hraftd

import (
	"errors"
	"io/ioutil"
	"net/http"
	"strings"

	jsoniter "github.com/json-iterator/go"
)

func (s *Service) handleKeyRequest(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		if key, ok := getKey(r, w); ok {
			if v, ok := s.Store.Get(key); ok {
				WriteAsJSON(map[string]interface{}{key: TryParseJSON(v)}, w)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}
	case "POST":
		s.tryForwardToLeader(s.doKeyPost, w, r)
	case "DELETE":
		if key, ok := getKey(r, w); ok {
			s.tryForwardToLeader(func(w http.ResponseWriter, r *http.Request) {
				if err := s.Store.Delete(key); err != nil {
					w.WriteHeader(http.StatusInternalServerError)
				}
			}, w, r)
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func getKey(r *http.Request, w http.ResponseWriter) (string, bool) {
	key := strings.TrimPrefix(r.URL.Path, KeyPath)
	key = strings.TrimPrefix(key, "/")

	if key != "" {
		return key, true
	}

	w.WriteHeader(http.StatusBadRequest)

	return "", false
}

// Set sets the key-value to the raft log.
func (s *Service) Set(k, v string) error {
	return s.Store.Set(k, v)
}

func (s *Service) doKeyPost(w http.ResponseWriter, r *http.Request) {
	// Read the value from the POST body.
	m := map[string]string{}
	if err := jsoniter.NewDecoder(r.Body).Decode(&m); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	for k, v := range m {
		if err := s.Store.Set(k, v); err != nil {
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
	if err := jsoniter.Unmarshal(b, &m); err != nil {
		return err
	}

	remoteID, ok := m["id"]
	if !ok {
		return errors.New("ID not found") // nolint:goerr113
	}

	if err := s.Store.Remove(remoteID); err != nil {
		return err
	}

	WriteAsJSON(Rsp{OK: true, Msg: "OK"}, w)

	return nil
}
