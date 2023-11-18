package tcp

import (
	"errors"
	"io"
)

const (
	// 65535
	MaxPacketHeadSize = 0xFFFF
	// 8388607
	MaxPacketBodySize = 0x7FFFFF
)

var ErrWrongPacketType = errors.New("wrong packet type")
var ErrWronMetaData = errors.New("wrong packet meta data")
var ErrBodySizeWrong = errors.New("packet body size error")
var ErrHeadSizeWrong = errors.New("packet head size error")
var ErrParseHead = errors.New("parse packet error")
var ErrDisconn = errors.New("socket disconnected")

// |--------------Meta--------------------|--------Body-----|
// |-<PacketType>-|-<HeadLen>-|-<BodyLen>-|-<Head>-|-<Body>-|
// |-1------------|-2---------|-3---------|-N------|-N------|

type PacketType = uint8

const (
	// inner 224
	PacketTypeInnerStartAt_ PacketType = 0xE0
	// ack
	PacketTypeHandShake     PacketType = PacketTypeInnerStartAt_ + iota
	PacketTypeActionRequire PacketType = PacketTypeInnerStartAt_ + iota
	PacketTypeDoAction      PacketType = PacketTypeInnerStartAt_ + iota
	PacketTypeAckResult     PacketType = PacketTypeInnerStartAt_ + iota

	//
	PacketTypeHeartbeat PacketType = PacketTypeInnerStartAt_ + iota
	PacketTypeEcho      PacketType = PacketTypeInnerStartAt_ + iota

	PacketTypeInnerEndAt_ PacketType = PacketTypeInnerStartAt_ + iota
)

const (
	PacketTypeTHVPacket PacketType = 0
)

type Packet interface {
	io.ReaderFrom
	io.WriterTo
}

func GetUint24(b []uint8) uint32 {
	_ = b[2] // bounds check hint to compiler; see golang.org/issue/14808
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16
}

func PutUint24(b []uint8, v uint32) {
	_ = b[2] // early bounds check to guarantee safety of writes below
	b[0] = uint8(v)
	b[1] = uint8(v >> 8)
	b[2] = uint8(v >> 16)
}

func GetUint16(b []uint8) uint16 {
	_ = b[1]
	return uint16(b[0]) | uint16(b[1])<<8
}

func PutUint16(b []uint8, v uint16) {
	_ = b[1]
	b[0] = uint8(v)
	b[1] = uint8(v >> 8)
}

const PackMetaLen = 6

type THVPacketMeta []uint8

func newPackMeta() THVPacketMeta {
	return make([]uint8, PackMetaLen)
}
func (hr THVPacketMeta) GetType() uint8 {
	return hr[0]
}
func (hr THVPacketMeta) SetType(l uint8) {
	hr[0] = l
}
func (h THVPacketMeta) GetHeadLen() uint16 {
	return GetUint16(h[1:3])
}
func (hr THVPacketMeta) SetHeadLen(l uint16) {
	PutUint16(hr[1:3], l)
}
func (hr THVPacketMeta) GetBodyLen() uint32 {
	return GetUint24(hr[3:6])
}
func (hr THVPacketMeta) SetBodyLen(l uint32) {
	PutUint24(hr[3:6], l)
}

func (hr THVPacketMeta) Reset() {
	for i := 0; i < len(hr); i++ {
		hr[i] = 0
	}
}

func newEmptyTHVPacket() *THVPacket {
	return &THVPacket{
		meta: newPackMeta(),
	}
}

func NewTHVPacket(PacketType uint8, h []uint8, b []uint8) *THVPacket {
	p := newEmptyTHVPacket()
	p.SetType(PacketType)
	p.SetHead(h)
	p.SetBody(b)
	return p
}

type THVPacket struct {
	meta THVPacketMeta
	Head []uint8
	Body []uint8
}

func (p *THVPacket) ReadFrom(reader io.Reader) (int64, error) {
	var err error
	if _, err = io.ReadFull(reader, p.meta); err != nil {
		return 0, err
	}

	headlen := p.meta.GetHeadLen()
	bodylen := p.meta.GetBodyLen()

	if headlen > MaxPacketHeadSize || bodylen > MaxPacketBodySize {
		return 0, ErrWronMetaData
	}

	if headlen > 0 {
		p.Head = make([]uint8, headlen)
		_, err = io.ReadFull(reader, p.Head)
		if err != nil {
			return 0, err
		}
	}

	if bodylen > 0 {
		p.Body = make([]uint8, bodylen)
		_, err = io.ReadFull(reader, p.Body)
		if err != nil {
			return 0, err
		}
	}
	return int64(PackMetaLen + int(headlen) + int(bodylen)), nil
}

func (p *THVPacket) WriteTo(writer io.Writer) (int64, error) {
	_, err := writer.Write(p.meta)
	if err != nil {
		return 0, err
	}
	if len(p.Head) > 0 {
		_, err = writer.Write(p.Head)
		if err != nil {
			return 0, err
		}
	}
	if len(p.Body) > 0 {
		_, err = writer.Write(p.Body)
		if err != nil {
			return 0, err
		}
	}
	return int64(PackMetaLen + len(p.Head) + len(p.Body)), nil
}

func (p *THVPacket) Name() string {
	return "THVPacket"
}

func (p *THVPacket) Reset() {
	p.meta.Reset()
	if p.Head != nil {
		p.Head = p.Head[:0]
	}
	if p.Body != nil {
		p.Body = p.Body[:0]
	}
}

func (p *THVPacket) Clone() *THVPacket {
	ret := &THVPacket{
		meta: p.meta[:],
	}
	if p.Head != nil {
		ret.Head = p.Head[:]
	}
	if p.Body != nil {
		ret.Body = p.Body[:]
	}
	return ret
}

func (p *THVPacket) SetType(t uint8) {
	p.meta.SetType(t)
}

func (p *THVPacket) GetType() uint8 {
	return p.meta.GetType()
}

func (p *THVPacket) SetHead(b []uint8) {
	if len(b) > MaxPacketHeadSize {
		panic(ErrHeadSizeWrong)
	}
	p.Head = b
	p.meta.SetHeadLen(uint16(len(b)))
}

func (p *THVPacket) GetHead() []uint8 {
	return p.Head
}

func (p *THVPacket) SetBody(b []uint8) {
	if len(b) > MaxPacketBodySize {
		panic(ErrBodySizeWrong)
	}
	p.Body = b
	p.meta.SetBodyLen(uint32(len(b)))
}

func (p *THVPacket) GetBody() []uint8 {
	return p.Body
}

func (p *THVPacket) GetMeta() THVPacketMeta {
	return p.meta
}
