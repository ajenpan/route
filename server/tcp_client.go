package server

import (
	"context"
	"errors"
	"net"
	"reflect"
	"sync"
	"sync/atomic"

	"google.golang.org/protobuf/proto"

	"route/server/tcp"
)

type TcpClientOptions struct {
	RemoteAddress        string
	AuthToken            string
	ReconnectDelaySecond int32
	OnMessage            func(*TcpClient, *Message)
	OnStatus             func(*TcpClient, bool)
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

var ErrTimeout = errors.New("timeout")

type FuncRespCallback = func(error, *TcpClient, *Message)

type TcpClient struct {
	*TcpSession
	imp    *tcp.Client
	opts   *TcpClientOptions
	seqidx uint32
	cb     sync.Map
}

func (c *TcpClient) SetCallback(askid uint32, f FuncRespCallback) {
	c.cb.Store(askid, f)
}

func (c *TcpClient) RemoveCallback(askid uint32) {
	c.cb.Delete(askid)
}

func (c *TcpClient) GetCallback(askid uint32) FuncRespCallback {
	if v, has := c.cb.LoadAndDelete(askid); has {
		return v.(FuncRespCallback)
	}
	return nil
}

func (c *TcpClient) OnTcpMessage(socket *tcp.Client, p *tcp.THVPacket) {
	msg := pkg2msg(p)
	if msg.Head.Msgtype == 2 {
		if cb := c.GetCallback(msg.Head.Seqid); cb != nil {
			cb(nil, c, msg)
			return
		}
		return
	}

	if c.opts.OnMessage != nil {
		c.opts.OnMessage(c, msg)
	}
}

func (s *TcpClient) Start() error {
	return s.imp.Connect()
}

func (s *TcpClient) RemoteAddr() net.Addr {
	return s.imp.RemoteAddr()
}

func (s *TcpClient) OnTcpConn(c *tcp.Client, enable bool) {
	if enable {
		s.TcpSession = &TcpSession{
			imp: c.Socket,
		}
	}
	if s.opts.OnStatus != nil {
		s.opts.OnStatus(s, enable)
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

func (c *TcpClient) SendAsyncMsg(target uint64, req proto.Message) error {
	head := &Head{
		Msgname: string(req.ProtoReflect().Descriptor().FullName().Name()),
		Uid:     target,
		Msgtype: 0,
	}
	body, err := proto.Marshal(req)
	if err != nil {
		return err
	}
	msg := &Message{
		ContentType: ProtoBinaryRoute,
		Head:        head,
		Body:        body,
	}
	return c.imp.SendPacket(msg2pkg(msg))
}

func (c *TcpClient) NextSeqID() uint32 {
	id := atomic.AddUint32(&c.seqidx, 1)
	if id == 0 {
		return c.NextSeqID()
	}
	return id
}

func NewTcpRespCallbackFunc[T proto.Message](f func(error, *TcpClient, T)) FuncRespCallback {
	return func(err error, c *TcpClient, resp *Message) {
		var tresp T
		if err != nil {
			f(err, c, tresp)
			return
		}
		rsep := reflect.New(reflect.TypeOf(tresp).Elem()).Interface().(T)
		err1 := proto.Unmarshal(resp.Body, rsep)
		if err1 != nil {
			f(err1, c, rsep)
			return
		}
		f(err, c, rsep)
	}
}

func (c *TcpClient) SendReqMsg(target uint64, req proto.Message, cb FuncRespCallback) error {
	seqid := c.NextSeqID()

	head := &Head{
		Msgname: string(req.ProtoReflect().Descriptor().FullName().Name()),
		Uid:     target,
		Msgtype: 1,
		Seqid:   seqid,
	}
	body, err := proto.Marshal(req)
	if err != nil {
		return err
	}
	msg := &Message{
		ContentType: ProtoBinaryRoute,
		Head:        head,
		Body:        body,
	}

	c.SetCallback(seqid, cb)

	err = c.imp.SendPacket(msg2pkg(msg))
	if err != nil {
		c.RemoveCallback(seqid)
	}
	return err
}

func (c *TcpClient) SendRespMsg(target uint64, seqid uint32, resp proto.Message, resperr *Error) error {
	head := &Head{
		Msgname: string(resp.ProtoReflect().Descriptor().FullName().Name()),
		Uid:     target,
		Seqid:   seqid,
		Err:     resperr,
		Msgtype: 2,
	}
	body, err := proto.Marshal(resp)
	if err != nil {
		return err
	}
	m := &Message{
		ContentType: ProtoBinaryRoute,
		Head:        head,
		Body:        body,
	}
	return c.imp.SendPacket(msg2pkg(m))
}

func (c *TcpClient) SyncCall(target uint64, ctx context.Context, req proto.Message, resp proto.Message) error {
	// var err error

	// seqid := c.NextSeqID()

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
