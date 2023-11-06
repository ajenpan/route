package auth

import (
	"fmt"
	"sync/atomic"
)

type FakeAuth struct {
	c uint64
}

func (a *FakeAuth) TokenAuth(token string) (*UserInfo, error) {
	id := a.nextID()
	name := fmt.Sprintf("user-%d", id)
	role := "user"

	return &UserInfo{
		ID:   id,
		Name: name,
		Role: role,
	}, nil
}

func (a *FakeAuth) nextID() uint64 {
	ret := atomic.AddUint64(&a.c, 1)
	if ret == 0 {
		ret = atomic.AddUint64(&a.c, 1)
	}
	return ret
}
