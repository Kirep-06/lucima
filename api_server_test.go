package lucima

import (
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
