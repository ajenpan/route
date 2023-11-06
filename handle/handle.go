package handle

import (
	"context"

	"route/msg"
)

func (r *Router) OnEcho(ctx context.Context, req *msg.Echo) (*msg.Echo, error) {
	return req, nil
}

func (r *Router) OnListGroupRequest(ctx context.Context, req *msg.ListGroupRequest, resp *msg.ListGroupResponse) error {
	// resp.Groups = r.gm.Groups()
	return nil
}

func (r *Router) OnGroupBroadcastRequest(ctx context.Context, req *msg.GroupBroadcastRequest) (*msg.GroupBroadcastResponse, error) {
	// ss := GetSocketFromCtx(ctx)
	// ssuid := ss.UID()

	// head := tcp.NewRoutHead()
	// head.SetMsgID(req.Msgid)
	// head.SetSrouceUID(ss.UID())
	// head.SetTargetUID(0)
	// p := tcp.NewPackFrame(tcp.PacketTypRoute, head, req.Msgdata)

	// resp := &msg.GroupBroadcastResponse{}

	// g := r.gm.GetGroup(req.Group)
	// if g == nil {
	// 	return resp, nil
	// }

	// sockets := g.GetAll()
	// for _, s := range sockets {
	// 	if s.UID() == ssuid {
	// 		continue
	// 	}
	// 	if s.SendPacket(p) == nil {
	// 		resp.RecvCount++
	// 	}
	// }
	return nil, nil
}

func (r *Router) OnPutInGroupRequest(ctx context.Context, req *msg.PutInGroupRequest, resp *msg.PutInGroupResponse) error {
	// group := r.gm.GetGroup(req.Group)
	// if group == nil {
	// 	resp.Errcode = msg.PutInGroupResponse_group_not_found
	// 	return nil
	// }

	// for _, uid := range req.Invite {
	// 	s := r.GetUserSession(uint64(uid))
	// 	if s == nil {
	// 		continue
	// 	}
	// 	group.Add(uint64(uid), s)
	// 	resp.Invite = append(resp.Invite, uid)
	// }

	// resp.Errcode = msg.PutInGroupResponse_ok
	return nil
}

func (r *Router) OnListGroupSessionRequest(ctx context.Context, req *msg.ListGroupSessionRequest, resp *msg.ListGroupSessionResponse) error {
	// page := req.Page
	// if page == nil {
	// 	return &msg.Error{Detail: "page is nil"}
	// }

	// group := r.gm.GetGroup(req.Group)
	// if group == nil {
	// 	return &msg.Error{
	// 		Detail: "group not found",
	// 	}
	// }

	// list, total := group.Range(int(page.StartAt), int(page.EndAt))
	// resp.Total = uint32(total)

	// resp.Uinfos = make([]*msg.UserInfo, len(list))

	// for i, s := range list {
	// 	uinfo := GetSocketUserInfo(s)
	// 	if uinfo == nil {
	// 		continue
	// 	}
	// 	resp.Uinfos[i] = &msg.UserInfo{
	// 		Uid:   uinfo.UID,
	// 		Uname: uinfo.UserName,
	// 		Role:  uinfo.Role,
	// 	}
	// }
	return nil
}
