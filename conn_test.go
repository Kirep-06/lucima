package lucima

import (
	"bytes"
	"io"
	"net"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPacketPool(t *testing.T) {
	pool := make(packetPool, 1)
	first := pool.get()
	if len(first.data) != packetSize {
		t.Fatalf("packet size: %d", len(first.data))
	}
	pool.put(first)
	if got := pool.get(); got != first {
		t.Fatal("pool did not reuse packet")
	}
	pool.put(first)
	pool.put(&packet{data: make([]byte, packetSize)})
	if len(pool) != 1 {
		t.Fatalf("pool kept %d packets, want 1", len(pool))
	}
}

func TestNewConnectionUpdatesState(t *testing.T) {
	api := NewApiServer()
	server := httptest.NewServer(api)
	previous := Cli
	Cli = &ApiClient{ApiAddr: strings.TrimPrefix(server.URL, "http://")}
	if err := Cli.ServerStarted("animal.land.lion", "8585"); err != nil {
		t.Fatal(err)
	}
	if err := Cli.ClientConnected("animal.land.tiger", "32768"); err != nil {
		t.Fatal(err)
	}
	left, right := net.Pipe()
	defer right.Close()
	c, err := newConnection(left, "32768", "8585")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		server.Close()
		select {
		case <-c.updateErrCh:
		case <-time.After(5 * time.Second):
			t.Error("state update loop did not stop after API shutdown")
		}
		c.Close()
		Cli = previous
	}()
	if state := c.getState(); !state.OK || state.Latency != 2_000_000 {
		t.Fatalf("initial state: %+v", state)
	}
	oldUpdate := c.updateCh
	if err := api.topo.setNodeState("animal.land.tiger", NodeState{InternalDown: true}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-oldUpdate:
		if state := c.getState(); state.OK || state.Latency != tcpTimeOut {
			t.Fatalf("updated state: %+v", state)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("connection did not observe topology update")
	}
}

func TestListenerRegistersServerPort(t *testing.T) {
	api := NewApiServer()
	server := httptest.NewServer(api)
	defer server.Close()
	previous := Cli
	Cli = &ApiClient{ApiAddr: strings.TrimPrefix(server.URL, "http://")}
	defer func() { Cli = previous }()
	l, err := Listen("tcp", "127.0.0.1:0", "animal.land.lion")
	if err != nil {
		t.Fatal(err)
	}
	port := listenerPort(l)
	portNum, err := strconv.Atoi(port)
	if err != nil || api.topo.ports[portNum] == nil {
		t.Fatalf("listener port %q was not registered: %v", port, err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := api.topo.dialState("animal.land.tiger", portNum); err != nil {
		t.Fatalf("source topology retains global port mapping: %v", err)
	}
}

func TestDialerRejectsUnavailableServer(t *testing.T) {
	api := NewApiServer()
	server := httptest.NewServer(api)
	defer server.Close()
	previous := Cli
	Cli = &ApiClient{ApiAddr: strings.TrimPrefix(server.URL, "http://")}
	defer func() { Cli = previous }()
	dial := NewDialFunc("animal.land.tiger", 50*time.Millisecond)
	conn, err := dial("tcp", "127.0.0.1:8585")
	if conn != nil || err == nil {
		t.Fatalf("unregistered server dial returned %v, %v", conn, err)
	}
}

func TestConnectionWritePackets(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	c := &connection{
		conn:          left,
		connState:     &ConnState{OK: true},
		pool:          make(packetPool, 10),
		writePacketCh: make(chan *packet, 3),
		writeErrCh:    make(chan error, 1),
		updateErrCh:   make(chan error, 1),
		updateCh:      make(chan struct{}),
		closeCh:       make(chan struct{}),
	}
	go c.writeLoop()
	defer close(c.closeCh)
	want := bytes.Repeat([]byte("a"), packetSize+17)
	result := make(chan []byte, 1)
	go func() {
		buf := make([]byte, len(want))
		_, err := io.ReadFull(right, buf)
		if err == nil {
			result <- buf
		}
	}()
	n, err := c.Write(want)
	if err != nil || n != len(want) {
		t.Fatalf("Write returned %d, %v", n, err)
	}
	select {
	case got := <-result:
		if !bytes.Equal(got, want) {
			t.Fatal("written packets were corrupted")
		}
	case <-time.After(time.Second):
		t.Fatal("packets were not delivered")
	}
}

func TestConnectionReadPacket(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	c := &connection{
		conn:         left,
		connState:    &ConnState{OK: true},
		pool:         make(packetPool, 1),
		readPacketCh: make(chan *packet, 1),
		updateErrCh:  make(chan error, 1),
		updateCh:     make(chan struct{}),
		closeCh:      make(chan struct{}),
	}
	go c.readLoop()
	defer close(c.closeCh)
	want := []byte("hello from the wire")
	go right.Write(want)
	buf := make([]byte, len(want))
	n, err := c.Read(buf)
	if err != nil || n != len(want) || !bytes.Equal(buf[:n], want) {
		t.Fatalf("Read returned %q, %v", buf[:n], err)
	}
}

func TestConnectionAddressesDeadlinesAndClose(t *testing.T) {
	left, right := net.Pipe()
	defer right.Close()
	c := &connection{conn: left, closeCh: make(chan struct{})}
	if c.LocalAddr() != left.LocalAddr() || c.RemoteAddr() != left.RemoteAddr() {
		t.Fatal("underlying addresses were not forwarded")
	}
	deadline := time.Now().Add(time.Second)
	if err := c.SetDeadline(deadline); err != nil || c.readDeadline != deadline || c.writeDeadline != deadline {
		t.Fatalf("SetDeadline failed: %v", err)
	}
	if err := c.SetReadDeadline(deadline.Add(time.Second)); err != nil || c.readDeadline != deadline.Add(time.Second) {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	if err := c.SetWriteDeadline(deadline.Add(2 * time.Second)); err != nil || c.writeDeadline != deadline.Add(2*time.Second) {
		t.Fatalf("SetWriteDeadline failed: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-c.closeCh:
	default:
		t.Fatal("close channel remained open")
	}
}
