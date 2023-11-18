package server

import "route/msg"

type ContentType byte

const (
	_                ContentType = iota // proto
	ProtoBinaryRoute ContentType = iota // route
)

type Head = msg.Head

type Message struct {
	ContentType ContentType
	Head        *Head
	Body        []byte
}
