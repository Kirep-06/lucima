package lucima

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	serverPortType = true
	clientPortType = false
	tcpTimeOut     = 15 * time.Minute
	dialTimeOut    = 3 * time.Minute
)

type dataCenter struct {
	name    string
	rackMap map[string]*rack
	topo    *topology
	NodeState
}

type node interface {
	state() NodeState
	setState(NodeState)
	String() string
}

func (dc *dataCenter) String() string {
	var racks []*rack
	for _, r := range dc.rackMap {
		racks = append(racks, r)
	}
	return fmt.Sprintf("\nname:%v internalDown:%v externalDown:%v latency:%v racks:%v",
		dc.name, dc.InternalDown, dc.ExternalDown, dc.Latency, racks)
}

func (r *rack) String() string {
	var hosts []*host
	for _, h := range r.hostMap {
		hosts = append(hosts, h)
	}
	return fmt.Sprintf("\n\tname:%v internalDown:%v externalDown:%v latency:%v hosts:%v",
		r.name, r.InternalDown, r.ExternalDown, r.Latency, hosts)
}

func (h *host) String() string {
	var ports []int
	for port := range h.portMap {
		ports = append(ports, port)
	}
	return fmt.Sprintf("\n\t\tname:%v internalDown:%v externalDown:%v latency:%v ports:\n\t\t\t%v",
		h.name, h.InternalDown, h.ExternalDown, h.Latency, ports)
}

func (topo *topology) String() string {
	var dcs []*dataCenter
	for _, dc := range topo.dataCenterMap {
		dcs = append(dcs, dc)
	}
	return fmt.Sprint(dcs)
}

func (dc *dataCenter) state() NodeState { return dc.NodeState }
func (r *rack) state() NodeState        { return r.NodeState }
func (h *host) state() NodeState        { return h.NodeState }

func (dc *dataCenter) setState(state NodeState) {
	if state.Latency == 0 {
		state.Latency = dc.Latency
	}
	dc.NodeState = state
}

func (r *rack) setState(state NodeState) {
	if state.Latency == 0 {
		state.Latency = r.Latency
	}
	r.NodeState = state
}

