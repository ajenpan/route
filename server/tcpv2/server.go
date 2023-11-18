package tcp

import (
	"bytes"
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
	Address           string
	HeatbeatInterval  time.Duration
	OnSocketMessage   func(*Socket, []byte)
	OnSocketConn      func(*Socket)
	OnSocketDisconn   func(*Socket, error)
	OnAccpectConnFunc func(net.Conn) bool
	NewIDFunc         func() string
	AuthFunc          func([]byte) (*auth.UserInfo, error)
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

func (s *Server) doHandShake(conn net.Conn) (*Socket, error) {

	return nil, nil
}

func (s *Server) onAccept(conn net.Conn) {
	if s.opts.OnAccpectConnFunc != nil {
		if !s.opts.OnAccpectConnFunc(conn) {
			conn.Close()
			return
		}
	}
	socket, err := s.doHandShake(conn)
	if err != nil {
		conn.Close()
		return
	}

	socket.status = Connected
	defer socket.Close()

	s.wgConns.Add(1)
	defer s.wgConns.Done()

	// the connection is established here
	var socketErr error = nil

	go func() {
		err := socket.writeWork()
		if err != nil {
			fmt.Println(err)
		}
	}()

	s.storeSocket(socket)
	defer s.removeSocket(socket)

	head := newHead()
	bodyhold := make([]byte, MaxPacketBodySize)

	if s.opts.OnSocketConn != nil {
		s.opts.OnSocketConn(socket)
	}

	if s.opts.OnSocketDisconn != nil {
		defer func() {
			s.opts.OnSocketDisconn(socket, socketErr)
		}()
	}

	for {
		select {
		case <-s.die:
			return
		default:
			if _, socketErr := socket.readWork(head); socketErr != nil {
				return
			}
			if !head.Valid() {
				socketErr = ErrInvalidPacket
				return
			}
			body := bodyhold[:head.GetBodyLen()]
			if _, socketErr := socket.readWork(body); socketErr != nil {
				return
			}

			//TODO:
			// p := newEmptyTHVPacket()
			// socketErr = socket.readPacket(p)
			// if socketErr != nil {
			// 	break
			// }

			typ := head.GetType()
			if typ >= PacketTypeInnerStartAt_ && typ <= PacketTypeInnerEndAt_ {
				switch typ {
				case PacketTypeHeartbeat:
					fallthrough
				case PacketTypeEcho:
					socket.Send(bytes.Clone(body))
				}
			} else {
				if s.opts.OnSocketMessage != nil {
					s.opts.OnSocketMessage(socket, body)
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
