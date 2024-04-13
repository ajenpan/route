package server

import (
	"errors"
	"io"
	"net"
	"sync/atomic"
	"time"
)

var ErrDisconn = errors.New("socket disconnected")
var ErrInvalidPacket = errors.New("invalid packet")

var DefaultTimeoutSec = 30
var DefaultMinTimeoutSec = 10

type UserInfo struct {
	UId uint32
}

func (u *UserInfo) UserID() uint32 {
	return u.UId
}

type tcpSocket struct {
	UserInfo
	userData

	conn net.Conn
	id   string

	chWrite  chan Packet
	chRead   chan Packet
	chClosed chan struct{}

	timeOut time.Duration

	lastSendAt int64
	lastRecvAt int64

	status SessionStatus

	writeSize int64
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
		return nil
	}
}

func (s *tcpSocket) Close() error {
	old := atomic.SwapInt32((*int32)(&s.status), int32(Disconnected))
	if old != Connected {
		return nil
	}

	select {
	case <-s.chClosed:
		return nil
	default:
		close(s.chClosed)
		// close(s.chWrite)
		// close(s.chRead)
		return nil
	}
}

// returns the remote network address.
func (s *tcpSocket) RemoteAddr() net.Addr {
	if !s.IsValid() {
		return nil
	}
	return s.conn.RemoteAddr()
}

func (s *tcpSocket) LocalAddr() net.Addr {
	if !s.IsValid() {
		return nil
	}
	return s.conn.LocalAddr()
}

func (s *tcpSocket) IsValid() bool {
	return s.Status() == Connected
}

func (s *tcpSocket) Status() SessionStatus {
	return SessionStatus(atomic.LoadInt32((*int32)(&s.status)))
}

func (s *tcpSocket) writeWork() error {
	for {
		select {
		case <-s.chClosed:
			return nil
		case p, ok := <-s.chWrite:
			if !ok {
				return nil
			}
			s.conn.SetWriteDeadline(time.Now().Add(s.timeOut))
			n, err := WritePacket(s.conn, p)
			if err != nil {
				return err
			}
			s.writeSize += n
			s.lastSendAt = time.Now().Unix()
		}
	}

	// for p := range s.chWrite {
	// 	s.conn.SetWriteDeadline(time.Now().Add(s.timeOut))
	// 	n, err := WritePacket(s.conn, p)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	s.writeSize += n
	// 	s.lastSendAt = time.Now().Unix()
	// }
	// return nil
}

func (s *tcpSocket) readWork() error {
	for {
		s.conn.SetReadDeadline(time.Now().Add(s.timeOut))

		p, err := ReadPacket(s.conn)
		if err != nil {
			return err
		}
		s.lastRecvAt = time.Now().Unix()
		select {
		case <-s.chClosed:
			return nil
		case s.chRead <- p:
		}
	}
}

func ReadPacket(conn io.Reader) (Packet, error) {
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

func ReadPacketT[PacketTypeT Packet](conn io.Reader) (PacketTypeT, error) {
	pk, err := ReadPacket(conn)
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

func WritePacket(conn io.Writer, p Packet) (int64, error) {
	var n int64
	var err error
	_, err = conn.Write([]byte{p.PacketType()})
	if err != nil {
		return 0, err
	}
	n, err = p.WriteTo(conn)
	return n + 1, err
}