func (h *host) setState(state NodeState) {
	if state.Latency == 0 {
		state.Latency = h.Latency
	}
	h.NodeState = state
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

func (topo *topology) lookup(fullName string) (node, error) {
	parts := strings.Split(fullName, ".")
	dc := topo.dataCenterMap[parts[0]]
	if dc == nil {
		err := errors.New("undefined dataCenter name," + parts[0])
		log.Println(err)
		return nil, err
	}
	if len(parts) < 2 {
		return dc, nil
	}
	r := dc.rackMap[parts[1]]
	if r == nil {
		err := errors.New("undefined rack name," + parts[1])
		log.Println(err)
		return nil, err
	}
	if len(parts) < 3 {
		return r, nil
	}
	h := r.hostMap[parts[2]]
	if h == nil {
		err := errors.New("undefined host name," + parts[2])
		log.Println(err)
		return nil, err
	}
	return h, nil
}

func (topo *topology) lookupHost(name string) (*host, error) {
	n, err := topo.lookup(name)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	h, ok := n.(*host)
	if !ok {
		err = errors.New("invalid host name")
		log.Println(err)
		return nil, err
	}
	return h, nil
}

func (topo *topology) setNodeState(name string, state NodeState) error {
	topo.mutex.Lock()
	defer topo.mutex.Unlock()
	n, err := topo.lookup(name)
	if err != nil {
		log.Println(err)
		return err
	}
	n.setState(state)
	close(topo.updateCh)
	topo.updateCh = make(chan struct{})
	return nil
}

func (topo *topology) nodeState(name string) (NodeState, error) {
	topo.mutex.RLock()
	defer topo.mutex.RUnlock()
	n, err := topo.lookup(name)
	if err != nil {
		log.Println(err)
		return NodeState{}, err
	}
	return n.state(), nil
}

func (topo *topology) addServerPort(name string, port int) error {
	topo.mutex.Lock()
	defer topo.mutex.Unlock()
	h, err := topo.lookupHost(name)
	if err != nil {
		log.Println(err)
		return err
	}
	if _, exists := h.portMap[port]; exists {
		err = errors.New("port has been taken already.")
		log.Println(err)
		return err
	}
	h.portMap[port] = serverPortType
	topo.ports[port] = h
	close(topo.updateCh)
	topo.updateCh = make(chan struct{})
	return nil
}

func (topo *topology) removeServerPort(name string, port int) error {
	topo.mutex.Lock()
	defer topo.mutex.Unlock()
	h, err := topo.lookupHost(name)
	if err != nil {
		log.Println(err)
		return err
	}
	portType, exists := h.portMap[port]
	if !exists {
		err = errors.New("port hasn't been registered")
		log.Println(err)
		return err
	}
	if portType != serverPortType {
		err = errors.New("port isn't registered as server")
		log.Println(err)
		return err
	}
	delete(h.portMap, port)
	delete(topo.ports, port)
	close(topo.updateCh)
	topo.updateCh = make(chan struct{})
	return nil
}

func (topo *topology) addClientPort(name string, port int) error {
	topo.mutex.Lock()
	defer topo.mutex.Unlock()
	h, err := topo.lookupHost(name)
	if err != nil {
		log.Println(err)
		return err
	}
	if _, exists := h.portMap[port]; exists {
		err = errors.New("port has been taken already.")
		log.Println(err)
		return err
	}
	h.portMap[port] = clientPortType
	topo.ports[port] = h
	return nil
}

func (topo *topology) removeClientPort(port int) error {
	topo.mutex.Lock()
	defer topo.mutex.Unlock()
	h := topo.ports[port]
	if h == nil {
		err := errors.New("unknown port")
		log.Println(err)
		return err
	}
	portType, exists := h.portMap[port]
	if !exists {
		err := errors.New("port hasn't been registered")
		log.Println(err)
		return err
	}
	if portType != clientPortType {
		err := errors.New("port isn't registered as client port")
		log.Println(err)
		return err
	}
	delete(h.portMap, port)
	delete(topo.ports, port)
	return nil
}

func computeNetworkState(clientHost, serverHost *host) (bool, time.Duration) {
	clientSideDown := clientHost.InternalDown
	serverSideDown := serverHost.InternalDown
	var latency time.Duration
	if clientHost != serverHost {
		clientSideDown = clientSideDown || clientHost.ExternalDown || clientHost.rack.InternalDown
		serverSideDown = serverSideDown || serverHost.ExternalDown || serverHost.rack.InternalDown
		latency += clientHost.Latency + serverHost.Latency
		if clientHost.rack != serverHost.rack {
			clientSideDown = clientSideDown || clientHost.rack.ExternalDown || clientHost.rack.dataCenter.InternalDown
			serverSideDown = serverSideDown || serverHost.rack.ExternalDown || serverHost.rack.dataCenter.InternalDown
			latency += clientHost.rack.Latency + serverHost.rack.Latency
			if clientHost.rack.dataCenter != serverHost.rack.dataCenter {
				clientSideDown = clientSideDown || clientHost.rack.dataCenter.ExternalDown
				serverSideDown = serverSideDown || serverHost.rack.dataCenter.ExternalDown
				latency += clientHost.rack.dataCenter.Latency + serverHost.rack.dataCenter.Latency
			}
		}
	}
	return !(clientSideDown || serverSideDown), latency
}

func (topo *topology) dialState(clientName string, serverPort int) (ConnState, error) {
	topo.mutex.RLock()
	defer topo.mutex.RUnlock()
	clientHost, err := topo.lookupHost(clientName)
	if err != nil {
		log.Println(err)
		return ConnState{}, err
	}
	serverHost := topo.ports[serverPort]
	if serverHost == nil {
		err = errors.New("host undefined for address " + strconv.Itoa(serverPort))
		log.Println(err)
		return ConnState{}, err
	}
	ok, latency := computeNetworkState(clientHost, serverHost)
	if !ok {
		return ConnState{Latency: dialTimeOut}, nil
	}
	_, registered := serverHost.portMap[serverPort]
	return ConnState{OK: registered, Latency: latency * 2}, nil
}

func (topo *topology) connState(clientPort, serverPort int) (ConnState, error) {
	topo.mutex.RLock()
	defer topo.mutex.RUnlock()
	clientHost := topo.ports[clientPort]
	if clientHost == nil {
		return ConnState{}, fmt.Errorf("connState:unknown client port %d", clientPort)
	}
	serverHost := topo.ports[serverPort]
	if serverHost == nil {
		return ConnState{}, fmt.Errorf("connState:unknown server port %d", serverPort)
	}
	if _, exists := clientHost.portMap[clientPort]; !exists {
		return ConnState{}, nil
	}
	ok, latency := computeNetworkState(clientHost, serverHost)
	if !ok {
		return ConnState{Latency: tcpTimeOut}, nil
	}
	_, registered := serverHost.portMap[serverPort]
	return ConnState{OK: registered, Latency: latency}, nil
}

func (topo *topology) getUpdateChannel() chan struct{} {
	topo.mutex.RLock()
	updateCh := topo.updateCh
	topo.mutex.RUnlock()
	return updateCh
}

func parsePort(port string) (portNum int, passive bool, err error) {
	portNum, err = strconv.Atoi(port)
	if err != nil {
		log.Println(err)
		return
	}
	if portNum < 0 {
		portNum = -portNum
		passive = true
	}
	return
}
