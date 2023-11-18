package server

import (
	"context"
	"reflect"

	"google.golang.org/protobuf/proto"

	"route/server/tcp"
)

type TcpClientOptions struct {
	RemoteAddress        string
	AuthToken            string
	ReconnectDelaySecond int32
	OnSessionMessage     FuncOnSessionMessage
	OnSessionStatus      FuncOnSessionStatus
}

func NewTcpClient(opts *TcpClientOptions) *TcpClient {

	ret := &TcpClient{
		opts: opts,
	}
	imp := tcp.NewClient(&tcp.ClientOptions{
		RemoteAddress:        opts.RemoteAddress,
		Token:                opts.AuthToken,
		OnMessage:            ret.OnTcpMessage,
		OnConnStat:           ret.OnTcpConn,
		ReconnectDelaySecond: opts.ReconnectDelaySecond,
	})
	ret.imp = imp
	return ret
}

type OnRespCBFunc func(*TcpClient, *tcp.THVPacket)

type TcpClient struct {
	imp  *tcp.Client
	opts *TcpClientOptions
}

func (s *TcpClient) OnTcpMessage(socket *tcp.Socket, p *tcp.THVPacket) {
	sess := loadTcpSession(socket)
	if sess == nil {
		return
	}
	msg := sess.pkg2msg(p)
	if s.opts.OnSessionMessage != nil {
		s.opts.OnSessionMessage(sess, msg)
	}
}

func (s *TcpClient) Start() error {
	return s.imp.Connect()
}

func (s *TcpClient) Stop() error {
	s.imp.Close()
	return nil
}

func (s *TcpClient) OnTcpConn(socket *tcp.Socket, enable bool) {
	var sess *TcpSession
	if enable {
		sess = &TcpSession{
			Socket: socket,
		}
		socket.Meta.Store(tcpSessionKey, sess)
	} else {
		sess = loadTcpSession(socket)
		socket.Meta.Delete(tcpSessionKey)
	}

	if sess != nil && s.opts.OnSessionStatus != nil {
		s.opts.OnSessionStatus(sess, enable)
	}
}

func SendRequestWithCB[T proto.Message](c *TcpClient, target uint64, ctx context.Context, req proto.Message, cb func(error, *TcpClient, T)) {
	go func() {
		var tresp T
		rsep := reflect.New(reflect.TypeOf(tresp).Elem()).Interface().(T)
		err := c.SyncCall(target, ctx, req, rsep)
		cb(err, c, rsep)
	}()
}

func (c *TcpClient) SyncCall(target uint64, ctx context.Context, req proto.Message, resp proto.Message) error {
	// var err error

	// msg, err := c.MakeRequestPacket(target, req)
	// if err != nil {
	// 	return err
	// }
	// askid := msg.Head.Seq

	// res := make(chan error, 1)
	// var askid = 0
	// c.SetCallback(uint32(askid), func(c *TcpClient, p *tcp.THVPacket) {
	// var err error
	// defer func() {
	// 	res <- err
	// }()
	// head, err := tcp.CastRoutHead(p.GetHead())
	// if err != nil {
	// 	return
	// }
	// msgtype := head.GetMsgTyp()
	// if msgtype == tcp.RouteTypRespErr {
	// 	resperr := &msg.Error{Code: -1}
	// 	err := proto.Unmarshal(p.GetBody(), resperr)
	// 	if err != nil {
	// 		return
	// 	}
	// 	err = resperr
	// 	return
	// } else if head.GetMsgTyp() == tcp.RouteTypResponse {
	// 	gotmsgid := head.GetMsgID()
	// 	expectmsgid := uint32(calltable.GetMessageMsgID(resp))
	// 	if gotmsgid == expectmsgid {
	// 		err = proto.Unmarshal(p.GetBody(), resp)
	// 	} else {
	// 		err = fmt.Errorf("msgid not match, expect:%v, got:%v", expectmsgid, gotmsgid)
	// 	}
	// } else {
	// 	err = fmt.Errorf("unknow msgtype:%v", msgtype)
	// }
	// })

	// err = c.imp.SendPacket(packet)

	// if err != nil {
	// 	c.RemoveCallback(askid)
	// 	return err
	// }

	// select {
	// case err = <-res:
	// 	return err
	// case <-ctx.Done():
	// 	// dismiss callback
	// 	c.SetCallback(askid, func(c *TcpClient, packet *tcp.THVPacket) {})
	// 	return ctx.Err()
	// }
	return nil
}
