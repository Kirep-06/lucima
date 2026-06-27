package lucima

import (
	"bytes"
	"errors"
	"log"
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

func (c *connection) writeLoop() {
	for {
		select {
		case <-c.closeCh:
			return
		case p := <-c.writePacketCh:
			c.writePacket(p)
		}
	}
}

func (c *connection) writePacket(p *packet) {
	for {
		elapsed := time.Now().UnixNano() - p.sentTime
		remaining := c.connState.Latency - time.Duration(elapsed)
		select {
		case <-c.closeCh:
			return
		case <-c.updateCh:
			continue
		case <-time.After(remaining):
		}
		var err error
		if c.connState.OK {
			_, err = c.conn.Write(p.data[:p.length])
		} else {
			err = errors.New("connection error")
		}
		c.pool.put(p)
		if err != nil {
			select {
			case c.writeErrCh <- err:
			default:
			}
		}
		return
	}
}

func (c *connection) Write(b []byte) (n int, err error) {
	for n < len(b) {
		now := time.Now()
		p := c.pool.get()
		length := copy(p.data, b[n:])
		p.length = length
		p.sentTime = now.UnixNano()
		var deadlineTimer <-chan time.Time
		c.mutex.Lock()
		if !c.writeDeadline.IsZero() {
			deadlineTimer = time.After(c.writeDeadline.Sub(now))
		}
		c.mutex.Unlock()
		select {
		case err = <-c.updateErrCh:
			log.Println(err)
		case err = <-c.writeErrCh:
		case <-deadlineTimer:
			err = errors.New("write timeout")
		case c.writePacketCh <- p:
			n += length
		case <-c.closeCh:
			err = errors.New("connection closed")
		}
		if err != nil {
			return
		}
	}
	return
}
