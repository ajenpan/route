package tcp

type PacketType = uint8

const (
	// inner 224
	PacketTypeInnerStartAt_ PacketType = 0xE0
	// ack
	PacketTypeHandShake     PacketType = PacketTypeInnerStartAt_ + iota
	PacketTypeActionRequire PacketType = PacketTypeInnerStartAt_ + iota
	PacketTypeDoAction      PacketType = PacketTypeInnerStartAt_ + iota
	PacketTypeAckResult     PacketType = PacketTypeInnerStartAt_ + iota

	PacketTypeHeartbeat   PacketType = PacketTypeInnerStartAt_ + iota
	PacketTypeEcho        PacketType = PacketTypeInnerStartAt_ + iota
	PacketTypeMessage     PacketType = PacketTypeInnerStartAt_ + iota
	PacketTypeInnerEndAt_ PacketType = PacketTypeInnerStartAt_ + iota
)

var MaxPacketBodySize = 8096

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

const PackMetaLen = 4

type Head []uint8

func newHead() Head {
	return make([]uint8, PackMetaLen)
}

func (hr Head) GetType() uint8 {
	return hr[0]
}

func (hr Head) SetType(l uint8) {
	hr[0] = l
}

func (h Head) GetBodyLen() uint16 {
	return GetUint16(h[2:3])
}

func (hr Head) SetBodyLen(l uint16) {
	PutUint16(hr[2:3], l)
}

func (hr Head) Reset() {
	for i := 0; i < len(hr); i++ {
		hr[i] = 0
	}
}

func (hr Head) Valid() bool {
	if hr.GetType() <= PacketTypeInnerStartAt_ || hr.GetType() >= PacketTypeInnerEndAt_ {
		return false
	}
	if hr.GetBodyLen() > uint16(MaxPacketBodySize) {
		return false
	}
	return true
}
