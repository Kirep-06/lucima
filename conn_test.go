package lucima

import "testing"

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
