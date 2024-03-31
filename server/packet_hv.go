package server

import "io"

type hvPacketFlag = uint8

const (
	// inner 224
	HVPacketTypeInnerStartAt_ hvPacketFlag = 0xE0
	hvPacketFlagHandShake     hvPacketFlag = HVPacketTypeInnerStartAt_ + iota
	hvPacketFlagActionRequire hvPacketFlag = HVPacketTypeInnerStartAt_ + iota
	hvPacketFlagDoAction      hvPacketFlag = HVPacketTypeInnerStartAt_ + iota
	hvPacketFlagAckResult     hvPacketFlag = HVPacketTypeInnerStartAt_ + iota
	HVPacketFlagHeartbeat     hvPacketFlag = HVPacketTypeInnerStartAt_ + iota
	HVPacketFlagEcho          hvPacketFlag = HVPacketTypeInnerStartAt_ + iota
	HVPacketFlagMessage       hvPacketFlag = HVPacketTypeInnerStartAt_ + iota
	HVPcketTypeInnerEndAt_    hvPacketFlag = HVPacketTypeInnerStartAt_ + iota
)

// const hvPacketMaxBodySize = 0x7FFFFF

const hvPackMetaLen = 4

type hvHead []byte

type HVPacket struct {
	head hvHead
	body []byte
}

func newHead() hvHead {
	return make([]byte, hvPackMetaLen)
}

func NewHVPacket() *HVPacket {
	return &HVPacket{
		head: newHead(),
	}
}

func (hr hvHead) getFlag() byte {
	return hr[0]
}

func (hr hvHead) setFlag(l byte) {
	hr[0] = l
}

func (h hvHead) getBodyLen() uint32 {
	return GetUint24(h[1:4])
}

func (hr hvHead) setBodyLen(l uint32) {
	PutUint24(hr[1:4], l)
}

func (hr hvHead) reset() {
	for i := 0; i < len(hr); i++ {
		hr[i] = 0
	}
}

func (p *HVPacket) PacketType() byte {
	return HVPacketType
}

func (p *HVPacket) ReadFrom(reader io.Reader) (int64, error) {
	var err error
	if _, err = io.ReadFull(reader, p.head); err != nil {
		return 0, err
	}

	bodylen := p.head.getBodyLen()

	if bodylen > 0 {
		p.body = make([]byte, bodylen)
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

func (p *HVPacket) SetFlag(h byte) {
	p.head.setFlag(h)
}

func (p *HVPacket) GetFlag() byte {
	return p.head.getFlag()
}

func (p *HVPacket) SetBody(b []byte) {
	p.body = b
	p.head.setBodyLen(uint32(len(b)))
}

func (p *HVPacket) GetBody() []byte {
	return p.body
}

func (p *HVPacket) Reset() {
	p.head.reset()
	p.body = nil
}
