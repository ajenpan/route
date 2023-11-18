package tcpv2

import (
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"route/auth"
)

var ErrDisconn = errors.New("socket disconnected")
var ErrInvalidPacket = errors.New("invalid packet")

var DefaultTimeoutSec = 30
var DefaultMinTimeoutSec = 10

type SocketStatus int32

const (
	Disconnected SocketStatus = iota
	Connectting  SocketStatus = iota
	Handshake    SocketStatus = iota
	Connected    SocketStatus = iota
)

type SocketOptions struct {
	ID      string
	Timeout time.Duration
}

type SocketOption func(*SocketOptions)

func NewSocket(conn net.Conn, opts SocketOptions) *Socket {
	if opts.Timeout < time.Duration(DefaultMinTimeoutSec)*time.Second {
		opts.Timeout = time.Duration(DefaultTimeoutSec) * time.Second
	}

	ret := &Socket{
		id:       opts.ID,
		conn:     conn,
		timeOut:  opts.Timeout,
		chWrite:  make(chan *Packet, 100),
		chClosed: make(chan struct{}),
		readcache: &Packet{
			head: newHead(),
			body: make([]byte, MaxPacketBodySize),
		},
		status: Handshake,
	}
	return ret
}

type Socket struct {
	auth.UserInfo
	Meta sync.Map

	conn net.Conn
	id   string

	chWrite  chan *Packet
	chClosed chan struct{}

	timeOut time.Duration

	lastSendAt int64
	lastRecvAt int64

	status SocketStatus

	readcache *Packet
}

func (s *Socket) SessionID() string {
	return s.id
}

func (s *Socket) Send(raw []byte) error {
	if !s.Valid() {
		return ErrDisconn
	}
	if len(raw) == 0 {
		return nil
	}

	for len(raw) > 0 {
		p := &Packet{head: newHead()}
		if len(raw) > MaxPacketBodySize {
			p.SetBody(raw[:MaxPacketBodySize])
			raw = raw[MaxPacketBodySize:]
		} else {
			p.SetBody(raw)
			raw = nil
		}
		p.head.SetType(PacketTypeMessage)
		s.chWrite <- p
	}
	return nil
}

func (s *Socket) Close() error {
	if s == nil {
		return nil
	}
	stat := atomic.SwapInt32((*int32)(&s.status), int32(Disconnected))
	if stat == int32(Disconnected) {
		return nil
	}

	select {
	case <-s.chClosed:
		return nil
	default:
		close(s.chClosed)
	}

	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}
	close(s.chWrite)
	return nil
}

// returns the remote network address.
func (s *Socket) RemoteAddr() net.Addr {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.RemoteAddr()
}

func (s *Socket) LocalAddr() net.Addr {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.LocalAddr()
}

func (s *Socket) Valid() bool {
	return s.Status() == Connected
}

// retrun socket work status
func (s *Socket) Status() SocketStatus {
	return SocketStatus(atomic.LoadInt32((*int32)(&s.status)))
}

func (s *Socket) writeWork() error {
	defer func() {
		close(s.chClosed)
	}()
	for {
		select {
		case <-s.chClosed:
			return nil
		case p := <-s.chWrite:
			if err := writePacket(s.conn, p, s.timeOut); err != nil {
				return err
			}
			s.lastSendAt = time.Now().Unix()
		}
	}
}

func (s *Socket) readWork(recv chan<- *Packet) error {
	defer func() {
		close(s.chClosed)
	}()
	for {
		select {
		case <-s.chClosed:
			return nil
		default:
			// TODO: with pool
			p := NewPacket()
			err := readPacket(s.conn, p, s.timeOut)
			if err != nil {
				return err
			}
			s.lastRecvAt = time.Now().Unix()
			recv <- p
		}
	}
}

func readPacket(conn net.Conn, p *Packet, timeout time.Duration) error {
	if timeout > 0 {
		conn.SetReadDeadline(time.Now().Add(timeout))
	}
	if _, err := io.ReadFull(conn, p.head); err != nil {
		return err
	}
	if !p.head.Valid() {
		return ErrInvalidPacket
	}
	bodylen := p.head.GetBodyLen()
	if bodylen > 0 {
		if int(bodylen) > cap(p.body) {
			p.body = make([]byte, bodylen)
		} else {
			p.body = p.body[:bodylen]
		}
		if _, err := io.ReadFull(conn, p.body); err != nil {
			return err
		}
	}
	return nil
}

func writePacket(conn net.Conn, p *Packet, timeout time.Duration) error {
	if timeout > 0 {
		conn.SetWriteDeadline(time.Now().Add(timeout))
	}
	var err error
	if _, err = conn.Write(p.head); err != nil {
		return err
	}
	if _, err = conn.Write(p.body[:p.head.GetBodyLen()]); err != nil {
		return err
	}
	return nil
}
