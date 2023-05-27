package tcp

import (
	"io"
	"net"
	"sync/atomic"
	"time"

	"route/transport"
)

type SocketStat int32

const (
	Disconnected SocketStat = iota
	Connected    SocketStat = iota
)

var DefaultTimeout = 30 * time.Second
var DefaultMinTimeout = 1 * time.Second

type OnMessageFunc func(*Socket, *PackFrame)
type OnConnStatFunc func(*Socket, SocketStat)
type NewIDFunc func() string

type SocketOptions struct {
	ID      string
	Timeout time.Duration
}

type SocketOption func(*SocketOptions)

func NewSocket(conn net.Conn, opts SocketOptions) *Socket {
	if opts.Timeout < DefaultMinTimeout {
		opts.Timeout = DefaultTimeout
	}

	ret := &Socket{
		id:       opts.ID,
		conn:     conn,
		timeOut:  opts.Timeout,
		chSend:   make(chan *PackFrame, 10),
		chClosed: make(chan bool),
		state:    Connected,
	}
	return ret
}

type Socket struct {
	conn     net.Conn   // low-level conn fd
	state    SocketStat // current state
	id       string
	uid      uint32
	chSend   chan *PackFrame // push message queue
	chClosed chan bool

	timeOut time.Duration

	lastSendAt uint64
	lastRecvAt uint64

	transport.MapMeta
}

func (s *Socket) ID() string {
	return s.id
}

func (s *Socket) UID() uint32 {
	return atomic.LoadUint32(&s.uid)
}

func (s *Socket) SetUID(uid uint32) {
	atomic.StoreUint32(&s.uid, uid)
}

func (s *Socket) SendPacket(p *PackFrame) error {
	if len(p.body) > MaxPacketSize {
		return ErrBodySizeWrong
	}
	if !IsValidPacketType(p.GetType()) {
		return ErrWrongPacketType
	}
	return s.sendPacket(p)
}

func (s *Socket) sendPacket(p *PackFrame) error {
	if atomic.LoadInt32((*int32)(&s.state)) == int32(Disconnected) {
		return ErrDisconn
	}
	s.chSend <- p
	return nil
}

func (s *Socket) Close() error {
	if s == nil {
		return nil
	}
	stat := atomic.SwapInt32((*int32)(&s.state), int32(Disconnected))
	if stat == int32(Disconnected) {
		return nil
	}

	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}
	close(s.chSend)
	close(s.chClosed)
	return nil
}

// returns the remote network address.
func (s *Socket) RemoteAddr() string {
	if s == nil {
		return ""
	}
	if s.conn == nil {
		return ""
	}
	return s.conn.RemoteAddr().String()
}

func (s *Socket) LocalAddr() string {
	if s == nil {
		return ""
	}
	if s.conn == nil {
		return ""
	}
	return s.conn.LocalAddr().String()
}

// retrun socket work status
func (s *Socket) Status() SocketStat {
	if s == nil {
		return Disconnected
	}
	return SocketStat(atomic.LoadInt32((*int32)(&s.state)))
}

func (s *Socket) writeWork() {
	for p := range s.chSend {
		s.writePacket(p)
	}
}

func (s *Socket) newPacket() *PackFrame {
	return NewEmptyPackFrame()
}

func writeAll(conn net.Conn, raw []byte) (int, error) {
	writelen := 0
	rawSize := len(raw)

	for writelen < rawSize {
		n, err := conn.Write(raw[writelen:])
		writelen += n
		if err != nil {
			return writelen, err
		}
	}

	return writelen, nil
}

func (s *Socket) readPacket(p *PackFrame) error {
	if s.Status() == Disconnected {
		return ErrDisconn
	}

	var err error

	if s.timeOut > 0 {
		s.conn.SetReadDeadline(time.Now().Add(s.timeOut))
	}

	_, err = io.ReadFull(s.conn, p.meta)
	if err != nil {
		return err
	}

	if !IsValidPacketType(p.GetType()) {
		return ErrWrongPacketType
	}

	headlen := p.meta.GetHeadLen()
	if headlen > 0 {
		p.head = make([]byte, headlen)
		_, err = io.ReadFull(s.conn, p.head)
		if err != nil {
			return err
		}
	}

	bodylen := p.meta.GetBodyLen()
	if bodylen > 0 {
		p.body = make([]byte, bodylen)
		_, err = io.ReadFull(s.conn, p.body)
		if err != nil {
			return err
		}
	}

	atomic.StoreUint64(&s.lastRecvAt, uint64(time.Now().Unix()))
	return nil
}

func (s *Socket) writePacket(p *PackFrame) error {
	if s.Status() == Disconnected {
		return ErrDisconn
	}

	if len(p.body) >= MaxPacketSize {
		return ErrBodySizeWrong
	}

	var err error
	_, err = writeAll(s.conn, p.meta[:])
	if err != nil {
		return err
	}

	if len(p.head) > 0 {
		_, err = writeAll(s.conn, p.head)
		if err != nil {
			return err
		}
	}

	if len(p.body) > 0 {
		_, err = writeAll(s.conn, p.body)
		if err != nil {
			return err
		}
	}

	atomic.StoreUint64(&s.lastSendAt, uint64(time.Now().Unix()))
	return nil
}
