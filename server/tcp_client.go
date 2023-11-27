package server

import (
	"context"
	"errors"
	"fmt"
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
		OnSocketMessage:      ret.OnMessage,
		OnSocketConn:         ret.OnConn,
		OnSocketDisconn:      ret.OnDisconn,
		ReconnectDelaySecond: opts.ReconnectDelaySecond,
	})
	ret.Client = imp
	return ret
}

var ErrTimeout = errors.New("timeout")

type FuncRespCallback = func(error, *TcpClient, *Message)

type TcpClient struct {
	*tcp.Client
	opts   *TcpClientOptions
	seqidx uint32
	cb     sync.Map
}

func (s *TcpClient) Send(p *Message) error {
	return s.Client.Send(p)
}

func (s *TcpClient) SessionType() string {
	return "tcp-session"
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

func (c *TcpClient) OnMessage(socket *tcp.Client, p tcp.Packet) {
	if p.PacketType() != ProtoBinaryRouteType {
		fmt.Println("unknow packet type:", p.PacketType())
		return
	}

	msg, ok := p.(*Message)
	if !ok {
		fmt.Println("unknow packet type:", p.PacketType())
	}

	if c.opts.OnMessage != nil {
		c.opts.OnMessage(c, msg)
	}
}

func (c *TcpClient) OnConn(socket *tcp.Client) {
	if c.opts.OnStatus != nil {
		c.opts.OnStatus(c, true)
	}
}

func (c *TcpClient) OnDisconn(socket *tcp.Client, err error) {
	if c.opts.OnStatus != nil {
		c.opts.OnStatus(c, false)
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
	return nil
	// head := &Head{
	// 	Msgname: string(req.ProtoReflect().Descriptor().FullName().Name()),
	// 	Uid:     target,
	// 	Msgtype: 0,
	// }
	// body, err := proto.Marshal(req)
	// if err != nil {
	// 	return err
	// }
	// msg := &Message{
	// 	ContentType: ProtoBinaryRoute,
	// 	Head:        head,
	// 	Body:        body,
	// }
	// return c.imp.SendPacket(msg2pkg(msg))
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

func (c *TcpClient) SendReqMsg(target uint32, req proto.Message, cb FuncRespCallback) error {
	var err error
	msg := &Message{}

	msg.SetMsgtype(1)
	msg.SetUid(target)

	// seqid := c.NextSeqID()
	// head := &Head{
	// 	Msgname: string(req.ProtoReflect().Descriptor().FullName().Name()),
	// 	Uid:     target,
	// 	Msgtype: 1,
	// 	Seqid:   seqid,
	// }
	// body, err := proto.Marshal(req)
	// if err != nil {
	// 	return err
	// }
	// msg := &Message{
	// 	ContentType: ProtoBinaryRoute,
	// 	Head:        head,
	// 	Body:        body,
	// }

	// c.SetCallback(seqid, cb)

	// err = c.imp.SendPacket(msg2pkg(msg))
	// if err != nil {
	// 	c.RemoveCallback(seqid)
	// }

	return err
}

func (c *TcpClient) SendRespMsg(target uint64, seqid uint32, resp proto.Message) error {

	var err error
	return err

	// head := &Head{
	// 	Msgname: string(resp.ProtoReflect().Descriptor().FullName().Name()),
	// 	Uid:     target,
	// 	Seqid:   seqid,
	// 	Err:     resperr,
	// 	Msgtype: 2,
	// }
	// body, err := proto.Marshal(resp)
	// if err != nil {
	// 	return err
	// }
	// m := &Message{
	// 	ContentType: ProtoBinaryRoute,
	// 	Head:        head,
	// 	Body:        body,
	// }
	// return c.imp.SendPacket(msg2pkg(m))
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
