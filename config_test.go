package lucima

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func TestConfigDecode(t *testing.T) {
	const input = `{
		"HostDefault":{"Latency":1000000},
		"DataCenters":[{"Name":"animal","Racks":[{
			"Name":"land","Hosts":[{"Name":"lion","Ports":[8585]}]
		}]}]
	}`
	var config Config
	if err := json.Unmarshal([]byte(input), &config); err != nil {
		t.Fatal(err)
	}
	if config.HostDefault == nil || config.HostDefault.Latency != time.Millisecond {
		t.Fatalf("unexpected host default: %+v", config.HostDefault)
	}
	if len(config.DataCenters) != 1 || config.DataCenters[0].Name != "animal" {
		t.Fatalf("unexpected data centers: %+v", config.DataCenters)
	}
	host := config.DataCenters[0].Racks[0].Hosts[0]
	if host.Name != "lion" || len(host.Ports) != 1 || host.Ports[0] != 8585 {
		t.Fatalf("unexpected host: %+v", host)
	}
}

func TestDefaultConfig(t *testing.T) {
	topo, err := newTopology(bytes.NewReader(DefaultConfig))
	if err != nil {
		t.Fatal(err)
	}
	if len(topo.dataCenterMap) != 3 {
		t.Fatalf("got %d data centers, want 3", len(topo.dataCenterMap))
	}
	count := 0
	for _, dc := range topo.dataCenterMap {
		if len(dc.rackMap) != 3 || dc.Latency != 100*time.Millisecond {
			t.Fatalf("unexpected data center: %+v", dc)
		}
		for _, rack := range dc.rackMap {
			if len(rack.hostMap) != 3 || rack.Latency != 10*time.Millisecond {
				t.Fatalf("unexpected rack: %+v", rack)
			}
			count += len(rack.hostMap)
		}
	}
	if count != 27 {
		t.Fatalf("got %d hosts, want 27", count)
	}
}
