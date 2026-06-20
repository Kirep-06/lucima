package lucima

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
)

type ApiServer struct {
	mu   sync.RWMutex
	topo *topology
}

func NewApiServer() *ApiServer {
	topo, _ := newTopology(bytes.NewReader(DefaultConfig))
	return &ApiServer{topo: topo}
}

func (s *ApiServer) postConfig(w http.ResponseWriter, r *http.Request) {
	topo, err := newTopology(r.Body)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	s.mu.Lock()
	if s.topo != nil {
		close(s.topo.getUpdateChannel())
	}
	s.topo = topo
	s.mu.Unlock()
}

func (s *ApiServer) nodeState(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	state, err := s.topo.nodeState(name)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if r.Method == "GET" {
		data, _ := json.Marshal(state)
		w.Write(data)
	} else if r.Method == "POST" {
		var newState NodeState
		if err := json.NewDecoder(r.Body).Decode(&newState); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		s.topo.setNodeState(name, newState)
	}
}

func (s *ApiServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/config" && r.Method == "POST" {
		s.postConfig(w, r)
		return
	}
	s.mu.Lock()
	topo := s.topo
	s.mu.Unlock()
	if topo == nil {
		http.Error(w, "server uninitialized.", 400)
		return
	}
	switch r.URL.Path {
	case "/nodeState":
		s.nodeState(w, r)
	default:
		w.WriteHeader(404)
	}
}
