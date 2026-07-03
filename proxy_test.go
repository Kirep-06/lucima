package lucima

import (
	"net"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
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
