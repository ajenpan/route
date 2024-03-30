package server

import (
	"errors"
	"net"
	"sync/atomic"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

func NewWebSocket(id string, c net.Conn) *webSocket {
	ret := &webSocket{
		id:       id,
		conn:     c,
		chSend:   make(chan *HVPacket, 10),
		chClosed: make(chan bool),
	}
	return ret
}

type webSocketOptions struct {
	ID string
}

type webSocket struct {
	conn     net.Conn      // low-level conn fd
	state    SessionStatus // current state
	id       string
	chSend   chan *HVPacket // push message queue
	chClosed chan bool

	lastSendAt uint64
	lastRecvAt uint64
}

func (s *webSocket) ID() string {
	return s.id
}

func (s *webSocket) Send(p *HVPacket) error {
	if atomic.LoadInt32((*int32)(&s.state)) == int32(Disconnected) {
		return errors.New("send packet failed, the socket is disconnected")
	}
	s.chSend <- p
	return nil
}

func (s *webSocket) Close() {
	stat := atomic.SwapInt32((*int32)(&s.state), int32(Disconnected))
	if stat == int32(Disconnected) {
		return
	}

	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}
	close(s.chSend)
	close(s.chClosed)
}

// returns the remote network address.
func (s *webSocket) RemoteAddr() net.Addr {
	if s == nil {
		return nil
	}
	return s.conn.RemoteAddr()
}

func (s *webSocket) LocalAddr() net.Addr {
	if s == nil {
		return nil
	}
	return s.conn.LocalAddr()
}

// retrun socket work status
func (s *webSocket) Status() SessionStatus {
	if s == nil {
		return Disconnected
	}
	return SessionStatus(atomic.LoadInt32((*int32)(&s.state)))
}

func (s *webSocket) writeWork() {
	for p := range s.chSend {
		s.writePacket(p)
	}
}

func (s *webSocket) readPacket(p *HVPacket) error {
	if s.Status() == Disconnected {
		return errors.New("recv packet failed, the socket is disconnected")
	}

	var err error
	data, _, err := wsutil.ReadClientData(s.conn)
	p.SetBody(data)

	if err != nil {
		return err
	}

	atomic.StoreUint64(&s.lastRecvAt, uint64(time.Now().Unix()))
	return nil
}

func (s *webSocket) writePacket(p *HVPacket) error {
	if s.Status() == Disconnected {
		return errors.New("recv packet failed, the socket is disconnected")
	}

	err := wsutil.WriteServerMessage(s.conn, ws.OpText, p.GetBody())
	if err != nil {
		return err
	}

	atomic.StoreUint64(&s.lastSendAt, uint64(time.Now().Unix()))
	return nil
}
