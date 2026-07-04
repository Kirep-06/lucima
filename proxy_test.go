package lucima

import (
	"net"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

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
