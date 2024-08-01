package client

import (
	"context"
	"time"

	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/model"
)

type AckFunc = func(any, *errors.ConferenceError)
type CustomAckFunc = func(string, *errors.ConferenceError)
type MsgCb = func(ack AckFunc, arg any) (remained bool)
type CustomMsgCb = func(content string, ack CustomAckFunc, room string, from string, to string) (remained bool)
type RoomedCustomMsgCb = func(content string, ack CustomAckFunc, from string, to string) (remained bool)
type CloseCb = func (err error)
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
	Id() string
	MakesureConnect(ctx context.Context, socket_id string) error
	sendMsg(ctx context.Context, ack bool, evt string, arg any) (res any, err error)
	SendMessage(ctx context.Context, ack bool, evt string, content string, to string, room string) (res string, err error)
	onMsg(evt string, cb MsgCb) error
	OnMessage(evt string, cb CustomMsgCb)
	SetCloseCb(cb CloseCb)
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
	SendMessage(ctx context.Context, ack bool, evt string, content string, to string) (res string, err error)
	OnMessage(evt string, cb RoomedCustomMsgCb)
	OnParticipantJoin(cb ParticipantCb) int
	OffParticipantJoin(id int)
	OnParticipantLeave(cb ParticipantCb) int
	OffParticipantLeave(id int)
	WaitParticipant(ctx context.Context, uid string) *model.Participant
	KeepAlive(ctx context.Context, uid string, mode KeepAliveMode, timeout time.Duration, errCb KeepAliveCb) (stopFun func(), err error)
}
