package server

import (
	"fmt"
	"net"
	"sync"
	"time"

	"route/auth"
)

type TcpServerOptions struct {
	ListenAddr       string
	HeatbeatInterval time.Duration

	AuthFunc        func([]byte) (*auth.UserInfo, error)
	OnSessionPacket FuncOnSessionPacket
	OnSessionStatus FuncOnSessionStatus
	// OnSessionDisconn FuncOnSessionDisconn
	OnAccpect FuncOnAccpect
}

type TcpServerOption func(*TcpServerOptions)

func NewTcpServer(opts TcpServerOptions) (*tcpServer, error) {
	ret := &tcpServer{
		opts:    opts,
		sockets: make(map[string]*tcpSocket),
		die:     make(chan bool),
	}
	if ret.opts.HeatbeatInterval < time.Duration(DefaultMinTimeoutSec)*time.Second {
		ret.opts.HeatbeatInterval = time.Duration(DefaultTimeoutSec) * time.Second
	}

	listener, err := net.Listen("tcp", opts.ListenAddr)
	if err != nil {
		return nil, err
	}
	ret.listener = listener
	return ret, nil
}

type tcpServer struct {
	opts     TcpServerOptions
	mu       sync.RWMutex
	sockets  map[string]*tcpSocket
	die      chan bool
	wgConns  sync.WaitGroup
	listener net.Listener
}

func (s *tcpServer) Stop() error {
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

func (s *tcpServer) Start() error {
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

func (s *tcpServer) onAccept(conn net.Conn) {
	defer conn.Close()

	if s.opts.OnAccpect != nil {
		if !s.opts.OnAccpect(conn) {
			return
		}
	}

	socket, err := s.handshake(conn)
	if err != nil {
		return
	}

	socket.status = Connected
	s.wgConns.Add(1)
	defer s.wgConns.Done()

	// the connection is established here
	go func() {
		defer socket.Close()
		socket.writeWork()
	}()

	go func() {
		defer socket.Close()
		socket.readWork()
	}()

	s.storeSocket(socket)
	defer s.removeSocket(socket)

	if s.opts.OnSessionStatus != nil {
		s.opts.OnSessionStatus(socket, true)
		defer s.opts.OnSessionStatus(socket, false)
	}

	for {
		select {
		case <-socket.chClosed:
			return
		case <-s.die:
			socket.Close()
			return
		case packet, ok := <-socket.chRead:
			if !ok {
				return
			}

			dealed := false

			if packet.PacketType() == HVPacketType {
				packet := packet.(*HVPacket)
				switch packet.GetFlag() {
				case HVPacketFlagEcho:
					socket.Send(packet)
					dealed = true
				case HVPacketFlagHeartbeat:
					socket.Send(packet)
					dealed = true
				}
			}

			if !dealed && s.opts.OnSessionPacket != nil {
				s.opts.OnSessionPacket(socket, packet)
			}
		}
	}
}

func (s *tcpServer) handshake(conn net.Conn) (*tcpSocket, error) {
	deadline := time.Now().Add(s.opts.HeatbeatInterval)
	conn.SetReadDeadline(deadline)
	conn.SetWriteDeadline(deadline)

	p, err := ReadPacketT[*HVPacket](conn)
	if err != nil {
		return nil, err
	}

	if p.GetFlag() != hvPacketFlagHandShake && len(p.GetBody()) != 0 {
		return nil, ErrInvalidPacket
	}

	var userinfo *auth.UserInfo

	// auth token
	if s.opts.AuthFunc != nil {
		p.SetFlag(hvPacketFlagActionRequire)
		p.SetBody([]byte("auth"))
		if _, err = WritePacket(conn, p); err != nil {
			return nil, err
		}

		if p, err = ReadPacketT[*HVPacket](conn); err != nil || p.GetFlag() != hvPacketFlagDoAction {
			return nil, err
		}

		if userinfo, err = s.opts.AuthFunc(p.GetBody()); err != nil {
			p.SetFlag(hvPacketFlagAckResult)
			p.SetBody([]byte("fail"))
			WritePacket(conn, p)
			return nil, err
		}
	}

	socketid := NewSessionID()

	socket := &tcpSocket{
		id:       socketid,
		conn:     conn,
		timeOut:  s.opts.HeatbeatInterval,
		chWrite:  make(chan Packet, 100),
		chRead:   make(chan Packet, 100),
		chClosed: make(chan struct{}),
		status:   Disconnected,
	}

	p.SetFlag(hvPacketFlagAckResult)
	p.SetBody([]byte(socketid))
	if _, err := WritePacket(conn, p); err != nil {
		return nil, err
	}

	if userinfo != nil {
		socket.UserInfo = *userinfo
	}

	return socket, nil
}

func (s *tcpServer) Address() net.Addr {
	return s.listener.Addr()
}

func (s *tcpServer) GetSocket(id string) *tcpSocket {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ret, ok := s.sockets[id]
	if ok {
		return ret
	}
	return nil
}

func (s *tcpServer) SocketCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sockets)
}

func (s *tcpServer) storeSocket(conn *tcpSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sockets[conn.SessionID()] = conn
}

func (s *tcpServer) removeSocket(conn *tcpSocket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sockets, conn.SessionID())
}
