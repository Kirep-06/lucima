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

func TestApiClientDialState(t *testing.T) {
	api := NewApiServer()
	server := httptest.NewServer(api)
	defer server.Close()
	client := &ApiClient{ApiAddr: strings.TrimPrefix(server.URL, "http://")}
	if err := client.ServerStarted("animal.land.lion", "8585"); err != nil {
		t.Fatal(err)
	}
	state, err := client.DialState("animal.land.tiger", "8585")
	if err != nil || !state.OK || state.Latency != 4_000_000 {
		t.Fatalf("dial state: %+v, %v", state, err)
	}
	if _, err := client.DialState("missing", "8585"); err == nil {
		t.Fatal("unknown client should return API error")
	}
}
