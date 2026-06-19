package lucima

import (
	"bytes"
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
	w.WriteHeader(404)
}
