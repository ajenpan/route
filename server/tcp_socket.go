package server

import (
	"errors"
	"net"
	"route/auth"
	"sync"
	"sync/atomic"
	"time"
)

var ErrDisconn = errors.New("socket disconnected")
var ErrInvalidPacket = errors.New("invalid packet")

var DefaultTimeoutSec = 30
var DefaultMinTimeoutSec = 10

type tcpSocketOptions struct {
	ID      string
	Timeout time.Duration
}

type tcpSocketOption func(*tcpSocketOptions)

func NewTcpSocket(conn net.Conn, opts tcpSocketOptions) *tcpSocket {
	if opts.Timeout < time.Duration(DefaultMinTimeoutSec)*time.Second {
		opts.Timeout = time.Duration(DefaultTimeoutSec) * time.Second
	}

	ret := &tcpSocket{
		id:       opts.ID,
		conn:     conn,
		timeOut:  opts.Timeout,
		chWrite:  make(chan Packet, 100),
		chClosed: make(chan struct{}),

		status: Handshake,
	}
	return ret
}

type tcpSocket struct {
	auth.UserInfo
	Meta sync.Map

	conn net.Conn
	id   string

	chWrite  chan Packet
	chClosed chan struct{}

	timeOut time.Duration

	lastSendAt int64
	lastRecvAt int64

	status SessionStatus
}

func (s *tcpSocket) SessionID() string {
	return s.id
}

func (s *tcpSocket) SessionType() string {
	return "tcp"
}

func (s *tcpSocket) Send(p Packet) error {
	if !s.IsValid() {
		return ErrDisconn
	}
	select {
	case <-s.chClosed:
		return ErrDisconn
	case s.chWrite <- p:
	}
	return nil
}

func (s *tcpSocket) Close() error {
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
func (s *tcpSocket) RemoteAddr() net.Addr {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.RemoteAddr()
}

func (s *tcpSocket) LocalAddr() net.Addr {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.LocalAddr()
}

func (s *tcpSocket) IsValid() bool {
	return s.Status() == Connected
}

// retrun socket work status
func (s *tcpSocket) Status() SessionStatus {
	return SessionStatus(atomic.LoadInt32((*int32)(&s.status)))
}

func (s *tcpSocket) writeWork() error {
	defer func() {
		s.Close()
	}()
	for {
		select {
		case <-s.chClosed:
			return nil
		case p, ok := <-s.chWrite:
			if !ok {
				return nil
			}
			if err := WritePacket(s.conn, s.timeOut, p); err != nil {
				return err
			}
			s.lastSendAt = time.Now().Unix()
		}
	}
}

func (s *tcpSocket) readWork(recv chan<- Packet) error {
	defer func() {
		s.Close()
	}()
	for {
		select {
		case <-s.chClosed:
			return nil
		default:
			p, err := ReadPacket(s.conn, s.timeOut)
			if err != nil {
				return err
			}
			s.lastRecvAt = time.Now().Unix()
			recv <- p
		}
	}
}

func ReadPacket(conn net.Conn, timeout time.Duration) (Packet, error) {
	if timeout > 0 {
		conn.SetReadDeadline(time.Now().Add(timeout))
	}
	var err error
	pktype := make([]byte, 1)
	_, err = conn.Read(pktype)
	if err != nil {
		return nil, err
	}
	pk := NewPacket(pktype[0])
	if pk == nil {
		return nil, ErrInvalidPacket
	}
	_, err = pk.ReadFrom(conn)
	return pk, err
}

func ReadPacketT[PacketTypeT Packet](conn net.Conn, timeout time.Duration) (PacketTypeT, error) {
	pk, err := ReadPacket(conn, timeout)
	var pkk PacketTypeT
	if err != nil {
		return pkk, err
	}
	pkk, ok := pk.(PacketTypeT)
	if !ok {
		return pkk, ErrInvalidPacket
	}
	return pkk, nil
}

func WritePacket(conn net.Conn, timeout time.Duration, p Packet) error {
	if timeout > 0 {
		conn.SetWriteDeadline(time.Now().Add(timeout))
	}
	var err error
	_, err = conn.Write([]byte{p.PacketType()})
	if err != nil {
		return err
	}
	_, err = p.WriteTo(conn)
	return err
}
