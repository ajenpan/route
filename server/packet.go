package server

import (
	"errors"
	"io"
)

type Packet interface {
	PacketType() uint8
	io.ReaderFrom
	io.WriterTo
}

var packetMap = make(map[uint8]func() Packet)

func NewPacket(typ uint8) Packet {
	if f, ok := packetMap[typ]; ok {
		return f()
	}
	return nil
}

func RegPacket(typ uint8, f func() Packet) error {
	if _, ok := packetMap[typ]; ok {
		return errors.New("packet type already registered")
	}
	packetMap[typ] = f
	return nil
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
