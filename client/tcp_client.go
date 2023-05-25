package client

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"route/transport/tcp"
	"route/utils/calltable"
)

type LoginStat int

const (
	LoginStat_Success    LoginStat = iota
	LoginStat_Fail       LoginStat = iota
	LoginStat_Disconnect LoginStat = iota
)

type TcpClient struct {
	*tcp.Client

	cb sync.Map

	isAuth   bool
	AuthFunc func() bool

	OnMessageFunc func(c *TcpClient, p *tcp.PackFrame)
	OnLoginFunc   func(c *TcpClient, stat LoginStat)
	AutoRecconect bool

	reconnectTimeDelay time.Duration
}

func (c *TcpClient) Reconnect() {
	err := c.Connect()
	if err != nil && c.AutoRecconect {
		fmt.Println("start to reconnect")
		time.AfterFunc(c.reconnectTimeDelay, func() {
			c.Reconnect()
		})
	}
}

func (c *TcpClient) SetCallback(askid uint32, f func(*tcp.Socket, *tcp.PackFrame)) {
	c.cb.Store(askid, f)
}

func (c *TcpClient) RemoveCallback(askid uint32) {
	c.cb.Delete(askid)
}

func (c *TcpClient) MakeRequestPacket(target uint32, req proto.Message) (*tcp.PackFrame, uint32, error) {
	msgid := calltable.GetMessageMsgID(req.ProtoReflect().Descriptor())
	if msgid == 0 {
		return nil, 0, fmt.Errorf("not found msgid:%v", msgid)
	}

	raw, err := proto.Marshal(req)
	if err != nil {
		return nil, 0, err
	}
	askid := c.GetAskID()
	head := tcp.NewRoutHead()
	head.SetAskID(askid)
	head.SetMsgID(uint32(msgid))
	head.SetTargetUID(target)
	head.SetMsgTyp(tcp.RouteTypRequest)

	ret := &tcp.PackFrame{
		Head: head,
		Body: raw,
	}
	ret.SetType(tcp.PacketTypRoute)
	ret.SetBodyLen(uint32(len(raw)))
	ret.SetHeadLen(uint8(len(head)))
	return ret, askid, nil
}

func SendRequestWithCB[T proto.Message](c *TcpClient, target uint32, ctx context.Context, req proto.Message, cb func(error, *TcpClient, T)) {
	go func() {
		var tresp T
		rsep := reflect.New(reflect.TypeOf(tresp).Elem()).Interface().(T)
		err := c.SyncCall(target, ctx, req, rsep)
		cb(err, c, rsep)
	}()
}

func (c *TcpClient) SyncCall(target uint32, ctx context.Context, req proto.Message, resp proto.Message) error {
	var err error

	packet, askid, err := c.MakeRequestPacket(target, req)
	if err != nil {
		return err
	}

	res := make(chan error, 1)

	c.SetCallback(askid, func(c *tcp.Socket, packet *tcp.PackFrame) {
		var err error
		defer func() {
			res <- err
		}()

		if err = proto.Unmarshal(packet.Body, resp); err != nil {
			return
		}
	})

	err = c.SendPacket(packet)

	if err != nil {
		c.RemoveCallback(askid)
		return err
	}

	select {
	case err = <-res:
		return err
	case <-ctx.Done():
		// dismiss callback
		c.SetCallback(askid, func(c *tcp.Socket, packet *tcp.PackFrame) {})
		return ctx.Err()
	}
}

func (r *TcpClient) AsyncCall(target uint32, m proto.Message) error {
	raw, err := proto.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal failed:%v", err)
	}

	msgid := calltable.GetMessageMsgID(m.ProtoReflect().Descriptor())
	if msgid == 0 {
		return fmt.Errorf("not found msgid:%v", msgid)
	}

	head := tcp.NewRoutHead()
	head.SetMsgID(uint32(msgid))
	head.SetSrouceUID(r.UID())
	head.SetTargetUID(target)
	head.SetMsgTyp(tcp.RouteTypAsync)
	p := &tcp.PackFrame{
		Head: head,
		Body: raw,
	}
	p.SetType(tcp.PacketTypRoute)
	p.SetBodyLen(uint32(len(raw)))
	p.SetHeadLen(uint8(len(head)))
	return r.SendPacket(p)
}

func NewTcpClient(remoteAddr string) *TcpClient {
	ret := &TcpClient{
		AutoRecconect: true,
	}

	c := tcp.NewClient(&tcp.ClientOptions{
		RemoteAddress: remoteAddr,
		OnMessage: func(s *tcp.Socket, p *tcp.PackFrame) {
			ret.OnMessageFunc(ret, p)
		},
		OnConnStat: func(s *tcp.Socket, ss tcp.SocketStat) {
			ret.isAuth = false
			if ss == tcp.Disconnected {
				if ret.OnLoginFunc != nil {
					ret.OnLoginFunc(ret, LoginStat_Disconnect)
				}
				if ret.AutoRecconect {
					ret.Reconnect()
				}
			} else {
				go func() {
					stat := LoginStat_Fail
					if ret.AuthFunc != nil {
						if ret.AuthFunc() {
							stat = LoginStat_Success
						}
					}
					if ret.OnLoginFunc != nil {
						ret.OnLoginFunc(ret, stat)
					}
				}()
			}

		},
	})
	ret.Client = c
	return ret
}
