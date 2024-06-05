package client

import "time"

type AckFunc = func(any, error)

type Signal interface {
	SendMsg(timeout time.Duration, ack bool, evt string, arg any) (res any, err error)
	SendCustomMsg(timeout time.Duration, ack bool, evt string, content string, to string) error
	On(evt string, cb func(ack AckFunc, arg any)) error
	OnCustom(evt string, cb CustomMsgCb)
}
