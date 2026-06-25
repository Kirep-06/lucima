package lucima

import (
	"bytes"
	"net"
	"sync"
	"time"
)

var NumOfPackets = 128

const packetSize = 4 * 1024

type connection struct {
	conn          net.Conn
	connState     *ConnState
	readDeadline  time.Time
	writeDeadline time.Time
	mutex         sync.RWMutex
	writePacketCh chan *packet
	readPacketCh  chan *packet
	readBuffer    bytes.Buffer
	readErr       error
	writeErrCh    chan error
	pool          packetPool
	closeCh       chan struct{}
	updateCh      chan struct{}
	updateErrCh   chan error
	oldState      *ConnState
	clientPort    string
	serverPort    string
}

type packet struct {
	data     []byte
	length   int
	sentTime int64
	err      error
}

type packetPool chan *packet

func (pp packetPool) get() *packet {
	select {
	case p := <-pp:
		return p
	default:
		return &packet{data: make([]byte, packetSize)}
	}
}

func (pp packetPool) put(p *packet) {
	select {
	case pp <- p:
	default:
	}
}
