package lucima

import (
	"io"
	"log"
	"net"
	"sync"
	"time"
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

func (ps *proxyServer) serve() {
	for {
		downstream, err := ps.listener.Accept()
		if err != nil {
			return
		}
		originConn, err := net.DialTimeout("tcp", ps.originAddr, time.Second)
		if err != nil {
			log.Println(err)
			downstream.Close()
			return
		}
		clientPort := remotePort(downstream)
		if err = Cli.ClientConnected(ps.getClientName(), clientPort); err != nil {
			log.Printf("%v, clinetPort:%s\n", err, clientPort)
			return
		}
		upstream, err := newConnection(originConn, clientPort, ps.proxyPort)
		if err != nil {
			log.Println(err)
			downstream.Close()
			originConn.Close()
			return
		}
		go handleCopy(downstream, upstream)
	}
}

func handleCopy(downstream, upstream net.Conn) {
	done := make(chan bool)
	go func() {
		n, err := io.Copy(downstream, upstream)
		if err != nil {
			log.Println(n, err)
		}
		done <- true
	}()
	n, err := io.Copy(upstream, downstream)
	if err != nil {
		log.Println(n, err)
	}
	<-done
	upstream.Close()
	downstream.Close()
	if err := Cli.ClientDisconnected(remotePort(downstream)); err != nil {
		log.Println(err)
	}
}
