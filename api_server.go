package lucima

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type ApiServer struct {
	mu      sync.RWMutex
	topo    *topology
	proxies map[string]*proxyServer
}

func NewApiServer() *ApiServer {
	topo, _ := newTopology(bytes.NewReader(DefaultConfig))
	return &ApiServer{topo: topo, proxies: make(map[string]*proxyServer)}
}

func (s *ApiServer) getProxy(port string) *proxyServer {
	s.mu.RLock()
	ps := s.proxies[port]
	s.mu.RUnlock()
	return ps
}

func (s *ApiServer) setProxy(port string, ps *proxyServer) {
	s.mu.Lock()
	s.proxies[port] = ps
	s.mu.Unlock()
}

func (s *ApiServer) DumpNode(name string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if name == "" {
		fmt.Println(s.topo)
		return
	}
	n, err := s.topo.lookup(name)
	if err != nil {
		log.Println(err)
		return
	}
	fmt.Println(n)
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

func (s *ApiServer) connState(w http.ResponseWriter, r *http.Request) {
	clientPort := intFormValue(r, "clientPort")
	if clientPort == 0 {
		http.Error(w, "'clientPort' required", 400)
		return
	}
	serverPort := intFormValue(r, "serverPort")
	if serverPort == 0 {
		http.Error(w, "'serverPort' required", 400)
		return
	}
	s.mu.Lock()
	topo := s.topo
	s.mu.Unlock()
	updateCh := topo.getUpdateChannel()
	state, err := topo.connState(clientPort, serverPort)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), 400)
		return
	}
	oldStateStr := r.Header.Get("If-None-Match")
	if oldStateStr == "" {
		data, _ := json.Marshal(state)
		w.Write(data)
		return
	}
	var oldState ConnState
	if err := json.Unmarshal([]byte(oldStateStr), &oldState); err != nil {
		log.Println(err)
		http.Error(w, "invalid If-None-Match header", 400)
		return
	}
	if oldState == state {
		select {
		case <-time.After(3 * time.Second):
		case <-updateCh:
			state, _ = topo.connState(clientPort, serverPort)
		}
	}
	if oldState == state {
		w.WriteHeader(304)
	} else {
		data, _ := json.Marshal(state)
		w.Write(data)
	}
}

func (s *ApiServer) proxy(w http.ResponseWriter, r *http.Request) {
	proxyPort := r.FormValue("proxyPort")
	if proxyPort == "" {
		http.Error(w, "'proxyPort' required", 400)
		return
	}
	ps := s.getProxy(proxyPort)
	var err error
	switch r.Method {
	case "POST":
		if ps != nil {
			message := "proxy port is taken"
			log.Println(message)
			http.Error(w, message, 400)
			return
		}
		proxyName := r.FormValue("proxyName")
		if proxyName == "" {
			http.Error(w, "'proxyName' required", 400)
			return
		}
		originAddr := r.FormValue("originAddr")
		if originAddr == "" {
			http.Error(w, "'originAddr' required", 400)
			return
		}
		clientName := r.FormValue("clientName")
		if clientName == "" {
			http.Error(w, "'clientName' required", 400)
			return
		}
		ps, err = newProxyServer(clientName, proxyName, proxyPort, originAddr)
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), 400)
			return
		}
		go ps.serve()
		s.setProxy(proxyPort, ps)
	case "PUT":
		if ps == nil {
			message := "proxy server not found"
			log.Println(message)
			http.Error(w, message, 404)
			return
		}
		clientName := r.FormValue("clientName")
		if clientName == "" {
			http.Error(w, "'clientName' required", 400)
			return
		}
		ps.clientName = clientName
	case "DELETE":
		if ps == nil {
			message := "proxy server not found"
			log.Println(message)
			http.Error(w, message, 404)
			return
		}
		err = ps.close()
		if err != nil {
			log.Println(err)
			return
		}
		s.setProxy(proxyPort, nil)
	}
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
	case "/connState":
		s.connState(w, r)
	case "/proxy":
		s.proxy(w, r)
	default:
		w.WriteHeader(404)
	}
}
