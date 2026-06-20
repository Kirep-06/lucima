package lucima

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestApiServerConfig(t *testing.T) {
	server := NewApiServer()
	if len(server.topo.dataCenterMap) != 3 {
		t.Fatal("default topology was not loaded")
	}
	oldUpdate := server.topo.getUpdateChannel()
	request := httptest.NewRequest(http.MethodPost, "/config", strings.NewReader(`{"DataCenters":[{"Name":"new"}]}`))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != 200 || server.topo.dataCenterMap["new"] == nil {
		t.Fatalf("config update failed: %d, %s", response.Code, response.Body.String())
	}
	select {
	case <-oldUpdate:
	default:
		t.Fatal("config update did not notify listeners")
	}
	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/config", strings.NewReader(`{`)))
	if response.Code != 400 || server.topo.dataCenterMap["new"] == nil {
		t.Fatalf("invalid config should leave topology unchanged: %d", response.Code)
	}
}

func TestApiServerNodeState(t *testing.T) {
	server := NewApiServer()
	get := httptest.NewRecorder()
	server.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/nodeState?name=animal.land.lion", nil))
	if get.Code != 200 {
		t.Fatalf("GET failed: %d, %s", get.Code, get.Body.String())
	}
	var state NodeState
	if err := json.Unmarshal(get.Body.Bytes(), &state); err != nil || state.Latency != 1_000_000 {
		t.Fatalf("unexpected state: %+v, %v", state, err)
	}
	post := httptest.NewRecorder()
	server.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/nodeState?name=animal.land.lion", strings.NewReader(`{"ExternalDown":true}`)))
	if post.Code != 200 {
		t.Fatalf("POST failed: %d, %s", post.Code, post.Body.String())
	}
	updated, err := server.topo.nodeState("animal.land.lion")
	if err != nil || !updated.ExternalDown || updated.Latency != state.Latency {
		t.Fatalf("unexpected updated state: %+v, %v", updated, err)
	}
	bad := httptest.NewRecorder()
	server.ServeHTTP(bad, httptest.NewRequest(http.MethodPost, "/nodeState?name=missing", strings.NewReader(`{}`)))
	if bad.Code != 400 {
		t.Fatalf("unknown node status: %d", bad.Code)
	}
}
