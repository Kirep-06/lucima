package lucima

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestApiClientRegistersPorts(t *testing.T) {
	api := NewApiServer()
	server := httptest.NewServer(api)
	defer server.Close()
	client := &ApiClient{ApiAddr: strings.TrimPrefix(server.URL, "http://")}
	if err := client.ServerStarted("animal.land.lion", "8585"); err != nil {
		t.Fatal(err)
	}
	if err := client.ClientConnected("plant.fruit.apple", "32768"); err != nil {
		t.Fatal(err)
	}
	if api.topo.ports[8585] == nil || api.topo.ports[32768] == nil {
		t.Fatal("ports were not registered")
	}
	if err := client.ClientDisconnected("32768"); err != nil {
		t.Fatal(err)
	}
	if err := client.ServerStopped("animal.land.lion", "8585"); err != nil {
		t.Fatal(err)
	}
	if err := client.ServerStopped("animal.land.lion", "8585"); err == nil {
		t.Fatal("duplicate removal should return an API error")
	}
}
