package lucima

import (
	"io"
	"net"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

type remotePortConn struct {
	net.Conn
	remote net.Addr
}

func (c remotePortConn) RemoteAddr() net.Addr { return c.remote }

func TestProxyServerLifecycle(t *testing.T) {
	api := NewApiServer()
	server := httptest.NewServer(api)
	defer server.Close()
	previous := Cli
	Cli = &ApiClient{ApiAddr: strings.TrimPrefix(server.URL, "http://")}
	defer func() { Cli = previous }()
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := strconv.Itoa(probe.Addr().(*net.TCPAddr).Port)
	probe.Close()
	ps, err := newProxyServer("plant.fruit.apple", "animal.land.lion", port, "127.0.0.1:12345")
	if err != nil {
		t.Fatal(err)
	}
	portNum, _ := strconv.Atoi(port)
	if api.topo.ports[portNum] == nil {
		t.Fatal("proxy port was not registered")
	}
	ps.setClientName("plant.fruit.pear")
	if ps.getClientName() != "plant.fruit.pear" {
		t.Fatal("proxy client name was not updated")
	}
	if err := ps.close(); err != nil {
		t.Fatal(err)
	}
}

func TestProxyStopsWhenOriginUnavailable(t *testing.T) {
	api := NewApiServer()
	server := httptest.NewServer(api)
	defer server.Close()
	previous := Cli
	Cli = &ApiClient{ApiAddr: strings.TrimPrefix(server.URL, "http://")}
	defer func() { Cli = previous }()
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	originAddr := probe.Addr().String()
	probe.Close()
	proxyProbe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := strconv.Itoa(proxyProbe.Addr().(*net.TCPAddr).Port)
	proxyProbe.Close()
	ps, err := newProxyServer("plant.fruit.apple", "animal.land.lion", port, originAddr)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { ps.serve(); close(done) }()
	conn, err := net.Dial("tcp", ps.listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("proxy did not stop after origin dial failure")
	}
	ps.close()
}

func TestHandleCopyForwardsAndUnregisters(t *testing.T) {
	api := NewApiServer()
	server := httptest.NewServer(api)
	defer server.Close()
	previous := Cli
	Cli = &ApiClient{ApiAddr: strings.TrimPrefix(server.URL, "http://")}
	defer func() { Cli = previous }()
	if err := Cli.ClientConnected("animal.air.eagle", "32768"); err != nil {
		t.Fatal(err)
	}
	downstream, client := net.Pipe()
	upstream, origin := net.Pipe()
	defer client.Close()
	defer origin.Close()
	done := make(chan struct{})
	go func() {
		handleCopy(remotePortConn{downstream, &net.TCPAddr{Port: 32768}}, upstream)
		close(done)
	}()
	go func() {
		request := make([]byte, 4)
		if _, err := io.ReadFull(origin, request); err != nil {
			return
		}
		origin.Write(request)
		origin.Close()
	}()
	client.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := client.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, 4)
	if _, err := io.ReadFull(client, response); err != nil || string(response) != "ping" {
		t.Fatalf("response %q, %v", response, err)
	}
	client.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("copy did not finish")
	}
	if api.topo.ports[32768] != nil {
		t.Fatal("client port remained registered")
	}
}
