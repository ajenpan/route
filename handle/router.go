package handle

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"route/auth"
	"route/permit"
	msg "route/proto"
	group "route/sessiongroup"
	"route/transport/tcp"
	"route/utils/calltable"
)

func NewRouter() (*Router, error) {
	ret := &Router{
		userSession: make(map[uint32]*tcp.Socket),
	}
	ct := calltable.ExtractAsyncMethodByMsgID(msg.File_proto_route_proto.Messages(), ret)
	if ct == nil {
		return nil, fmt.Errorf("ExtractProtoFile failed")
	}
	ret.ct = ct
	return ret, nil
}

type UserInfo struct {
	UID      uint32
	UserName string
	Role     string
	Groups   sync.Map
	LoginAt  time.Time
}

type Router struct {
	userSession     map[uint32]*tcp.Socket
	userSessionLock sync.RWMutex

	ct *calltable.CallTable[int]
	gm group.Manager

	Authc   auth.Auth
	Permitc permit.Permit

	Selfinfo *UserInfo
}

const uinfoKey string = "uinfo"
const errcntKey string = "errcnt"

type tcpSocketKeyT struct{}
type tcpPacketKeyT struct{}

var tcpSocketKey = tcpSocketKeyT{}
var tcpPacketKey = tcpPacketKeyT{}

func (r *Router) OnMessage(s *tcp.Socket, p *tcp.PackFrame) {
	ptype := p.GetType()

	if ptype == tcp.PacketTypRoute {
		head, err := tcp.CastRoutHead(p.GetHead())
		if err != nil {
			return
		}
		targetid := head.GetTargetUID()
		if targetid == 0 {
			r.OnCall(s, p, head, p.GetBody())
			return
		}

		defer func() {
			if err != nil {
				fmt.Println(err)
			}
		}()

		suid := s.UID()
		if suid == 0 {
			err = fmt.Errorf("not login")
			return
		}

		tsocket := r.GetUserSession(targetid)
		if tsocket == nil {
			err = fmt.Errorf("targetid:%d not found", targetid)
			return
		}

		if !r.forwardEnable(s, tsocket, head.GetMsgID()) {
			err = fmt.Errorf("forwardEnable failed")
			return
		}

		head.SetSrouceUID(suid)
		err = tsocket.SendPacket(p)
	}
}

func (r *Router) OnConn(s *tcp.Socket, stat tcp.SocketStat) {
	fmt.Println("OnConn", s.ID(), ",stat:", int(stat))

	if stat == tcp.Disconnected {
		uinfo := GetSocketUserInfo(s)
		if uinfo != nil {
			r.onUserOffline(s, uinfo)
		}
	}
}

func (r *Router) GetUserSession(uid uint32) *tcp.Socket {
	r.userSessionLock.RLock()
	defer r.userSessionLock.RUnlock()
	return r.userSession[uid]
}

func (r *Router) StoreUserSession(uid uint32, s *tcp.Socket) {
	r.userSessionLock.Lock()
	defer r.userSessionLock.Unlock()
	r.userSession[uid] = s
}

func (r *Router) RemoveUserSession(uid uint32, s *tcp.Socket) {
	r.userSessionLock.Lock()
	defer r.userSessionLock.Unlock()
	if r.userSession[uid] == s {
		delete(r.userSession, uid)
	}
}

func (r *Router) onUserOnline(s *tcp.Socket, uinfo *UserInfo) {
	r.StoreUserSession(uinfo.UID, s)

	uinfo.Groups.Range(func(k, v interface{}) bool {
		r.gm.AddTo(k.(string), uinfo.UID, s)
		return true
	})

	r.PublishEvent(&msg.UserStatChange{
		Sid:  s.ID(),
		Uid:  uinfo.UID,
		Stat: msg.UserStatChange_Online,
	})
}

func (r *Router) onUserOffline(s *tcp.Socket, uinfo *UserInfo) {
	r.RemoveUserSession(uinfo.UID, s)

	uinfo.Groups.Range(func(k, v interface{}) bool {
		r.gm.RemoveFromGroup(k.(string), uinfo.UID, s)
		return true
	})

	r.PublishEvent(&msg.UserStatChange{
		Sid:  s.ID(),
		Uid:  uinfo.UID,
		Stat: msg.UserStatChange_Offline,
	})
}

func (r *Router) PublishEvent(event proto.Message) {
	fmt.Printf("[Mock PublishEvent] name:%v, msg:%v\n", string(proto.MessageName(event).Name()), event)
}

