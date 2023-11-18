package tcp

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
		chWrite:  make(chan []byte, 100),
		chClosed: make(chan bool),
	}
	return ret
}

type Socket struct {
	auth.UserInfo

	Meta sync.Map

	conn net.Conn
	id   string

	chWrite  chan []byte
	chClosed chan bool

	timeOut time.Duration

	lastSendAt int64
	lastRecvAt int64

	status SocketStatus
}

func (s *Socket) SessionID() string {
	return s.id
}

func (s *Socket) Send(p []byte) error {
	if !s.Valid() {
		return ErrDisconn
	}
	s.chWrite <- p
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
	var err error
	for {
		select {
		case <-s.chClosed:
			return err
		case p := <-s.chWrite:
			if s.timeOut > 0 {
				s.conn.SetWriteDeadline(time.Now().Add(s.timeOut))
			}
			if _, err = s.conn.Write(p); err != nil {
				return err
			}
			s.lastSendAt = time.Now().Unix()
		}
	}
}

func (s *Socket) readWork(p []byte) ([]byte, error) {
	if !s.Valid() {
		return p, ErrDisconn
	}
	var err error
	select {
	case <-s.chClosed:
		break
	default:
		if s.timeOut > 0 {
			s.conn.SetReadDeadline(time.Now().Add(s.timeOut))
		}
		if _, err = io.ReadFull(s.conn, p); err != nil {
			break
		}
		s.lastRecvAt = time.Now().Unix()
	}
	return p, err
}
