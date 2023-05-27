package handle

import (
	"context"
	"fmt"
	"hash/fnv"

	"route/transport/tcp"
)

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

func StringToInt64(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}
