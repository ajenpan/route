package server

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type User interface {
	UserID() uint32
}

type UserData interface {
	SetUserData(key any, value any)
	GetUserData(key any) (vluae any, has bool)
}

type Session interface {
	User
	UserData
	SessionID() string
	SessionType() string

	IsValid() bool
	Close() error
	Send(Packet) error

	RemoteAddr() net.Addr
}

type FuncOnSessionPacket func(Session, Packet)
type FuncOnSessionStatus func(s Session, enable bool)
type FuncOnAccpect func(net.Conn) bool

var sid int64 = 0

func NewSessionID() string {
	return fmt.Sprintf("%d_%d", atomic.AddInt64(&sid, 1), time.Now().Unix())
}

type SessionStatus = int32

const (
	Disconnected SessionStatus = iota
	Connectting  SessionStatus = iota
	Handshake    SessionStatus = iota
	Connected    SessionStatus = iota
)

type userData struct {
	sync.Map
}

func (ud *userData) SetUserData(k any, v any) {
	ud.Map.Store(k, v)
}

func (ud *userData) GetUserData(k any) (any, bool) {
	return ud.Map.Load(k)
}
