package lucima

import (
	"strings"
	"testing"
	"time"
)

func TestNewTopologyInheritsDefaultsAndRegistersPorts(t *testing.T) {
	const input = `{
		"DcDefault":{"Latency":100000000},
		"RackDefault":{"Latency":10000000},
		"HostDefault":{"Latency":1000000},
		"DataCenters":[{"Name":"animal","Racks":[{"Name":"land",
			"Hosts":[{"Name":"lion","Ports":[8585]}]}]}]
	}`
	topo, err := newTopology(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	dc := topo.dataCenterMap["animal"]
	r := dc.rackMap["land"]
	h := r.hostMap["lion"]
	if dc.Latency != 100*time.Millisecond || r.Latency != 10*time.Millisecond || h.Latency != time.Millisecond {
		t.Fatalf("unexpected latencies: %v, %v, %v", dc.Latency, r.Latency, h.Latency)
	}
	if h.rack != r || r.dataCenter != dc || dc.topo != topo {
		t.Fatal("topology parent links are broken")
	}
	if topo.ports[8585] != h || !h.portMap[8585] {
		t.Fatal("configured server port was not registered")
	}
}
