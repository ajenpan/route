package server

import "io"

type hvPacketFlag = uint8

const (
	// inner 224
	hvPacketTypeInnerStartAt_ hvPacketFlag = 0xE0
	hvPacketFlagHandShake     hvPacketFlag = hvPacketTypeInnerStartAt_ + iota
	hvPacketFlagActionRequire hvPacketFlag = hvPacketTypeInnerStartAt_ + iota
	hvPacketFlagDoAction      hvPacketFlag = hvPacketTypeInnerStartAt_ + iota
	hvPacketFlagAckResult     hvPacketFlag = hvPacketTypeInnerStartAt_ + iota
	hvPacketFlagHeartbeat     hvPacketFlag = hvPacketTypeInnerStartAt_ + iota
	hvPacketFlagEcho          hvPacketFlag = hvPacketTypeInnerStartAt_ + iota
	hvPacketFlagMessage       hvPacketFlag = hvPacketTypeInnerStartAt_ + iota
	PacketTypeInnerEndAt_     hvPacketFlag = hvPacketTypeInnerStartAt_ + iota
)

// const hvPacketMaxBodySize = 0x7FFFFF

const hvPackMetaLen = 4

type HVHead []uint8

type HVPacket struct {
	head HVHead
	body []uint8
}

func newHead() HVHead {
	return make([]uint8, hvPackMetaLen)
}

func newHVPacket() *HVPacket {
	return &HVPacket{
		head: newHead(),
	}
}

func (hr HVHead) getType() uint8 {
	return hr[0]
}

func (hr HVHead) setType(l uint8) {
	hr[0] = l
}

func (h HVHead) getBodyLen() uint32 {
	return GetUint24(h[1:4])
}

func (hr HVHead) setBodyLen(l uint32) {
	PutUint24(hr[1:4], l)
}

func (hr HVHead) reset() {
	for i := 0; i < len(hr); i++ {
		hr[i] = 0
	}
}

func (p *HVPacket) PacketType() uint8 {
	return p.head.getType()
}

func (p *HVPacket) ReadFrom(reader io.Reader) (int64, error) {
	var err error
	if _, err = io.ReadFull(reader, p.head); err != nil {
		return 0, err
	}

	bodylen := p.head.getBodyLen()

	if bodylen > 0 {
		p.body = make([]uint8, bodylen)
		_, err = io.ReadFull(reader, p.body)
		if err != nil {
			return 0, err
		}
	}
	return int64(hvPackMetaLen + int(bodylen)), nil
}

func (p *HVPacket) WriteTo(writer io.Writer) (int64, error) {
	_, err := writer.Write(p.head)
	if err != nil {
		return 0, err
	}

	if len(p.body) > 0 {
		_, err = writer.Write(p.body)
		if err != nil {
			return 0, err
		}
	}
	return int64(hvPackMetaLen + len(p.body)), nil
}

func (p *HVPacket) SetType(h uint8) {
	p.head.setType(h)
}

func (p *HVPacket) GetType() uint8 {
	return p.head.getType()
}

func (p *HVPacket) SetBody(b []uint8) {
	p.body = b
	p.head.setBodyLen(uint32(len(b)))
}

func (p *HVPacket) GetBody() []uint8 {
	return p.body
}

func (p *HVPacket) Reset() {
	p.head.reset()
	p.body = nil
}
