package client

import (
	"time"

	"github.com/vipcxj/conference.go/model"
)

type AckFunc = func(any, error)
type MsgCb = func(ack AckFunc, arg any) (remained bool)
type CustomMsgCb = func(content string, ack func(), room string, from string, to string) (remained bool)
type RoomedCustomMsgCb = func(content string, ack func(), from string, to string) (remained bool)

type KeepAliveContext struct {
	Err             error
	TimeoutNum      int
	TimeoutDuration time.Duration
}

type KeepAliveCb = func(ctx *KeepAliveContext) (stop bool)
type KeepAliveMode int

const (
	KEEP_ALIVE_MODE_ACTIVE  KeepAliveMode = 1
	KEEP_ALIVE_MODE_PASSIVE KeepAliveMode = 2
)

type Signal interface {
	MakesureConnect() error
	sendMsg(timeout time.Duration, ack bool, evt string, arg any) (res any, err error)
	SendMessage(timeout time.Duration, ack bool, evt string, content string, to string, room string) error
	onMsg(evt string, cb MsgCb) error
	OnMessage(evt string, cb CustomMsgCb)
	Join(timeout time.Duration, rooms ...string) error
	Leave(timeout time.Duration, rooms ...string) error
	UserInfo(timeout time.Duration) (*model.UserInfo, error)
	IsInRoom(timeout time.Duration, room string) (bool, error)
	KeepAlive(room string, uid string, mode KeepAliveMode, timeout time.Duration, errCb KeepAliveCb) (stopFun func(), err error)
	Roomed(timeout time.Duration, room string) (RoomedSignal, error)
}

type RoomedSignal interface {
	GetRoom() string
	SendMessage(timeout time.Duration, ack bool, evt string, content string, to string) error
	OnMessage(evt string, cb RoomedCustomMsgCb)
	KeepAlive(uid string, mode KeepAliveMode, timeout time.Duration, errCb KeepAliveCb) (stopFun func(), err error)
}
