package tcp

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var staticIdx uint64

func nextID() string {
	idx := atomic.AddUint64(&staticIdx, 1)
	if idx == 0 {
		idx = atomic.AddUint64(&staticIdx, 1)
	}
	return fmt.Sprintf("tcp_%v_%v", idx, time.Now().Unix())
}

type ServerOptions struct {
	Address          string
	HeatbeatInterval time.Duration
	OnMessage        OnMessageFunc
	OnConn           OnConnStatFunc
	NewIDFunc        NewIDFunc
}

type ServerOption func(*ServerOptions)

func NewServer(opts ServerOptions) *Server {
	ret := &Server{
		opts:    opts,
		sockets: make(map[string]*Socket),
		die:     make(chan bool),
	}

	if ret.opts.NewIDFunc == nil {
		ret.opts.NewIDFunc = nextID
	}
	return ret
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
	s.wgConns.Wait()
	return nil
}

func (s *Server) Start() error {
	s.die = make(chan bool)
	listener, err := net.Listen("tcp", s.opts.Address)
	if err != nil {
		return err
	}
	s.listener = listener

	go func() {
		defer listener.Close()
		var tempDelay time.Duration = 0
		for {
			select {
			case <-s.die:
				return
			default:
				conn, err := listener.Accept()
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
					return
				}
				tempDelay = 0

				socket := NewSocket(conn, SocketOptions{
					ID: s.opts.NewIDFunc(),
				})

				go s.accept(socket)
			}
		}
	}()
	return nil
}

func (n *Server) accept(socket *Socket) {
	n.wgConns.Add(1)
	defer n.wgConns.Done()
	defer socket.Close()

	//read ack
	ack := socket.newPacket()
	if err := socket.readPacket(ack); err != nil {
		return
	}

	if ack.GetType() != PacketTypAck {
		return
	}

	// set socket id
	ack.Body = []byte(socket.ID())
	if err := socket.writePacket(ack); err != nil {
		return
	}

	// after ack, the connection is established
	go socket.writeWork()
	n.storeSocket(socket)
	defer n.removeSocket(socket)

	if n.opts.OnConn != nil {
		n.opts.OnConn(socket, Connected)
		defer n.opts.OnConn(socket, Disconnected)
	}

	var socketErr error = nil

	for {
		p := socket.newPacket()
		socketErr = socket.readPacket(p)
		if socketErr != nil {
			break
		}

		typ := p.GetType()

		if typ > PacketTypStartAt_ && typ < PacketTypEndAt_ {
			if n.opts.OnMessage != nil {
				n.opts.OnMessage(socket, p)
			}
		} else if typ > PacketTypInnerStartAt_ && typ < PacketTypInnerEndAt_ {

		} else {
			break
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
	s.sockets[conn.ID()] = conn
}

func (s *Server) removeSocket(conn *Socket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sockets, conn.ID())
}
