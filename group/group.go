package group

import (
	"sync"

	"route/transport/tcp"
)

type Group struct {
	imp  map[uint32]*tcp.Socket
	lock sync.RWMutex
}

func (g *Group) Add(uid uint32, s *tcp.Socket) {
	g.lock.Lock()
	defer g.lock.Unlock()
	g.imp[uid] = s
}

func (g *Group) Remove(uid uint32) {
	g.lock.Lock()
	defer g.lock.Unlock()
	delete(g.imp, uid)
}

func (g *Group) Get(uid uint32) *tcp.Socket {
	g.lock.RLock()
	defer g.lock.RUnlock()
	return g.imp[uid]
}

func (g *Group) GetAll() map[uint32]*tcp.Socket {
	g.lock.RLock()
	defer g.lock.RUnlock()
	return g.imp
}
