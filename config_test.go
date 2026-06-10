package lucima

import (
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
