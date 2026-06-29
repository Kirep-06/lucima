package lucima

import (
	"bytes"
	"io"
	"net"
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
