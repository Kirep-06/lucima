package lucima

import (
	"log"
	"net"
	"sync"
)

type proxyServer struct {
	mu         sync.RWMutex
	clientName string
	proxyName  string
	proxyPort  string
	originAddr string
	listener   net.Listener
}

func newProxyServer(clientName, proxyName, proxyPort, originAddr string) (*proxyServer, error) {
	ps := &proxyServer{
		clientName: clientName,
		proxyName:  proxyName,
		proxyPort:  proxyPort,
		originAddr: originAddr,
	}
	var err error
	ps.listener, err = net.Listen("tcp", "localhost:"+ps.proxyPort)
	if err != nil {
		log.Println("failed to listen proxy", err)
		return ps, err
	}
	if err = Cli.ServerStarted(ps.proxyName, ps.proxyPort); err != nil {
		log.Println("at newProxyServer", err)
		return ps, err
	}
	return ps, nil
}

func (ps *proxyServer) close() error {
	ps.listener.Close()
	return Cli.ServerStopped(ps.proxyName, ps.proxyPort)
}

func (ps *proxyServer) setClientName(name string) {
	ps.mu.Lock()
	ps.clientName = name
	ps.mu.Unlock()
}

func (ps *proxyServer) getClientName() string {
	ps.mu.RLock()
	name := ps.clientName
	ps.mu.RUnlock()
	return name
}
