package server

import (
	"errors"
	"io"
)

const (
	RoutePacketType byte = 1 // route
	HVPacketType    byte = 0x10
)

func init() {
	RegPacket(NewHVPacket)
	RegPacket(NewRoutePacket)
}

type Packet interface {
	PacketType() byte
	io.ReaderFrom
	io.WriterTo
}

var packetMap = make(map[byte]func() Packet)

func NewPacket(typ byte) Packet {
	if f, ok := packetMap[typ]; ok {
		return f()
	}
	return nil
}

func RegPacket[T Packet](f func() T) error {
	temp := f()
	typ := temp.PacketType()
	if _, ok := packetMap[typ]; ok {
		panic(errors.New("packet type already registered"))
	}
	packetMap[typ] = func() Packet {
		return f()
	}
	return nil
}

func GetUint24(b []byte) uint32 {
	_ = b[2] // bounds check hint to compiler; see golang.org/issue/14808
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16
}

func PutUint24(b []byte, v uint32) {
	_ = b[2] // early bounds check to guarantee safety of writes below
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
}
