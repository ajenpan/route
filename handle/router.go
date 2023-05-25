package handle

import (
	"context"
	"crypto/rsa"
	"fmt"
	"hash/fnv"
	"strconv"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/protobuf/proto"

	"route/group"
	msg "route/proto"
	"route/transport/tcp"
	"route/utils/calltable"
)

func NewRouter() (*Router, error) {
	ret := &Router{}
	ct := calltable.ExtractAsyncMethodByMsgID(msg.File_proto_route_proto.Messages(), ret)
	if ct == nil {
		return nil, fmt.Errorf("ExtractProtoFile failed")
	}
	ret.ct = ct
	return ret, nil
}

type Router struct {
	user    sync.Map
	session sync.Map

	PublicKey *rsa.PublicKey
	ct        *calltable.CallTable[int]

	groups group.Manager
}

type UserInfo struct {
	UID      uint32
	UserName string
	Role     string
	Groups   sync.Map
	LoginAt  time.Time
}

const uinfoKey string = "uinfo"
const errcntKey string = "errcnt"

type tcpSocketKeyT struct{}
type tcpPacketKeyT struct{}

var tcpSocketKey = tcpSocketKeyT{}
var tcpPacketKey = tcpPacketKeyT{}

func VerifyToken(pk *rsa.PublicKey, tokenRaw string) (uint32, string, string, error) {
	claims := make(jwt.MapClaims)
	token, err := jwt.ParseWithClaims(tokenRaw, claims, func(t *jwt.Token) (interface{}, error) {
		return pk, nil
	})
	if err != nil {
		return 0, "", "", err
	}
	if !token.Valid {
		return 0, "", "", fmt.Errorf("invalid token")
	}
	uname := claims["sub"]
	uidstr := claims["uid"]
	role := claims["role"]
	uid, err := strconv.ParseUint(uidstr.(string), 10, 64)
	return uint32(uid), uname.(string), role.(string), err
}

func GetSocketUserInfo(s *tcp.Socket) *UserInfo {
	if s == nil {
		return nil
	}
	if v, ok := s.MetaLoad(uinfoKey); ok {
		return v.(*UserInfo)
	}
	return nil
}

func addSocketErrCnt(s *tcp.Socket) int {
	if v, ok := s.MetaLoad(errcntKey); ok {
		cnt := v.(int)
		cnt++
		s.MetaStore(errcntKey, cnt)
		return cnt
	}
	s.MetaStore(errcntKey, 1)
	return 1
}

func dealSocketErrCnt(s *tcp.Socket) {
	cnt := addSocketErrCnt(s)
	fmt.Printf("socket:%v, uid:%v, errcnt:%v", s.ID(), s.UID(), cnt)
}

func (r *Router) OnMessage(s *tcp.Socket, p *tcp.PackFrame) {
	ptype := p.GetType()

	if ptype == tcp.PacketTypRoute {
		head, err := tcp.CastRoutHead(p.Head)
		if err != nil {
			return
		}
		targetid := head.GetTargetUID()
		if targetid == 0 {
			r.OnCall(s, p, head, p.Body)
			return
		}

		suid := s.UID()
		if suid != 0 {
			tsocket := r.GetSocketByUID(targetid)
			if tsocket != nil {
				head.SetSrouceUID(suid)
				err = tsocket.SendPacket(p)
			} else {
				err = fmt.Errorf("targetid:%d not found", targetid)
			}
		} else {
			err = fmt.Errorf("not login")
		}
		if err != nil {
			fmt.Println(err)
		}
	}
}

func (r *Router) OnConn(s *tcp.Socket, stat tcp.SocketStat) {
	fmt.Println("OnConn", s.ID(), ",stat:", int(stat))

	if stat == tcp.Disconnected {
		r.session.Delete(s.ID())
		uinfo := GetSocketUserInfo(s)
		if uinfo != nil {
			r.OnUserOffline(s, uinfo)
		}
	} else {
		r.session.Store(s.ID(), s)
	}
}

func (r *Router) OnUserOnline(s *tcp.Socket, uinfo *UserInfo) {
	r.user.Store(uinfo.UID, s)
	uinfo.Groups.Range(func(k, v interface{}) bool {
		r.groups.AddTo(k.(string), uinfo.UID, s)
		return true
	})

	r.PublishEvent(&msg.UserStatChange{
		Sid:  s.ID(),
		Uid:  uinfo.UID,
		Stat: msg.UserStatChange_Online,
	})
}

func (r *Router) OnUserOffline(s *tcp.Socket, uinfo *UserInfo) {
	r.user.Delete(uinfo.UID)

	uinfo.Groups.Range(func(k, v interface{}) bool {
		r.groups.RemoveFrom(k.(string), uinfo.UID)
		return true
	})

	r.PublishEvent(&msg.UserStatChange{
		Sid:  s.ID(),
		Uid:  uinfo.UID,
		Stat: msg.UserStatChange_Offline,
	})
}

