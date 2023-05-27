package sessiongroup

import (
	"sync"

	"route/transport/tcp"
)

type Manager struct {
	groups sync.Map
}

func (m *Manager) MustGetGroup(groupName string) *Group {
	v, _ := m.groups.LoadOrStore(groupName, NewGroup())
	return v.(*Group)
}

func (m *Manager) GetGroup(groupName string) *Group {
	if v, has := m.groups.Load(groupName); has {
		return v.(*Group)
	}
	return nil
}

func (m *Manager) RemoveFromGroup(groupName string, uid uint32, s *tcp.Socket) {
	if v, has := m.groups.Load(groupName); has {
		v.(*Group).RemoveIfSame(uid, s)
	}
}

func (m *Manager) AddTo(groupName string, uid uint32, s *tcp.Socket) {
	g := m.MustGetGroup(groupName)
	g.Add(uid, s)
}

func (m *Manager) Groups() []string {
	var groups []string
	m.groups.Range(func(key, value interface{}) bool {
		groups = append(groups, key.(string))
		return true
	})
	// todo:
	// sort.Slice(groups, func(i, j int) bool {
	// 	return groups[i] < groups[j]
	// })

	return groups
}
