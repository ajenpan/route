package tcp

import (
	"encoding/binary"
	"errors"
)

// Codec constants.
const (
	//8MB 8388607
	MaxPacketSize = 0x7FFFFF
)

var ErrWrongPacketType = errors.New("wrong packet type")
var ErrBodySizeWrong = errors.New("packet body size error")
var ErrHeadSizeWrong = errors.New("packet head size error")
var ErrParseHead = errors.New("parse packet error")
var ErrDisconn = errors.New("socket disconnected")

// -<PacketType>-|-<HeadLen>-|-<BodyLen>-|-<Body>-
// -1------------|-1---------|-3---------|--------

type PacketType = uint8

const (
	// inner
	PacketTypInnerStartAt_ PacketType = iota
	PacketTypHeartbeat
	PacketTypAck
	PacketTypInnerEndAt_ PacketType = 0x0f
	// user
	PacketTypStartAt_   PacketType = 0x0f
	PacketTypRoute      PacketType = 0x0f + 1
	PacketTypRouteBrige PacketType = 0x0f + 2
	PacketTypEndAt_     PacketType = 0xff
)

func Uint24(b []uint8) uint32 {
	_ = b[2] // bounds check hint to compiler; see golang.org/issue/14808
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16
}

func PutUint24(b []uint8, v uint32) {
	_ = b[2] // early bounds check to guarantee safety of writes below
	b[0] = uint8(v)
	b[1] = uint8(v >> 8)
	b[2] = uint8(v >> 16)
}

type PackMeta [1 + 1 + 3]uint8

func (hr PackMeta) GetType() uint8 {
	return hr[0]
}
func (hr PackMeta) GetHeadLen() uint8 {
	return hr[1]
}

func (hr PackMeta) GetBodyLen() uint32 {
	return Uint24(hr[2:5])
}

func (hr PackMeta) SetType(t uint8) {
	hr[0] = t
}

func (hr PackMeta) SetHeadLen(t uint8) {
	hr[1] = t
}

func (hr PackMeta) SetBodyLen(l uint32) {
	PutUint24(hr[2:4], l)
}

func (hr PackMeta) Reset() {
	for i := 0; i < len(hr); i++ {
		hr[i] = 0
	}
}

func (hr PackMeta) Clone() PackMeta {
	ret := PackMeta{}
	copy(ret[:], hr[:])
	return ret
}

type PackFrame struct {
	PackMeta
	Head []uint8
	Body []uint8
}

func (p *PackFrame) Reset() {
	p.PackMeta.Reset()
	p.Body = p.Body[:0]
}

func (p *PackFrame) Clone() *PackFrame {
	return &PackFrame{
		PackMeta: p.PackMeta.Clone(),
		Body:     p.Body[:],
	}
}

type RouteHead []uint8

const RouteHeadLen = 25

type RouteMsgTyp uint8

const (
	RouteTypAsync RouteMsgTyp = iota
	RouteTypRequest
	RouteTypResponse
)

func CastRoutHead(head []uint8) (RouteHead, error) {
	if len(head) != RouteHeadLen {
		return nil, ErrHeadSizeWrong
	}
	return RouteHead(head), nil
}

func NewRoutHead() RouteHead {
	return make([]uint8, RouteHeadLen)
}

func (h RouteHead) GetTargetUID() uint32 {
	return binary.LittleEndian.Uint32(h[0:4])
}

func (h RouteHead) GetSrouceUID() uint32 {
	return binary.LittleEndian.Uint32(h[4:8])
}

func (h RouteHead) GetAskID() uint32 {
	return binary.LittleEndian.Uint32(h[16:20])
}

func (h RouteHead) GetMsgID() uint32 {
	return binary.LittleEndian.Uint32(h[20:24])
}

func (h RouteHead) GetMsgTyp() RouteMsgTyp {
	return RouteMsgTyp(h[24])
}

func (h RouteHead) SetTargetUID(u uint32) {
	binary.LittleEndian.PutUint32(h[0:4], u)
}

func (h RouteHead) SetSrouceUID(u uint32) {
	binary.LittleEndian.PutUint32(h[4:8], u)
}

func (h RouteHead) SetAskID(id uint32) {
	binary.LittleEndian.PutUint32(h[16:20], id)
}

func (h RouteHead) SetMsgID(id uint32) {
	binary.LittleEndian.PutUint32(h[20:24], id)
}

func (h RouteHead) SetMsgTyp(typ RouteMsgTyp) {
	h[24] = uint8(typ)
}

type RouteErrHead RouteHead
