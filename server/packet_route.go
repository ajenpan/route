package server

import (
	"encoding/binary"
	"io"
)

const (
	ProtoBinaryRouteType uint8 = 1 // route
)

func init() {
	RegPacket(ProtoBinaryRouteType, func() Packet {
		return &RoutePacket{}
	})
}

type RoutePacketHead []byte

type RoutePacket struct {
	RoutePacketHead
	Body []byte
}

func (m *RoutePacket) GetMsgtype() uint8 {
	return m.RoutePacketHead[0]
}

func (m *RoutePacket) SetMsgtype(typ uint8) {
	m.RoutePacketHead[0] = typ
}

func (m *RoutePacket) GetUid() uint32 {
	return binary.LittleEndian.Uint32(m.RoutePacketHead[4:8])
}

func (m *RoutePacket) SetUid(uid uint32) {
	binary.LittleEndian.PutUint32(m.RoutePacketHead[4:8], uid)
}

func (m *RoutePacket) PacketType() uint8 {
	return ProtoBinaryRouteType
}

func (m *RoutePacket) ReadFrom(r io.Reader) (int64, error) {
	// 3+1+4 + bodylen
	var err error
	_, err = io.ReadFull(r, m.RoutePacketHead)
	if err != nil {
		return 0, err
	}
	bodylen := GetUint24(m.RoutePacketHead[1:3])
	if bodylen > 0 {
		m.Body = make([]byte, bodylen)
		_, err = io.ReadFull(r, m.Body)
	}
	return (8 + int64(bodylen)), err
}

func (m *RoutePacket) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(m.RoutePacketHead)
	if err != nil {
		return 0, err
	}
	if len(m.Body) > 0 {
		bn, err := w.Write(m.Body)
		if err != nil {
			return 0, err
		}
		n += bn
	}
	return int64(n), nil
}
