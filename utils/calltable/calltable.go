package calltable

import (
	"reflect"
	"sync"
)

type MethodStyle int

const (
	StyleSync  MethodStyle = iota
	StyleAsync MethodStyle = iota
)

type NewTypeFunc = func() interface{}

const EndpointSplit = "/"

type Method struct {
	Imp   reflect.Method
	Style MethodStyle

	RequestType  reflect.Type
	ResponseType reflect.Type

	reqPool  *sync.Pool
	respPool *sync.Pool
}

func (m *Method) InitPool() {
	if m.RequestType != nil {
		m.reqPool = &sync.Pool{New: m.NewRequest}
	}

	if m.ResponseType != nil {
		m.respPool = &sync.Pool{New: m.NewResponse}
	}
}

func (m *Method) Call(args ...interface{}) []reflect.Value {
	values := make([]reflect.Value, len(args))
	for i, v := range args {
		values[i] = reflect.ValueOf(v)
	}
	return m.Imp.Func.Call(values)
}

func (m *Method) NewRequest() interface{} {
	if m.RequestType == nil {
		return nil
	}
	return reflect.New(m.RequestType).Interface()
}

func (m *Method) NewResponse() interface{} {
	if m.ResponseType == nil {
		return nil
	}
	return reflect.New(m.ResponseType).Interface()
}

func (m *Method) GetRequest() interface{} {
	if m.reqPool == nil {
		return m.NewRequest()
	}
	return m.reqPool.Get()
}

func (m *Method) PutRequest(req interface{}) {
	if m.reqPool == nil {
		return
	}
	m.reqPool.Put(req)
}

func (m *Method) GetResponse() interface{} {
	if m.respPool == nil {
		return m.NewResponse()
	}
	return m.respPool.Get()
}

func (m *Method) PutResponse(resp interface{}) {
	if m.respPool == nil {
		return
	}
	m.respPool.Put(resp)
}

type CallTable[T comparable] struct {
	sync.RWMutex
	list map[T]*Method
}

func NewCallTable[T comparable]() *CallTable[T] {
	return &CallTable[T]{
		list: make(map[T]*Method),
	}
}

func (m *CallTable[T]) Len() int {
	m.RLock()
	defer m.RUnlock()
	return len(m.list)
}

func (m *CallTable[T]) Get(name T) *Method {
	m.RLock()
	defer m.RUnlock()
	return m.list[name]
}

func (m *CallTable[T]) Range(f func(key T, value *Method) bool) {
	m.Lock()
	defer m.Unlock()
	for k, v := range m.list {
		if !f(k, v) {
			return
		}
	}
}

func (m *CallTable[T]) Merge(other *CallTable[T], overWrite bool) int {
	ret := 0
	other.RWMutex.RLock()
	defer other.RWMutex.RUnlock()

	m.Lock()
	defer m.Unlock()

	for k, v := range other.list {
		_, has := m.list[k]
		if has && !overWrite {
			continue
		}
		m.list[k] = v
		ret++
	}
	return ret
}

func (m *CallTable[T]) Add(name T, method *Method) bool {
	m.Lock()
	defer m.Unlock()
	if _, has := m.list[name]; has {
		return false
	}
	m.list[name] = method
	return true
}
