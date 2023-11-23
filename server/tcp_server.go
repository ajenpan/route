package server

import (
	"crypto/rsa"
	"fmt"
	"net"

	"route/auth"
	"route/msg"
	"route/server/tcp"

	"route/server/marshal"
)

type TcpServerOptions struct {
	ListenAddr       string
	AuthPublicKey    *rsa.PublicKey
	OnSessionMessage FuncOnSessionMessage
	OnSessionStatus  FuncOnSessionStatus
	Marshal          marshal.Marshaler
}

func NewTcpServer(opts *TcpServerOptions) (*TcpServer, error) {
	ret := &TcpServer{
		opts:       opts,
		listenAddr: opts.ListenAddr,
	}
	if opts.Marshal == nil {
		opts.Marshal = &marshal.ProtoMarshaler{}
	}
	tcpopt := tcp.ServerOptions{
		Address:   opts.ListenAddr,
		OnMessage: ret.OnTcpMessage,
		OnConn:    ret.OnTcpConn,
		OnAccpectConn: func(conn net.Conn) bool {
			fmt.Printf("OnAccpectConn remote:%s, local:%s\n", conn.RemoteAddr(), conn.LocalAddr())
			return true
		},
		NewIDFunc: NewSessionID,
	}

	if opts.AuthPublicKey != nil {
		tcpopt.AuthFunc = auth.RsaTokenAuth(opts.AuthPublicKey)
	}

	imp, err := tcp.NewServer(tcpopt)
	if err != nil {
		return nil, err
	}
	ret.imp = imp
	return ret, nil
}

type TcpServer struct {
	imp *tcp.Server

	opts       *TcpServerOptions
	listenAddr string
}

type TcpSession struct {
	*tcp.Socket
	marshaler marshal.Marshaler
}

var tcpSessionKey = &struct{}{}

func (s *TcpSession) Send(msg *Message) error {
	if msg == nil {
		return fmt.Errorf("message is nil")
	}
	p := s.msg2pkg(msg)
	if p == nil {
		return fmt.Errorf("message is nil")
	}
	return s.Socket.SendPacket(p)
}

func (s *TcpSession) SessionType() string {
	return "tcp-session"
}

func (s *TcpSession) msg2pkg(p *Message) *tcp.THVPacket {
	headraw, _ := s.marshaler.Marshal(p.Head)
	return tcp.NewTHVPacket(uint8(p.ContentType), headraw, p.Body)
}

func (s *TcpSession) pkg2msg(p *tcp.THVPacket) *Message {
	head := &msg.Head{}
	s.marshaler.Unmarshal(p.GetHead(), head)
	ret := &Message{
		ContentType: ContentType(p.GetType()),
		Head:        head,
		Body:        p.Body,
	}
	return ret
}

func (s *TcpServer) Start() error {
	return s.imp.Start()
}

func (s *TcpServer) Stop() error {
	return s.imp.Stop()
}

func loadTcpSession(socket *tcp.Socket) *TcpSession {
	v, ok := socket.Meta.Load(tcpSessionKey)
	if !ok {
		return nil
	}
	return v.(*TcpSession)
}

func (s *TcpServer) OnTcpMessage(socket *tcp.Socket, p *tcp.THVPacket) {
	sess := loadTcpSession(socket)
	if sess == nil {
		return
	}
	if s.opts.OnSessionMessage != nil {
		msg := sess.pkg2msg(p)
		if msg == nil {
			return
		}
		s.opts.OnSessionMessage(sess, msg)
	}
}

func (s *TcpServer) OnTcpConn(socket *tcp.Socket, valid bool) {
	fmt.Printf("OnTcpConn remote:%s, local:%s, valid:%v\n", socket.RemoteAddr(), socket.LocalAddr(), valid)

	var sess *TcpSession
	if valid {
		sess = &TcpSession{
			Socket:    socket,
			marshaler: s.opts.Marshal,
		}
		socket.Meta.Store(tcpSessionKey, sess)
	} else {
		sess = loadTcpSession(socket)
		socket.Meta.Delete(tcpSessionKey)
	}

	if sess != nil && s.opts.OnSessionStatus != nil {
		s.opts.OnSessionStatus(sess, valid)
	}
}
