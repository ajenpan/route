package handle

import (
	"context"

	msg "route/proto"
	"route/transport/tcp"
)

func (r *Router) OnLoginRequest(ctx context.Context, req *msg.LoginRequest) (*msg.LoginResponse, error) {
	resp := &msg.LoginResponse{
		Errcode: msg.LoginResponse_unkown_err,
	}
	uinfo, err := r.authToken(req.Token)
	if err != nil {
		resp.Errcode = msg.LoginResponse_invalid_token
		return resp, nil
	}
	if olds := r.GetUserSession(uinfo.UID); olds != nil {
		olds.Close()
	}
	s := GetSocketFromCtx(ctx)
	r.handLoginScuess(s, uinfo)
	resp.Errcode = msg.LoginResponse_ok
	return resp, nil
}

func (r *Router) OnEcho(ctx context.Context, req *msg.Echo) (*msg.Echo, error) {
	return req, nil
}

func (r *Router) OnListGroupRequest(ctx context.Context, req *msg.ListGroupRequest) (*msg.ListGroupResponse, error) {
	resp := &msg.ListGroupResponse{
		Groups: r.gm.Groups(),
	}
	return resp, nil
}

func (r *Router) OnGroupBroadcastRequest(ctx context.Context, req *msg.GroupBroadcastRequest) (*msg.GroupBroadcastResponse, error) {
	ss := GetSocketFromCtx(ctx)
	ssuid := ss.UID()

	head := tcp.NewRoutHead()
	head.SetMsgID(req.Msgid)
	head.SetSrouceUID(ss.UID())
	head.SetTargetUID(0)
	p := tcp.NewPackFrame(tcp.PacketTypRoute, head, req.Msgdata)

	resp := &msg.GroupBroadcastResponse{}

	g := r.gm.GetGroup(req.Group)
	if g == nil {
		return resp, nil
	}

	sockets := g.GetAll()
	for _, s := range sockets {
		if s.UID() == ssuid {
			continue
		}
		if s.SendPacket(p) == nil {
			resp.RecvCount++
		}
	}
	return resp, nil
}

func (r *Router) OnPutInGroupRequest(ctx context.Context, req *msg.PutInGroupRequest) (*msg.PutInGroupResponse, error) {
	resp := &msg.PutInGroupResponse{
		Errcode: msg.PutInGroupResponse_unkown_err,
	}
	group := r.gm.GetGroup(req.Group)
	if group == nil {
		resp.Errcode = msg.PutInGroupResponse_group_not_found
		return resp, nil
	}

	for _, uid := range req.Invite {
		s := r.GetUserSession(uid)
		if s == nil {
			continue
		}
		group.Add(uid, s)
		resp.Invite = append(resp.Invite, uid)
	}

	resp.Errcode = msg.PutInGroupResponse_ok
	return resp, nil
}

func (r *Router) OnListGroupSessionRequest(ctx context.Context, req *msg.ListGroupSessionRequest) (*msg.ListGroupSessionResponse, error) {
	resp := &msg.ListGroupSessionResponse{}
	page := req.Page
	if page == nil {
		return nil, &msg.Error{Detail: "page is nil"}
	}

	group := r.gm.GetGroup(req.Group)
	if group == nil {
		return nil, &msg.Error{
			Detail: "group not found",
		}
	}

	list, total := group.Range(int(page.StartAt), int(page.EndAt))
	resp.Total = uint32(total)

	resp.Uinfos = make([]*msg.UserInfo, len(list))

	for i, s := range list {
		uinfo := GetSocketUserInfo(s)
		if uinfo == nil {
			continue
		}
		resp.Uinfos[i] = &msg.UserInfo{
			Uid:   uinfo.UID,
			Uname: uinfo.UserName,
			Role:  uinfo.Role,
		}
	}
	return resp, nil
}
