package server

import (
	"encoding/binary"
	"io"
	"route/server/tcp"
)

const (
	ProtoBinaryRouteType uint8 = 1 // route
)

func init() {
	tcp.RegPacket(ProtoBinaryRouteType, func() tcp.Packet {
		return &Message{}
	})
}

type msgHead []byte

type Message struct {
	msgHead
	Body []byte
}

func (m *Message) GetMsgtype() uint8 {
	return m.msgHead[0]
}

func (m *Message) SetMsgtype(typ uint8) {
	m.msgHead[0] = typ
}

func (m *Message) GetUid() uint32 {
	return binary.LittleEndian.Uint32(m.msgHead[4:8])
}

func (m *Message) SetUid(uid uint32) {
	binary.LittleEndian.PutUint32(m.msgHead[4:8], uid)
}

func (m *Message) PacketType() uint8 {
	return ProtoBinaryRouteType
}

func (m *Message) ReadFrom(r io.Reader) (int64, error) {
	// 3+1+4 + bodylen
	var err error
	_, err = io.ReadFull(r, m.msgHead)
	if err != nil {
		return 0, err
	}
	bodylen := tcp.GetUint24(m.msgHead[1:3])
	if bodylen > 0 {
		m.Body = make([]byte, bodylen)
		_, err = io.ReadFull(r, m.Body)
	}
	return (8 + int64(bodylen)), err
}

func (m *Message) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(m.msgHead)
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
