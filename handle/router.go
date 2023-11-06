package handle

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ajenpan/surf/logger"
	"google.golang.org/protobuf/proto"

	"route/auth"
	"route/msg"
	"route/permission"

	"github.com/ajenpan/surf/server"
	"github.com/ajenpan/surf/utils/calltable"
)

func NewRouter() (*Router, error) {
	ret := &Router{
		userSession: make(map[uint64]server.Session),
	}
	ct := calltable.ExtractAsyncMethodByMsgID(msg.File_msg_route_proto.Messages(), ret)
	if ct == nil {
		return nil, fmt.Errorf("ExtractProtoFile failed")
	}
	ret.ct = ct
	return ret, nil
}

type UserInfo struct {
	UID      uint64
	UserName string
	Role     string
	Groups   sync.Map
	LoginAt  time.Time
}

type Router struct {
	userSession     map[uint64]server.Session
	userSessionLock sync.RWMutex

	UserSessions *UserSessions

	ct *calltable.CallTable[uint32]
	// gm session.Groups

	Authc   auth.Auth
	Permitc permission.Permit

	Selfinfo *UserInfo
}

const uinfoKey string = "uinfo"
const errcntKey string = "errcnt"

type tcpSocketKeyT struct{}
type tcpPacketKeyT struct{}

var tcpSocketKey = tcpSocketKeyT{}
var tcpPacketKey = tcpPacketKeyT{}

func (r *Router) OnSessionStatus(s server.Session, enable bool) {
	us := r.UserSessions.MustGetSessionMap(s.UID())
	if enable {
		if cnt := us.RemoveAll(); cnt > 0 {
			r.onUserOffline(s.UID())
		}
		if ok := us.Store(s); ok {
			r.onUserOnline(s)
		}
	} else {
		if ok := us.Remove(s.ID()); ok {
			r.onUserOffline(s.UID())
		}
	}
}

func (r *Router) OnSessionMessage(s server.Session, msg *server.Message) {
	// ptype := msg.Meta.MsgType
	meta := msg.Head
	if meta.Ttl <= 0 {
		return
	}

	if meta.TargetUid == 0 || meta.TargetUid == r.Selfinfo.UID {
		//call my self
		r.OnCall(s, msg)
		return
	}

	tsession := r.GetUserSession(meta.TargetUid)

	if tsession == nil {
		//TODO: send err to source
		return
	}

	if !r.forwardEnable(s, tsession, msg) {
		return
	}

	if meta.SourceUid == 0 {
		meta.SourceUid = s.UID()
	}

	meta.Ttl--
	err := s.Send(msg)
	if err != nil {
		logger.Error(err)
	}
}

func (r *Router) GetUserSession(uid uint64) server.Session {
	r.userSessionLock.RLock()
	defer r.userSessionLock.RUnlock()
	return r.userSession[uid]
}

func (r *Router) StoreUserSession(uid uint64, s server.Session) {
	r.userSessionLock.Lock()
	defer r.userSessionLock.Unlock()
	r.userSession[uid] = s
}

func (r *Router) RemoveUserSession(uid uint64, s server.Session) {
	r.userSessionLock.Lock()
	defer r.userSessionLock.Unlock()
	if r.userSession[uid] == s {
		delete(r.userSession, uid)
	}
}

func (r *Router) onUserOnline(s server.Session) {
	// r.StoreUserSession(uint64(uinfo.UID), s)

	// uinfo.Groups.Range(func(k, v interface{}) bool {
	// 	r.gm.AddTo(k.(string), uinfo.UID, s)
	// 	return true
	// })

	// r.PublishEvent(&msg.UserStatChange{
	// 	Sid:  s.ID(),
	// 	Uid:  uinfo.UID,
	// 	Stat: msg.UserStatChange_Online,
	// })
}

func (r *Router) onUserOffline(uid uint64) {
	// r.RemoveUserSession(uint64(uinfo.UID), s)

	// uinfo.Groups.Range(func(k, v interface{}) bool {
	// 	r.gm.RemoveFromGroup(k.(string), uinfo.UID, s)
	// 	return true
	// })

	// r.PublishEvent(&msg.UserStatChange{
	// 	Sid:  s.ID(),
	// 	Uid:  uinfo.UID,
	// 	Stat: msg.UserStatChange_Offline,
	// })
}

