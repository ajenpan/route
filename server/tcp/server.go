package tcp

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"route/auth"
)

var socketIdx uint64

func nextID() string {
	idx := atomic.AddUint64(&socketIdx, 1)
	if idx == 0 {
		idx = atomic.AddUint64(&socketIdx, 1)
	}
	return fmt.Sprintf("tcp_%v", idx)
}

type ServerOptions struct {
	Address          string
	HeatbeatInterval time.Duration
	NewIDFunc        func() string
	AuthFunc         func([]byte) (*auth.UserInfo, error)

	OnSocketMessage func(*Socket, Packet)
	OnSocketConn    func(*Socket)
	OnSocketDisconn func(*Socket, error)
	OnAccpect       func(net.Conn) bool
}

type ServerOption func(*ServerOptions)

func NewServer(opts ServerOptions) (*Server, error) {
	ret := &Server{
		opts:    opts,
		sockets: make(map[string]*Socket),
		die:     make(chan bool),
	}
	listener, err := net.Listen("tcp", opts.Address)
	if err != nil {
		return nil, err
	}

	ret.listener = listener

	if ret.opts.HeatbeatInterval == 0 {
		ret.opts.HeatbeatInterval = time.Duration(DefaultTimeoutSec) * time.Second
	}
	if ret.opts.NewIDFunc == nil {
		ret.opts.NewIDFunc = nextID
	}
	return ret, nil
}

type Server struct {
	opts     ServerOptions
	mu       sync.RWMutex
	sockets  map[string]*Socket
	die      chan bool
	wgConns  sync.WaitGroup
	listener net.Listener
}

func (s *Server) Stop() error {
	select {
	case <-s.die:
		return nil
	default:
		close(s.die)
	}
	s.listener.Close()
	s.wgConns.Wait()
	return nil
}

func (s *Server) Start() error {
	go func() {
		var tempDelay time.Duration = 0
		for {
			select {
			case <-s.die:
				return
			default:
				conn, err := s.listener.Accept()
				if err != nil {
					if ne, ok := err.(net.Error); ok && ne.Timeout() {
						if tempDelay == 0 {
							tempDelay = 5 * time.Millisecond
						} else {
							tempDelay *= 2
						}
						if max := 1 * time.Second; tempDelay > max {
							tempDelay = max
						}
						time.Sleep(tempDelay)
						continue
					}
					fmt.Println(err)
					return
				}
				tempDelay = 0
				go s.onAccept(conn)
			}
		}
	}()
	return nil
}

func (s *Server) handshake(conn net.Conn) error {
	var err error

	rwtimeout := s.opts.HeatbeatInterval

	p, err := readPacketT[*hvPacket](conn, rwtimeout)
	if err != nil {
		return err
	}

	if p.GetType() != PacketTypeHandShake && len(p.GetBody()) != 0 {
		return ErrInvalidPacket
	}

	var userinfo *auth.UserInfo

	// auth token
	if s.opts.AuthFunc != nil {
		p.SetType(PacketTypeActionRequire)
		p.SetBody([]byte("auth"))
		if err = writePacket(conn, rwtimeout, p); err != nil {
			return err
		}

		if p, err = readPacketT[*hvPacket](conn, rwtimeout); err != nil || p.GetType() != PacketTypeDoAction {
			return err
		}

		if userinfo, err = s.opts.AuthFunc(p.GetBody()); err != nil {
			p.SetType(PacketTypeAckResult)
			p.SetBody([]byte("fail"))
			writePacket(conn, rwtimeout, p)
			return err
		}
	}

	socketid := s.opts.NewIDFunc()
	socket := NewSocket(conn, SocketOptions{
		ID:      socketid,
		Timeout: s.opts.HeatbeatInterval,
	})

	p.SetType(PacketTypeAckResult)
	p.SetBody([]byte(socketid))

	if err := writePacket(conn, rwtimeout, p); err != nil {
		return err
	}
	socket.UserInfo = *userinfo
	return nil
}

func (s *Server) onAccept(conn net.Conn) {
	if s.opts.OnAccpect != nil {
		if !s.opts.OnAccpect(conn) {
			conn.Close()
			return
		}
	}

	if err := s.handshake(conn); err != nil {
		conn.Close()
		return
	}

	socket := NewSocket(conn, SocketOptions{
		ID:      s.opts.NewIDFunc(),
		Timeout: s.opts.HeatbeatInterval / 2,
	})
	defer socket.Close()

	socket.status = Connected
	s.wgConns.Add(1)
	defer s.wgConns.Done()

	// the connection is established here
	var writeErr error
	var readErr error

	s.storeSocket(socket)
	defer s.removeSocket(socket)

	if s.opts.OnSocketConn != nil {
		s.opts.OnSocketConn(socket)
	}
	if s.opts.OnSocketDisconn != nil {
		defer func() {
			s.opts.OnSocketDisconn(socket, errors.Join(writeErr, readErr))
		}()
	}

	wg := &sync.WaitGroup{}
	wg.Add(2)
	defer wg.Wait()

	go func() {
		defer wg.Done()
		writeErr = socket.writeWork()
	}()
	recvchan := make(chan Packet, 100)
	go func() {
		defer wg.Done()
		readErr = socket.readWork(recvchan)
	}()

	for {
		select {
		case <-socket.chClosed:
			return
		case <-s.die:
			return
		case packet, ok := <-recvchan:
			if !ok {
				return
			}
			switch packet.PacketType() {
			case PacketTypeInnerStartAt_:
				{
					// 	fallthrough
					// case PacketTypeEcho:
					// 	socket.Send(bytes.Clone(packet.body))
				}
			default:
				if s.opts.OnSocketMessage != nil {
					s.opts.OnSocketMessage(socket, packet)
				}
			}
		}
	}
}

func (s *Server) Address() net.Addr {
	return s.listener.Addr()
}

func (s *Server) GetSocket(id string) *Socket {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ret, ok := s.sockets[id]
	if ok {
		return ret
	}
	return nil
}

func (s *Server) SocketCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sockets)
}

func (s *Server) storeSocket(conn *Socket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sockets[conn.SessionID()] = conn
}

func (s *Server) removeSocket(conn *Socket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sockets, conn.SessionID())
}
