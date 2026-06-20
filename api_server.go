package lucima

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
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

func (s *ApiServer) serverPort(w http.ResponseWriter, r *http.Request) {
	port := intFormValue(r, "port")
	if port == 0 {
		http.Error(w, "'port' required", 400)
		return
	}
	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "'name' required", 400)
		return
	}
	var err error
	switch r.Method {
	case "POST":
		err = s.topo.addServerPort(name, port)
	case "DELETE":
		err = s.topo.removeServerPort(name, port)
	}
	if err != nil {
		http.Error(w, err.Error(), 400)
	}
}

func (s *ApiServer) clientPort(w http.ResponseWriter, r *http.Request) {
	port := intFormValue(r, "port")
	if port == 0 {
		http.Error(w, "'port' required", 400)
		return
	}
	var err error
	switch r.Method {
	case "POST":
		name := r.FormValue("name")
		if name == "" {
			http.Error(w, "'name' required", 400)
			return
		}
		err = s.topo.addClientPort(name, port)
	case "DELETE":
		err = s.topo.removeClientPort(port)
	}
	if err != nil {
		http.Error(w, err.Error(), 400)
	}
}

func (s *ApiServer) dialState(w http.ResponseWriter, r *http.Request) {
	clientName := r.FormValue("clientName")
	if clientName == "" {
		http.Error(w, "'clientName' required", 400)
		return
	}
	serverPort := intFormValue(r, "serverPort")
	if serverPort == 0 {
		http.Error(w, "'serverPort' required", 400)
		return
	}
	state, err := s.topo.dialState(clientName, serverPort)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	data, _ := json.Marshal(state)
	w.Write(data)
}

func intFormValue(r *http.Request, key string) int {
	value, _ := strconv.Atoi(r.FormValue(key))
	return value
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
	case "/serverPort":
		s.serverPort(w, r)
	case "/clientPort":
		s.clientPort(w, r)
	case "/dialState":
		s.dialState(w, r)
	default:
		w.WriteHeader(404)
	}
}