func (r *Router) SendMessage(s *tcp.Socket, askid uint32, msgtyp tcp.RouteMsgTyp, m proto.Message) error {
	raw, err := proto.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal failed:%v", err)
	}

	msgid := calltable.GetMessageMsgID(m)
	if msgid == 0 {
		return fmt.Errorf("not found msgid:%v", msgid)
	}

	head := tcp.NewRoutHead()
	head.SetMsgID(uint32(msgid))
	head.SetSrouceUID(0)
	head.SetTargetUID(s.UID())
	head.SetAskID(askid)
	head.SetMsgTyp(msgtyp)

	return s.SendPacket(tcp.NewPackFrame(tcp.PacketTypRoute, head, raw))
}

func (r *Router) OnCall(s *tcp.Socket, p *tcp.PackFrame, head tcp.RouteHead, body []byte) {
	var err error

	msgid := int(head.GetMsgID())

	enable := r.callEnable(s, uint32(msgid))
	if !enable {
		fmt.Println("not enable to call this method:", msgid)
		dealSocketErrCnt(s)
		return
	}

	askid := head.GetAskID()
	method := r.ct.Get(msgid)
	if method == nil {
		fmt.Println("not found method,msgid:", msgid)
		dealSocketErrCnt(s)
		return
	}

	reqRaw := method.NewRequest()
	if reqRaw == nil {
		fmt.Println("not found request,msgid:", msgid)
		return
	}

	req := reqRaw.(proto.Message)
	err = proto.Unmarshal(body, req)

	if err != nil {
		fmt.Println(err)
		return
	}

	ctx := context.WithValue(context.Background(), tcpSocketKey, s)
	ctx = context.WithValue(ctx, tcpPacketKey, p)

	result := method.Call(r, ctx, req)

	if len(result) != 2 {
		return
	}
	// if err is not nil, only return err
	resperrI := result[1].Interface()
	if resperrI != nil {
		var senderr error
		switch resperr := resperrI.(type) {
		case *msg.Error:
			senderr = r.SendMessage(s, askid, tcp.RouteTypRespErr, resperr)
		case error:
			senderr = r.SendMessage(s, askid, tcp.RouteTypRespErr, &msg.Error{
				Code:   -1,
				Detail: resperr.Error(),
			})
		default:
			fmt.Println("not support error type:")
		}
		if senderr != nil {
			fmt.Println("send err failed:", senderr)
		}
		return
	}

	respI := result[0].Interface()
	if respI != nil {
		resp, ok := respI.(proto.Message)
		if !ok {
			return
		}
		respMsgTyp := head.GetMsgTyp()
		if respMsgTyp == tcp.RouteTypRequest {
			respMsgTyp = tcp.RouteTypResponse
		}

		r.SendMessage(s, askid, respMsgTyp, resp)
		fmt.Printf("oncall sid:%v,uid:%v,msgid:%v,askid:%v,req:%v,resp:%v\n", s.ID(), s.UID(), msgid, askid, req, resp)
		return
	}
}

func (r *Router) handLoginScuess(s *tcp.Socket, uinfo *UserInfo) {
	uinfo.LoginAt = time.Now()

	s.MetaStore(uinfoKey, uinfo)
	s.SetUID(uinfo.UID)

	r.onUserOnline(s, uinfo)
}

func (r *Router) authToken(token string) (*UserInfo, error) {
	if r.Authc == nil {
		return nil, fmt.Errorf("auth is nil")
	}

	baseinfo, err := r.Authc.TokenAuth(token)
	if err != nil {
		return nil, err
	}

	uinfo := &UserInfo{
		UID:      baseinfo.ID,
		UserName: baseinfo.Name,
		Role:     baseinfo.Role,
		LoginAt:  time.Now(),
	}

	groups := r.roleGroups(baseinfo.Role)
	for _, v := range groups {
		uinfo.Groups.Store(v, struct{}{})
	}
	return uinfo, nil
}

func (r *Router) roleGroups(role string) []string {
	if r.Permitc == nil {
		return nil
	}
	return r.Permitc.RoleGroups(role)
}

func (r *Router) callEnable(s *tcp.Socket, msgid uint32) bool {
	if r.Permitc == nil {
		return true
	}
	uinfo := GetSocketUserInfo(s)
	if uinfo == nil {
		return false
	}
	return r.Permitc.CallEnable(uinfo.Role, msgid)
}

func (r *Router) forwardEnable(s *tcp.Socket, target *tcp.Socket, msgid uint32) bool {
	if r.Permitc == nil {
		return true
	}
	suinfo := GetSocketUserInfo(s)
	tuinfo := GetSocketUserInfo(target)
	if suinfo == nil || tuinfo == nil {
		return false
	}
	return r.Permitc.ForwardEnable(suinfo.Role, tuinfo.Role, msgid)
}
