package tcp

import (
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
	OnMessage        OnMessageFunc
	OnConn           OnConnStatFunc
	NewIDFunc        NewIDFunc
	AuthFunc         func([]byte) (*auth.UserInfo, error)

	BeforeAccpectConnFunc func(net.Conn) bool
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

	if ret.opts.OnMessage == nil {
		ret.opts.OnMessage = func(s *Socket, p *THVPacket) {}
	}
	if ret.opts.OnConn == nil {
		ret.opts.OnConn = func(s *Socket, enable bool) {}
	}
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
	default:
		close(s.die)
	}
	s.listener.Close()
	s.wgConns.Wait()
	return nil
}

func (s *Server) Start() error {
	var tempDelay time.Duration = 0
	for {
		select {
		case <-s.die:
			return nil
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
				return err
			}
			tempDelay = 0
			go s.onAccept(conn)
		}
	}
}

func (s *Server) doHandShake(conn net.Conn) (*Socket, error) {
	rwtimeout := s.opts.HeatbeatInterval
	//read ack
	ack := newEmptyTHVPacket()
	if err := readPacket(conn, ack, rwtimeout); err != nil {
		return nil, err
	}
	if ack.GetType() != PacketTypeHandShake &&
		ack.meta.GetHeadLen() != 0 &&
		ack.meta.GetBodyLen() != 0 {
		return nil, fmt.Errorf("invalid ack packet")
	}

	var userinfo *auth.UserInfo

	// auth token
	if s.opts.AuthFunc != nil {
		ack.Reset()
		ack.SetType(PacketTypeActionRequire)
		ack.SetHead([]byte("auth"))
		if err := writePacket(conn, ack, rwtimeout); err != nil {
			return nil, err
		}
		ack.Reset()
		if err := readPacket(conn, ack, rwtimeout); err != nil || ack.GetType() != PacketTypeDoAction {
			return nil, err
		}
		var err error
		if userinfo, err = s.opts.AuthFunc(ack.GetBody()); err != nil {
			ack.SetType(PacketTypeAckResult)
			ack.SetHead([]byte("fail"))
			ack.SetBody([]uint8(err.Error()))
			writePacket(conn, ack, rwtimeout)
			return nil, err
		}
	}

	socketid := s.opts.NewIDFunc()
	socket := NewSocket(conn, SocketOptions{
		ID:      socketid,
		Timeout: s.opts.HeatbeatInterval,
	})

	ack.Reset()
	ack.SetType(PacketTypeAckResult)
	ack.SetHead([]byte("ok"))
	ack.SetBody([]byte(socketid))
	if err := writePacket(conn, ack, rwtimeout); err != nil {
		return nil, err
	}
	socket.UserInfo = *userinfo
	return socket, nil
}

func (s *Server) onAccept(conn net.Conn) {
	if s.opts.BeforeAccpectConnFunc != nil {
		if !s.opts.BeforeAccpectConnFunc(conn) {
			conn.Close()
			return
		}
	}

	socket, err := s.doHandShake(conn)
	if err != nil {
		conn.Close()
		return
	}
	defer socket.Close()

	s.wgConns.Add(1)
	defer s.wgConns.Done()

	// the connection is established
	go socket.writeWork()

	s.storeSocket(socket)
	defer s.removeSocket(socket)

	if s.opts.OnConn != nil {
		s.opts.OnConn(socket, true)
		defer s.opts.OnConn(socket, false)
	}

	var socketErr error = nil

	for {
		select {
		case <-s.die:
			return
		default:
			//TODO:
			p := newEmptyTHVPacket()
			socketErr = socket.readPacket(p)
			if socketErr != nil {
				break
			}

			typ := p.GetType()
			if typ >= PacketTypeInnerStartAt_ && typ <= PacketTypeInnerEndAt_ {
				switch typ {
				case PacketTypeHeartbeat:
					fallthrough
				case PacketTypeEcho:
					socket.SendPacket(p)
				}
			} else {
				if s.opts.OnMessage != nil {
					s.opts.OnMessage(socket, p)
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
