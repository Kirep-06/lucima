package lucima

import (
	"net/http"
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

func TestApiClientConnState(t *testing.T) {
	api := NewApiServer()
	server := httptest.NewServer(api)
	defer server.Close()
	client := &ApiClient{ApiAddr: strings.TrimPrefix(server.URL, "http://")}
	if err := client.ServerStarted("animal.land.lion", "8585"); err != nil {
		t.Fatal(err)
	}
	if err := client.ClientConnected("animal.land.tiger", "32768"); err != nil {
		t.Fatal(err)
	}
	state, err := client.ConnState("32768", "8585", nil)
	if err != nil || state == nil || !state.OK || state.Latency != 2_000_000 {
		t.Fatalf("initial connection state: %+v, %v", state, err)
	}
	if err := api.topo.setNodeState("animal.land.tiger", NodeState{InternalDown: true}); err != nil {
		t.Fatal(err)
	}
	updated, err := client.ConnState("32768", "8585", state)
	if err != nil || updated == nil || updated.OK || updated.Latency != tcpTimeOut {
		t.Fatalf("updated connection state: %+v, %v", updated, err)
	}
}

func TestApiClientConnStateNotModified(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == "" {
			t.Error("old state header was not sent")
		}
		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()
	client := &ApiClient{ApiAddr: strings.TrimPrefix(server.URL, "http://")}
	old := &ConnState{OK: true}
	state, err := client.ConnState("1", "2", old)
	if err != nil || state != old {
		t.Fatalf("304 should return the same pointer: %+v, %v", state, err)
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
