package lucima

import "time"

// ConnState describes the simulated state of a connection.
type ConnState struct {
	Latency time.Duration
	OK      bool
}

// NodeState describes the simulated state of a topology node.
type NodeState struct {
	Latency      time.Duration
	InternalDown bool
	ExternalDown bool
}

type Config struct {
	DcDefault   *NodeState
	RackDefault *NodeState
	HostDefault *NodeState
	DataCenters []*DataCenter
}

type DataCenter struct {
	RackDefault *NodeState
	HostDefault *NodeState
	Name        string
	Racks       []*Rack
	*NodeState
}

type Rack struct {
	HostDefault *NodeState
	Name        string
	Hosts       []*Host
	*NodeState
}

type Host struct {
	Name  string
	Ports []int
	*NodeState
}
