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

func TestTopologyNodeState(t *testing.T) {
	const input = `{"DcDefault":{"Latency":100},"DataCenters":[{"Name":"animal","Racks":[{"Name":"land","Hosts":[{"Name":"lion"}]}]}]}`
	topo, err := newTopology(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := topo.lookupHost("animal.land"); err == nil {
		t.Fatal("rack should not be accepted as a host")
	}
	oldUpdate := topo.updateCh
	if err := topo.setNodeState("animal", NodeState{ExternalDown: true}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-oldUpdate:
	default:
		t.Fatal("state update did not notify listeners")
	}
	state, err := topo.nodeState("animal")
	if err != nil || !state.ExternalDown || state.Latency != 100 {
		t.Fatalf("unexpected state: %+v, %v", state, err)
	}
	if err := topo.setNodeState("missing", NodeState{}); err == nil {
		t.Fatal("unknown node should be rejected")
	}
}

func TestServerPortLifecycle(t *testing.T) {
	topo, err := newTopology(strings.NewReader(`{"DataCenters":[{"Name":"animal","Racks":[{"Name":"land","Hosts":[{"Name":"lion"}]}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	const name = "animal.land.lion"
	oldUpdate := topo.updateCh
	if err := topo.addServerPort(name, 8585); err != nil {
		t.Fatal(err)
	}
	if topo.ports[8585] != topo.dataCenterMap["animal"].rackMap["land"].hostMap["lion"] {
		t.Fatal("server port did not resolve to the host")
	}
	select {
	case <-oldUpdate:
	default:
		t.Fatal("adding a server port did not notify listeners")
	}
	if err := topo.addServerPort(name, 8585); err == nil {
		t.Fatal("duplicate port was accepted")
	}
	oldUpdate = topo.updateCh
	if err := topo.removeServerPort(name, 8585); err != nil {
		t.Fatal(err)
	}
	if topo.ports[8585] != nil {
		t.Fatal("removed server port remained in global map")
	}
	select {
	case <-oldUpdate:
	default:
		t.Fatal("removing a server port did not notify listeners")
	}
	if err := topo.removeServerPort(name, 8585); err == nil {
		t.Fatal("unregistered port was removed twice")
	}
}

func TestClientPortLifecycle(t *testing.T) {
	topo, err := newTopology(strings.NewReader(`{"DataCenters":[{"Name":"animal","Racks":[{"Name":"land","Hosts":[{"Name":"lion"}]}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	const name = "animal.land.lion"
	oldUpdate := topo.updateCh
	if err := topo.addClientPort(name, 32768); err != nil {
		t.Fatal(err)
	}
	h := topo.dataCenterMap["animal"].rackMap["land"].hostMap["lion"]
	if topo.ports[32768] != h || h.portMap[32768] != clientPortType {
		t.Fatal("client port was not registered")
	}
	if oldUpdate != topo.updateCh {
		t.Fatal("client port update should not notify listeners")
	}
	if err := topo.addClientPort(name, 32768); err == nil {
		t.Fatal("duplicate port was accepted")
	}
	if err := topo.removeClientPort(32768); err != nil {
		t.Fatal(err)
	}
	if topo.ports[32768] != nil {
		t.Fatal("client port remained in global port map")
	}
	if err := topo.removeClientPort(32768); err == nil {
		t.Fatal("unknown port was accepted")
	}
}

func TestComputeNetworkState(t *testing.T) {
	const input = `{"DcDefault":{"Latency":100},"RackDefault":{"Latency":10},"HostDefault":{"Latency":1},"DataCenters":[{"Name":"animal","Racks":[{"Name":"land","Hosts":[{"Name":"lion"},{"Name":"tiger"}]},{"Name":"sea","Hosts":[{"Name":"shark"}]}]},{"Name":"plant","Racks":[{"Name":"fruit","Hosts":[{"Name":"apple"}]}]}]}`
	topo, err := newTopology(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	lion := topo.dataCenterMap["animal"].rackMap["land"].hostMap["lion"]
	tiger := topo.dataCenterMap["animal"].rackMap["land"].hostMap["tiger"]
	shark := topo.dataCenterMap["animal"].rackMap["sea"].hostMap["shark"]
	apple := topo.dataCenterMap["plant"].rackMap["fruit"].hostMap["apple"]
	for _, tc := range []struct {
		name    string
		server  *host
		latency time.Duration
	}{
		{"same host", lion, 0},
		{"same rack", tiger, 2},
		{"same data center", shark, 22},
		{"different data centers", apple, 222},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ok, latency := computeNetworkState(lion, tc.server)
			if !ok || latency != tc.latency {
				t.Fatalf("got ok=%v latency=%v; want ok=true latency=%v", ok, latency, tc.latency)
			}
		})
	}
	topo.dataCenterMap["plant"].ExternalDown = true
	if ok, _ := computeNetworkState(lion, apple); ok {
		t.Fatal("external outage should disconnect different data centers")
	}
	if ok, _ := computeNetworkState(apple, apple); !ok {
		t.Fatal("external outage should not disconnect the same host")
	}
}

func TestDialAndConnectionState(t *testing.T) {
	const input = `{"HostDefault":{"Latency":5},"DataCenters":[{"Name":"animal","Racks":[{"Name":"land","Hosts":[{"Name":"lion","Ports":[8585]},{"Name":"tiger"}]}]}]}`
	topo, err := newTopology(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	const client = "animal.land.tiger"
	if err := topo.addClientPort(client, 32768); err != nil {
		t.Fatal(err)
	}
	dial, err := topo.dialState(client, 8585)
	if err != nil || dial != (ConnState{OK: true, Latency: 20}) {
		t.Fatalf("dial state: %+v, %v", dial, err)
	}
	conn, err := topo.connState(32768, 8585)
	if err != nil || conn != (ConnState{OK: true, Latency: 10}) {
		t.Fatalf("connection state: %+v, %v", conn, err)
	}
	if err := topo.setNodeState(client, NodeState{InternalDown: true}); err != nil {
		t.Fatal(err)
	}
	dial, err = topo.dialState(client, 8585)
	if err != nil || dial != (ConnState{Latency: dialTimeOut}) {
		t.Fatalf("failed dial state: %+v, %v", dial, err)
	}
	conn, err = topo.connState(32768, 8585)
	if err != nil || conn != (ConnState{Latency: tcpTimeOut}) {
		t.Fatalf("failed connection state: %+v, %v", conn, err)
	}
}

func TestParsePort(t *testing.T) {
	for _, tc := range []struct {
		input   string
		port    int
		passive bool
		bad     bool
	}{
		{"8585", 8585, false, false},
		{"-8585", 8585, true, false},
		{"0", 0, false, false},
		{"bad", 0, false, true},
	} {
		port, passive, err := parsePort(tc.input)
		if port != tc.port || passive != tc.passive || (err != nil) != tc.bad {
			t.Errorf("parsePort(%q) = %d, %v, %v", tc.input, port, passive, err)
		}
	}
}
