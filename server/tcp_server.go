package server

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"route/auth"
)

type TcpServerOptions struct {
	ListenAddr       string
	HeatbeatInterval time.Duration

	AuthFunc         func([]byte) (*auth.UserInfo, error)
	OnSessionPacket  FuncOnSessionPacket
	OnSessionConn    FuncOnSessionConn
	OnSessionDisconn FuncOnSessionDisconn
	OnAccpect        FuncOnAccpect
}

type TcpServerOption func(*TcpServerOptions)

func NewTcpServer(opts TcpServerOptions) (*tcpServer, error) {
	ret := &tcpServer{
		opts:    opts,
		sockets: make(map[string]*tcpSocket),
		die:     make(chan bool),
	}
	listener, err := net.Listen("tcp", opts.ListenAddr)
	if err != nil {
		return nil, err
	}

	ret.listener = listener

	if ret.opts.HeatbeatInterval == 0 {
		ret.opts.HeatbeatInterval = time.Duration(DefaultTimeoutSec) * time.Second
	}

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
	if s.opts.OnAccpect != nil {
		if !s.opts.OnAccpect(conn) {
			conn.Close()
			return
		}
	}

	socket, err := s.handshake(conn)
	if err != nil {
		conn.Close()
		return
	}

	// socket := NewTcpSocket(conn, tcpSocketOptions{
	// 	ID:      s.opts.NewIDFunc(),
	// 	Timeout: s.opts.HeatbeatInterval / 2,
	// })
	defer socket.Close()

	socket.status = Connected
	s.wgConns.Add(1)
	defer s.wgConns.Done()

	// the connection is established here
	var writeErr error
	var readErr error

	s.storeSocket(socket)
	defer s.removeSocket(socket)

	if s.opts.OnSessionConn != nil {
		s.opts.OnSessionConn(socket)
	}
	if s.opts.OnSessionDisconn != nil {
		defer func() {
			s.opts.OnSessionDisconn(socket, errors.Join(writeErr, readErr))
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
			if s.opts.OnSessionPacket != nil {
				s.opts.OnSessionPacket(socket, packet)
			}
		}
	}
}

func (s *tcpServer) handshake(conn net.Conn) (*tcpSocket, error) {
	var err error

	rwtimeout := time.Second * 10

	p, err := ReadPacketT[*HVPacket](conn, rwtimeout)
	if err != nil {
		return nil, err
	}

	if p.GetType() != hvPacketFlagHandShake && len(p.GetBody()) != 0 {
		return nil, ErrInvalidPacket
	}

	var userinfo *auth.UserInfo

	// auth token
	if s.opts.AuthFunc != nil {
		p.SetType(hvPacketFlagActionRequire)
		p.SetBody([]byte("auth"))
		if err = WritePacket(conn, rwtimeout, p); err != nil {
			return nil, err
		}

		if p, err = ReadPacketT[*HVPacket](conn, rwtimeout); err != nil || p.GetType() != hvPacketFlagDoAction {
			return nil, err
		}

		if userinfo, err = s.opts.AuthFunc(p.GetBody()); err != nil {
			p.SetType(hvPacketFlagAckResult)
			p.SetBody([]byte("fail"))
			WritePacket(conn, rwtimeout, p)
			return nil, err
		}
	}

	socketid := NewSessionID()
	socket := NewTcpSocket(conn, tcpSocketOptions{
		ID:      socketid,
		Timeout: s.opts.HeatbeatInterval,
	})
	p.SetType(hvPacketFlagAckResult)
	p.SetBody([]byte(socketid))
	if err := WritePacket(conn, rwtimeout, p); err != nil {
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
