package server

import (
	"encoding/binary"
	"io"
)

func NewRoutePacket() *RoutePacket {
	return &RoutePacket{
		RoutePacketHead: make(RoutePacketHead, RoutePacketHeadLen),
	}
}

const RoutePacketHeadLen = 8

type RoutePacketHead []byte

type RoutePacket struct {
	RoutePacketHead
	Body []byte
}

func (m *RoutePacket) GetMsgtype() byte {
	return m.RoutePacketHead[0]
}

func (m *RoutePacket) SetMsgtype(typ byte) {
	m.RoutePacketHead[0] = typ
}

func (m *RoutePacket) GetUid() uint32 {
	return binary.LittleEndian.Uint32(m.RoutePacketHead[4:8])
}

func (m *RoutePacket) SetUid(uid uint32) {
	binary.LittleEndian.PutUint32(m.RoutePacketHead[4:8], uid)
}

func (m *RoutePacket) PacketType() byte {
	return RoutePacketType
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
	return (RoutePacketHeadLen + int64(bodylen)), err
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
