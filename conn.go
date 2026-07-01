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

func (c *connection) readLoop() {
	for {
		p := c.pool.get()
		n, err := c.conn.Read(p.data)
		p.err = err
		p.length = n
		p.sentTime = time.Now().UnixNano()
		select {
		case <-c.closeCh:
			return
		case c.readPacketCh <- p:
		}
	}
}

func (c *connection) readPacket(p *packet, b []byte) (n int, err error) {
	for {
		elapsed := time.Now().UnixNano() - p.sentTime
		state := c.getState()
		remaining := state.Latency - time.Duration(elapsed)
		select {
		case <-c.closeCh:
			return 0, errors.New("connection closed")
		case <-time.After(remaining):
			n = copy(b, p.data[:p.length])
			if p.length <= len(b) {
				return n, p.err
			}
			c.mutex.Lock()
			c.readBuffer.Write(p.data[p.length:])
			c.readErr = p.err
			c.mutex.Unlock()
			c.pool.put(p)
			return n, nil
		case <-c.updateCh:
		}
	}
}

func (c *connection) Read(b []byte) (n int, err error) {
	c.mutex.Lock()
	n, _ = c.readBuffer.Read(b)
	if n == 0 {
		err = c.readErr
		c.readErr = nil
	}
	c.mutex.Unlock()
	if n > 0 || err != nil {
		return
	}
	var deadlineTimer <-chan time.Time
	c.mutex.Lock()
	if !c.readDeadline.IsZero() {
		deadlineTimer = time.After(c.readDeadline.Sub(time.Now()))
	}
	c.mutex.Unlock()
	select {
	case p := <-c.readPacketCh:
		n, err = c.readPacket(p, b)
	case err = <-c.updateErrCh:
		log.Println(err)
	case <-c.closeCh:
		err = errors.New("connection closed")
	case <-deadlineTimer:
		err = errors.New("read timeout")
	}
	return
}

func (c *connection) getState() *ConnState {
	c.mutex.RLock()
	state := c.connState
	c.mutex.RUnlock()
	return state
}

func (c *connection) Close() error {
	close(c.closeCh)
	return c.conn.Close()
}

func (c *connection) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *connection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *connection) SetDeadline(t time.Time) error {
	c.mutex.Lock()
	c.writeDeadline = t
	c.readDeadline = t
	c.conn.SetDeadline(t)
	c.mutex.Unlock()
	return nil
}

func (c *connection) SetReadDeadline(t time.Time) error {
	c.mutex.Lock()
	c.readDeadline = t
	c.conn.SetReadDeadline(t)
	c.mutex.Unlock()
	return nil
}

func (c *connection) SetWriteDeadline(t time.Time) error {
	c.mutex.Lock()
	c.writeDeadline = t
	c.conn.SetWriteDeadline(t)
	c.mutex.Unlock()
	return nil
}

func (c *connection) updateLoop() {
	for {
		select {
		case <-c.closeCh:
			return
		default:
			oldState := c.getState()
			newState, err := Cli.ConnState(c.clientPort, c.serverPort, oldState)
			if err != nil {
				log.Println(err)
				c.updateErrCh <- err
				return
			}
			if newState == oldState {
				continue
			}
			c.mutex.Lock()
			close(c.updateCh)
			c.updateCh = make(chan struct{})
			c.connState = newState
			c.mutex.Unlock()
		}
	}
}

func newConnection(conn net.Conn, clientPort, serverPort string) (*connection, error) {
	c := &connection{
		conn:          conn,
		clientPort:    clientPort,
		serverPort:    serverPort,
		pool:          make(packetPool, 10),
		writeErrCh:    make(chan error, 1),
		writePacketCh: make(chan *packet, NumOfPackets),
		readPacketCh:  make(chan *packet, NumOfPackets),
		updateCh:      make(chan struct{}),
		updateErrCh:   make(chan error, 1),
		closeCh:       make(chan struct{}),
	}
	state, err := Cli.ConnState(clientPort, serverPort, nil)
	if err != nil {
		log.Println(err)
		return c, err
	}
	c.connState = state
	go c.writeLoop()
	go c.readLoop()
	go c.updateLoop()
	return c, nil
}
