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
	PacketTypInnerEndAt_
)

const (
	// user
	PacketTypStartAt_ PacketType = 0x0f + iota
	PacketTypRoute
	PacketTypRouteBrige
	PacketTypEndAt_
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

const PackMetaLen = 5

type PackMeta []uint8

func IsValidPacketType(t uint8) bool {
	return t > PacketTypStartAt_ && t < PacketTypEndAt_ || t > PacketTypInnerStartAt_ && t < PacketTypInnerEndAt_
}

func NewPackMeta() PackMeta {
	return make([]uint8, PackMetaLen)
}

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
	PutUint24(hr[2:5], l)
}

func (hr PackMeta) Reset() {
	for i := 0; i < len(hr); i++ {
		hr[i] = 0
	}
}

func NewEmptyPackFrame() *PackFrame {
	return &PackFrame{
		meta: NewPackMeta(),
	}
}

func NewPackFrame(t uint8, h []uint8, b []uint8) *PackFrame {
	p := NewEmptyPackFrame()
	p.SetType(t)
	p.SetHead(h)
	p.SetBody(b)
	return p
}

type PackFrame struct {
	meta PackMeta
	head []uint8
	body []uint8
}

func (p *PackFrame) Name() string {
	return "tcp-binary"
}

func (p *PackFrame) Reset() {
	p.meta.Reset()
	p.body = p.body[:0]
}

func (p *PackFrame) Clone() *PackFrame {
	return &PackFrame{
		meta: p.meta[:],
		head: p.head[:],
		body: p.body[:],
	}
}

func (p *PackFrame) SetType(t uint8) {
	p.meta.SetType(t)
}
func (p *PackFrame) GetType() uint8 {
	return p.meta.GetType()
}

func (p *PackFrame) SetHead(h []uint8) {
	p.head = h
	p.meta.SetHeadLen(uint8(len(h)))
}

func (p *PackFrame) SetBody(b []uint8) {
	p.body = b
	p.meta.SetBodyLen(uint32(len(b)))
}

func (p *PackFrame) GetHead() []uint8 {
	return p.head
}
func (p *PackFrame) GetBody() []uint8 {
	return p.body
}

type RouteHead []uint8

const RouteHeadLen = 17

type RouteMsgTyp uint8

const (
	RouteTypAsync RouteMsgTyp = iota
	RouteTypRequest
	RouteTypResponse
	RouteTypRespErr
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
	return binary.LittleEndian.Uint32(h[8:12])
}

func (h RouteHead) GetMsgID() uint32 {
	return binary.LittleEndian.Uint32(h[12:16])
}

func (h RouteHead) GetMsgTyp() RouteMsgTyp {
	return RouteMsgTyp(h[16])
}

func (h RouteHead) SetTargetUID(u uint32) {
	binary.LittleEndian.PutUint32(h[0:4], u)
}

func (h RouteHead) SetSrouceUID(u uint32) {
	binary.LittleEndian.PutUint32(h[4:8], u)
}

func (h RouteHead) SetAskID(id uint32) {
	binary.LittleEndian.PutUint32(h[8:12], id)
}

func (h RouteHead) SetMsgID(id uint32) {
	binary.LittleEndian.PutUint32(h[12:16], id)
}

func (h RouteHead) SetMsgTyp(typ RouteMsgTyp) {
	h[16] = uint8(typ)
}

type RouteErrHead RouteHead
