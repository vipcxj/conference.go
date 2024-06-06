package client

import "time"

type AckFunc = func(any, error)
type MsgCb = func(ack AckFunc, arg any) (remained bool)
type CustomMsgCb = func(content string, ack func(), from string, to string) (remained bool)
type Signal interface {
	SendMsg(timeout time.Duration, ack bool, evt string, arg any) (res any, err error)
	SendCustomMsg(timeout time.Duration, ack bool, evt string, content string, to string) error
	On(evt string, cb MsgCb) error
	OnCustom(evt string, cb CustomMsgCb)
}