func (r *Router) PublishEvent(event proto.Message) {
	log.Printf("[Mock PublishEvent] name:%v, msg:%v\n", string(proto.MessageName(event).Name()), event)
}

func (r *Router) SendMessage(s server.Session, askid uint32, msgtyp server.RouteMsgTyp, m proto.Message) error {
	// raw, err := proto.Marshal(m)
	// if err != nil {
	// 	return fmt.Errorf("marshal failed:%v", err)
	// }

	// msgid := calltable.GetMessageMsgID(m)
	// if msgid == 0 {
	// 	return fmt.Errorf("not found msgid:%v", msgid)
	// }

	// return s.SendPacket(tcp.NewPackFrame(PacketTypRoute, nil, raw))
	return nil
}

func (r *Router) OnCall(s server.Session, msg *server.Message) {
	// var err error

	// msgid := int(head.GetMsgID())

	// enable := r.callEnable(s, uint32(msgid))
	// if !enable {
	// 	log.Print("not enable to call this method:", msgid)
	// 	dealSocketErrCnt(s)
	// 	return
	// }

	// askid := head.GetAskID()
	// method := r.ct.Get(msgid)
	// if method == nil {
	// 	log.Print("not found method,msgid:", msgid)
	// 	dealSocketErrCnt(s)
	// 	return
	// }

	// reqRaw := method.NewRequest()
	// if reqRaw == nil {
	// 	log.Print("not found request,msgid:", msgid)
	// 	return
	// }

	// req := reqRaw.(proto.Message)
	// err = proto.Unmarshal(body, req)

	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }

	// ctx := context.WithValue(context.Background(), tcpSocketKey, s)
	// ctx = context.WithValue(ctx, tcpPacketKey, p)

	// result := method.Call(r, ctx, req)

	// if len(result) != 2 {
	// 	return
	// }
	// // if err is not nil, only return err
	// resperrI := result[1].Interface()
	// if resperrI != nil {
	// 	var senderr error
	// 	switch resperr := resperrI.(type) {
	// 	case *msg.Error:
	// 		senderr = r.SendMessage(s, askid, RouteTypRespErr, resperr)
	// 	case error:
	// 		senderr = r.SendMessage(s, askid, RouteTypRespErr, &msg.Error{
	// 			Code:   -1,
	// 			Detail: resperr.Error(),
	// 		})
	// 	default:
	// 		log.Print("not support error type:")
	// 	}
	// 	if senderr != nil {
	// 		log.Print("send err failed:", senderr)
	// 	}
	// 	return
	// }

	// respI := result[0].Interface()
	// if respI != nil {
	// 	resp, ok := respI.(proto.Message)
	// 	if !ok {
	// 		return
	// 	}
	// 	respMsgTyp := head.GetMsgTyp()
	// 	if respMsgTyp == RouteTypRequest {
	// 		respMsgTyp = RouteTypResponse
	// 	}

	// 	r.SendMessage(s, askid, respMsgTyp, resp)
	// 	log.Printf("oncall sid:%v,uid:%v,msgid:%v,askid:%v,req:%v,resp:%v\n", s.ID(), s.UID(), msgid, askid, req, resp)
	// 	return
	// }
}

func (r *Router) handLoginScuess(s server.Session, uinfo *UserInfo) {
	uinfo.LoginAt = time.Now()

	// s.MetaStore(uinfoKey, uinfo)
	// r.onUserOnline(s, uinfo)
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

func (r *Router) callEnable(s server.Session, msgid uint32) bool {
	// if r.Permitc == nil {
	// 	return true
	// }
	// uinfo := GetSocketUserInfo(s)
	// if uinfo == nil {
	// 	return false
	// }
	// return r.Permitc.CallEnable(uinfo.Role, msgid)
	return false
}

func (r *Router) forwardEnable(s server.Session, target server.Session, msg *server.Message) bool {
	// if r.Permitc == nil {
	// 	return true
	// }
	// suinfo := GetSocketUserInfo(s)
	// tuinfo := GetSocketUserInfo(target)
	// if suinfo == nil || tuinfo == nil {
	// 	return false
	// }
	// return r.Permitc.ForwardEnable(suinfo.Role, tuinfo.Role, msgid)
	return true
}
