package calltable

import (
	"context"
	"reflect"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func ExtractParseGRpcMethod(ms protoreflect.ServiceDescriptors, h interface{}) *CallTable[string] {
	refh := reflect.TypeOf(h)

	ret := NewCallTable[string]()

	ctxType := reflect.TypeOf((*context.Context)(nil)).Elem()
	pbMsgType := reflect.TypeOf((*proto.Message)(nil)).Elem()
	errType := reflect.TypeOf((*error)(nil)).Elem()

	for i := 0; i < ms.Len(); i++ {
		service := ms.Get(i)
		methods := service.Methods()
		svrName := string(service.Name())

		for j := 0; j < methods.Len(); j++ {
			rpcMethod := methods.Get(j)
			rpcMethodName := string(rpcMethod.Name())

			method, has := refh.MethodByName(rpcMethodName)
			if !has {
				continue
			}
			if method.Type.NumIn() != 3 || method.Type.NumOut() != 2 {
				continue
			}
			if method.Type.In(1) != ctxType {
				continue
			}
			if !method.Type.In(2).Implements(pbMsgType) {
				continue
			}
			if !method.Type.Out(0).Implements(pbMsgType) {
				continue
			}
			if method.Type.Out(1) != errType {
				continue
			}
			epn := strings.Join([]string{svrName, rpcMethodName}, EndpointSplit)
			reqType := method.Type.In(2).Elem()
			respType := method.Type.Out(0).Elem()

			m := &Method{
				Imp:          method,
				Style:        StyleSync,
				RequestType:  reqType,
				ResponseType: respType,
			}
			m.InitPool()

			ret.list[epn] = m
		}
	}
	return ret
}

func ExtractAsyncMethod(ms protoreflect.MessageDescriptors, h interface{}) *CallTable[string] {
	const MethodPrefix string = "On"
	refh := reflect.TypeOf(h)

	ret := NewCallTable[string]()
	pbMsgType := reflect.TypeOf((*proto.Message)(nil)).Elem()

	for i := 0; i < ms.Len(); i++ {
		msg := ms.Get(i)
		msgName := string(msg.Name())
		fullName := string(msg.FullName())
		method, has := refh.MethodByName(MethodPrefix + msgName)
		if !has {
			continue
		}

		if method.Type.NumIn() != 3 {
			continue
		}

		reqMsgType := method.Type.In(2)
		if reqMsgType.Kind() != reflect.Ptr {
			continue
		}
		if !reqMsgType.Implements(pbMsgType) {
			continue
		}

		m := &Method{
			Imp:         method,
			Style:       StyleAsync,
			RequestType: reqMsgType.Elem(),
		}
		m.InitPool()
		ret.list[fullName] = m
	}
	return ret
}

func ExtractProtoFile(fd protoreflect.FileDescriptor, handler interface{}) *CallTable[string] {
	ret := NewCallTable[string]()

	rpcTable := ExtractParseGRpcMethod(fd.Services(), handler)
	asyncTalbe := ExtractAsyncMethod(fd.Messages(), handler)

	ret.Merge(rpcTable, false)
	ret.Merge(asyncTalbe, false)

	return ret
}

func GetMessageMsgID(msg protoreflect.MessageDescriptor) int {
	MSGIDDesc := msg.Enums().ByName("MSGID")
	if MSGIDDesc == nil {
		return 0
	}
	IDDesc := MSGIDDesc.Values().ByName("ID")
	if IDDesc == nil {
		return 0
	}
	return int(IDDesc.Number())
}

func ExtractAsyncMethodByMsgID(ms protoreflect.MessageDescriptors, h interface{}) *CallTable[int] {
	const MethodPrefix string = "On"
	refh := reflect.TypeOf(h)

	ret := NewCallTable[int]()
	pbMsgType := reflect.TypeOf((*proto.Message)(nil)).Elem()

	for i := 0; i < ms.Len(); i++ {
		msg := ms.Get(i)
		msgid := GetMessageMsgID(msg)
		if msgid == 0 {
			continue
		}
		msgName := string(msg.Name())
		method, has := refh.MethodByName(MethodPrefix + msgName)
		if !has {
			continue
		}

		if method.Type.NumIn() != 3 {
			continue
		}

		reqMsgType := method.Type.In(2)
		if reqMsgType.Kind() != reflect.Ptr {
			continue
		}

		if !reqMsgType.Implements(pbMsgType) {
			continue
		}

		m := &Method{
			Imp:         method,
			Style:       StyleAsync,
			RequestType: reqMsgType.Elem(),
		}
		m.InitPool()
		ret.Add(msgid, m)
	}
	return ret
}
