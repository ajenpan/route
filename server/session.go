package server

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"
)

type User interface {
	UserID() uint32
	UserRole() string
}

type Session interface {
	User

	SessionID() string
	SessionType() string

	IsValid() bool
	Close() error
	Send(Packet) error

	RemoteAddr() net.Addr
}

type FuncOnSessionPacket func(Session, Packet)
type FuncOnSessionConn func(Session)
type FuncOnSessionDisconn func(Session, error)
type FuncOnAccpect func(net.Conn) bool

var sid int64 = 0

func NewSessionID() string {
	return fmt.Sprintf("%d_%d", atomic.AddInt64(&sid, 1), time.Now().Unix())
}

type SessionStatus int32

const (
	Disconnected SessionStatus = iota
	Connectting  SessionStatus = iota
	Handshake    SessionStatus = iota
	Connected    SessionStatus = iota
)
