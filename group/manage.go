package group

import (
	"route/transport/tcp"
	"sync"
)

type Manager struct {
	groups sync.Map
}

func (m *Manager) MustGetGroup(groupName string) *Group {
	v, _ := m.groups.LoadOrStore(groupName, &Group{})
	return v.(*Group)
}

func (m *Manager) GetGroup(groupName string) *Group {
	if v, has := m.groups.Load(groupName); has {
		return v.(*Group)
	}
	return nil
}

func (m *Manager) RemoveFrom(groupName string, uid uint32) {
	if v, has := m.groups.Load(groupName); has {
		v.(*Group).Remove(uid)
	}
}
func (m *Manager) AddTo(groupName string, uid uint32, s *tcp.Socket) {
	g := m.MustGetGroup(groupName)
	g.Add(uid, s)
}