func (r *Router) PublishEvent(event proto.Message) {
	fmt.Printf("PublishEvent name:%v, msg:%v\n", string(proto.MessageName(event).Name()), event)
}

func (r *Router) SendMessage(s *tcp.Socket, askid uint32, msgtyp tcp.RouteMsgTyp, m proto.Message) error {
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
	head.SetSrouceUID(0)
	head.SetTargetUID(s.UID())
	head.SetAskID(askid)
	head.SetMsgTyp(msgtyp)

	p := &tcp.PackFrame{
		Body: raw,
	}
	p.SetType(tcp.PacketTypRoute)
	p.SetBodyLen(uint32(len(raw)))
	p.SetHeadLen(uint8(len(head)))
	return s.SendPacket(p)
}

func (r *Router) GetSocketByUID(uid uint32) *tcp.Socket {
	if v, ok := r.user.Load(uid); ok {
		return v.(*tcp.Socket)
	}
	return nil
}

func (r *Router) OnCall(s *tcp.Socket, p *tcp.PackFrame, head tcp.RouteHead, body []byte) {
	var err error
	msgid := int(head.GetMsgID())
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

	resperrI := result[1].Interface()
	if resperrI != nil {
		resperr, ok := resperrI.(error)
		if !ok {
			return
		}

		fmt.Println("resperr:", resperr)
		dealSocketErrCnt(s)
	}
}

func GetSocketFromCtx(ctx context.Context) *tcp.Socket {
	if v, ok := ctx.Value(tcpSocketKey).(*tcp.Socket); ok {
		return v
	}
	return nil
}

func GetPacketFromCtx(ctx context.Context) *tcp.PackFrame {
	if v, ok := ctx.Value(tcpPacketKey).(*tcp.PackFrame); ok {
		return v
	}
	return nil
}

func (r *Router) OnLoginRequest(ctx context.Context, req *msg.LoginRequest) (*msg.LoginResponse, error) {
	resp := &msg.LoginResponse{
		Errcode: msg.LoginResponse_unkown_err,
	}

	uid, uname, role, err := VerifyToken(r.PublicKey, req.Token)
	if err != nil {
		resp.Errcode = msg.LoginResponse_invalid_token
		return resp, nil
	}

	if olds := r.GetSocketByUID(uid); olds != nil {
		olds.Close()
	}

	uinfo := &UserInfo{
		UID:      uid,
		UserName: uname,
		Role:     role,
	}
	s := GetSocketFromCtx(ctx)

	r.handLoginScuess(s, uinfo)

	resp.Errcode = msg.LoginResponse_ok
	return resp, nil
}

func (r *Router) handLoginScuess(s *tcp.Socket, uinfo *UserInfo) {
	uinfo.LoginAt = time.Now()

	s.MetaStore(uinfoKey, uinfo)
	s.SetUID(uinfo.UID)

	r.OnUserOnline(s, uinfo)
}

func stringToInt64(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

func (r *Router) OnAccountLoginRequest(ctx context.Context, req *msg.AccountLoginRequest) (*msg.AccountLoginResponse, error) {
	resp := &msg.AccountLoginResponse{
		Errcode: msg.AccountLoginResponse_unkown_err,
	}

	uid := stringToInt64(req.Account)
	s := GetSocketFromCtx(ctx)

	r.handLoginScuess(s, &UserInfo{
		UID:      uint32(uid),
		UserName: req.Account,
		Role:     "user",
	})

	resp.Uinfo = &msg.UserInfo{
		Uid: uid,
	}
	return resp, nil
}

func (r *Router) OnEchoRequest(ctx context.Context, req *msg.EchoRequest) (*msg.EchoResponse, error) {
	resp := &msg.EchoResponse{
		Msg: req.Msg,
	}
	return resp, nil
}

func (r *Router) OnGroupBroadcastRequest(ctx context.Context, req *msg.GroupBroadcastRequest) (*msg.GroupBroadcastResponse, error) {

	ss := GetSocketFromCtx(ctx)

	head := tcp.NewRoutHead()
	head.SetMsgID(req.Msgid)
	head.SetSrouceUID(ss.UID())
	head.SetTargetUID(0)
	p := &tcp.PackFrame{
		Body: req.Msgdata,
	}

	p.SetType(tcp.PacketTypRoute)
	p.SetBodyLen(uint32(len(req.Msgdata)))
	p.SetHeadLen(uint8(len(head)))

	resp := &msg.GroupBroadcastResponse{}
	r.user.Range(func(key, value interface{}) bool {
		s := value.(*tcp.Socket)
		if s.SendPacket(p) == nil {
			resp.RecvCount++
		}
		return true
	})
	return resp, nil
}
