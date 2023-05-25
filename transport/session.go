package transport

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type SessionStat int32

const (
	Connected    SessionStat = 1
	Disconnected SessionStat = 2
)

type Packet interface {
	Type() int
	Name() string
	Head() []byte
	Body() []byte
}

type OnMessageFunc func(Session, Packet)
type OnConnStatFunc func(Session, SessionStat)
type NewSessionIDFunc func() string

func (s SessionStat) String() string {
	switch s {
	case Connected:
		return "connected"
	case Disconnected:
		return "disconnected"
	}
	return "unknown"
}

var sid int64 = 0

func NewSessionID() string {
	return fmt.Sprintf("%d_%d", atomic.AddInt64(&sid, 1), time.Now().Unix())
}

type SessionMeta interface {
	MetaLoad(key string) (interface{}, bool)
	MetaStore(key string, value interface{})
	MetaDelete(key string)
}

type Session interface {
	ID() string
	String() string
	RemoteAddr() string
	LocalAddr() string
	Status() SessionStat
	Send(Packet) error
	Close()
	SessionMeta
}

type MapMeta struct {
	imp sync.Map
}

func (m *MapMeta) MetaLoad(key string) (interface{}, bool) {
	return m.imp.Load(key)
}
func (m *MapMeta) MetaStore(key string, value interface{}) {
	m.imp.Store(key, value)
}
func (m *MapMeta) MetaDelete(key string) {
	m.imp.Delete(key)
}
