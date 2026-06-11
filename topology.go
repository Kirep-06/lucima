package lucima

import (
	"encoding/json"
	"io"
	"log"
	"sync"
	"time"
)

const (
	serverPortType = true
	clientPortType = false
)

type dataCenter struct {
	name    string
	rackMap map[string]*rack
	topo    *topology
	NodeState
}

type rack struct {
	name       string
	hostMap    map[string]*host
	dataCenter *dataCenter
	NodeState
}

type host struct {
	name    string
	portMap map[int]bool
	rack    *rack
	NodeState
}

type topology struct {
	HostLatency   time.Duration
	RackLatency   time.Duration
	DcLatency     time.Duration
	ports         map[int]*host
	dataCenterMap map[string]*dataCenter
	mutex         sync.RWMutex
	updateCh      chan struct{}
}

func newDc(conf *DataCenter, topo *topology) *dataCenter {
	dc := &dataCenter{name: conf.Name, rackMap: make(map[string]*rack), topo: topo}
	if conf.NodeState != nil {
		dc.NodeState = *conf.NodeState
	}
	for _, confRack := range conf.Racks {
		if confRack.HostDefault == nil {
			confRack.HostDefault = conf.HostDefault
		}
		if confRack.NodeState == nil {
			confRack.NodeState = conf.RackDefault
		}
		r := newRack(confRack, dc)
		dc.rackMap[r.name] = r
	}
	return dc
}

func newRack(conf *Rack, dc *dataCenter) *rack {
	r := &rack{name: conf.Name, hostMap: make(map[string]*host), dataCenter: dc}
	if conf.NodeState != nil {
		r.NodeState = *conf.NodeState
	}
	for _, confHost := range conf.Hosts {
		if confHost.NodeState == nil {
			confHost.NodeState = conf.HostDefault
		}
		h := newHost(confHost, r)
		r.hostMap[h.name] = h
	}
	return r
}

func newHost(conf *Host, r *rack) *host {
	h := &host{name: conf.Name, portMap: make(map[int]bool), rack: r}
	for _, port := range conf.Ports {
		h.portMap[port] = serverPortType
		r.dataCenter.topo.ports[port] = h
	}
	if conf.NodeState != nil {
		h.NodeState = *conf.NodeState
	}
	return h
}

func newTopology(reader io.Reader) (*topology, error) {
	var config Config
	if err := json.NewDecoder(reader).Decode(&config); err != nil {
		log.Println(err)
		return nil, err
	}
	topo := &topology{
		ports:         make(map[int]*host),
		dataCenterMap: make(map[string]*dataCenter),
		updateCh:      make(chan struct{}),
	}
	for _, confDC := range config.DataCenters {
		if confDC.RackDefault == nil {
			confDC.RackDefault = config.RackDefault
		}
		if confDC.HostDefault == nil {
			confDC.HostDefault = config.HostDefault
		}
		if confDC.NodeState == nil {
			confDC.NodeState = config.DcDefault
		}
		dc := newDc(confDC, topo)
		topo.dataCenterMap[dc.name] = dc
	}
	return topo, nil
}
