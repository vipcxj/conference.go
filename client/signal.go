package client

import (
	"context"
	"time"

	"github.com/vipcxj/conference.go/model"
)

type AckFunc = func(any, error)
type MsgCb = func(ack AckFunc, arg any) (remained bool)
type CustomMsgCb = func(content string, ack func(), room string, from string, to string) (remained bool)
type RoomedCustomMsgCb = func(content string, ack func(), from string, to string) (remained bool)
type ParticipantCb = func(participant *model.Participant) (remained bool)

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
	MakesureConnect(ctx context.Context) error
	sendMsg(ctx context.Context, ack bool, evt string, arg any) (res any, err error)
	SendMessage(ctx context.Context, ack bool, evt string, content string, to string, room string) error
	onMsg(evt string, cb MsgCb) error
	OnMessage(evt string, cb CustomMsgCb)
	Join(ctx context.Context, rooms ...string) error
	Leave(ctx context.Context, rooms ...string) error
	UserInfo(ctx context.Context) (*model.UserInfo, error)
	GetRooms(ctx context.Context) ([]string, error)
	IsInRoom(ctx context.Context, room string) (bool, error)
	KeepAlive(ctx context.Context, room string, uid string, mode KeepAliveMode, timeout time.Duration, errCb KeepAliveCb) (stopFun func(), err error)
	Roomed(ctx context.Context, room string) (RoomedSignal, error)
}

type RoomedSignal interface {
	GetRoom() string
	SendMessage(ctx context.Context, ack bool, evt string, content string, to string) error
	OnMessage(evt string, cb RoomedCustomMsgCb)
	OnParticipantJoin(cb ParticipantCb) int
	OffParticipantJoin(id int)
	OnParticipantLeave(cb ParticipantCb) int
	OffParticipantLeave(id int)
	GetParticipants() []model.Participant
	WaitParticipant(ctx context.Context, uid string) (success bool)
	KeepAlive(ctx context.Context, uid string, mode KeepAliveMode, timeout time.Duration, errCb KeepAliveCb) (stopFun func(), err error)
}
