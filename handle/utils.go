package handle

import (
	"context"
	"fmt"
	"hash/fnv"

	"github.com/ajenpan/surf/server"
	"github.com/ajenpan/surf/server/tcp"
)

func GetSocketUserInfo(s server.Session) *UserInfo {
	// if s == nil {
	// 	return nil
	// }
	// if v, ok := s.MetaLoad(uinfoKey); ok {
	// 	return v.(*UserInfo)
	// }
	return nil
}

func addSocketErrCnt(s server.Session) int {
	// if v, ok := s.MetaLoad(errcntKey); ok {
	// 	cnt := v.(int)
	// 	cnt++
	// 	s.MetaStore(errcntKey, cnt)
	// 	return cnt
	// }
	// s.MetaStore(errcntKey, 1)
	return 1
}

func dealSocketErrCnt(s server.Session) {
	cnt := addSocketErrCnt(s)
	fmt.Printf("socket:%v, uid:%v, errcnt:%v", s.ID(), s.UID(), cnt)
}

func GetSocketFromCtx(ctx context.Context) server.Session {
	if v, ok := ctx.Value(tcpSocketKey).(server.Session); ok {
		return v
	}
	return nil
}

func GetPacketFromCtx(ctx context.Context) *tcp.THVPacket {
	if v, ok := ctx.Value(tcpPacketKey).(*tcp.THVPacket); ok {
		return v
	}
	return nil
}

func StringToInt64(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}
