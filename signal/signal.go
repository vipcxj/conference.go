package signal

import (
	"context"
	"sync"

	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/model"
	"github.com/vipcxj/conference.go/utils"
	"go.uber.org/zap"
)

type AckFunc = func([]any, *errors.ConferenceError)
type MsgCb = func(ack AckFunc, args ...any) (remained bool)
type CustomMsgCb = func(evt string, msg *model.CustomMessage)
type CustomAckMsgCb = func(msg *model.CustomAckMessage)

type Signal interface {
	SendMsg(ctx context.Context, ack bool, evt string, args ...any) (res []any, err error)
	On(evt string, cb MsgCb) error
	OnCustom(cb CustomMsgCb) error
	OnCustomAck(cb CustomAckMsgCb) error
	OnClose(cb func(args ...any))
	GetContext() *SignalContext
	Sugar() *zap.SugaredLogger
	Close()
}

type MsgCbs struct {
	cbs      map[string][]MsgCb
	mux      sync.Mutex
	panic_cb func(err any, evt string, args ...any) (recover bool)
}

func NewMsgCbs() *MsgCbs {
	return &MsgCbs{
		cbs: make(map[string][]MsgCb),
	}
}

func (mc *MsgCbs) invokeMsgCb(cb MsgCb, evt string, ack AckFunc, args ...any) (remained bool) {
	defer func() {
		if mc.panic_cb != nil {
			if err := recover(); err != nil {
				if !mc.panic_cb(err, evt, args...) {
					panic(err)
				}
			}
		}
	}()
	return cb(ack, args...)
}

func (mc *MsgCbs) Run(evt string, ack AckFunc, args ...any) {
	mc.mux.Lock()
	defer mc.mux.Unlock()
	cbs, ok := mc.cbs[evt]
	if ok {
		cbs = utils.SliceMapChange(cbs, func(cb MsgCb) (mapped MsgCb, remove bool) {
			if mc.invokeMsgCb(cb, evt, ack, args...) {
				return cb, false
			} else {
				return nil, true
			}
		})
		if len(cbs) == 0 {
			delete(mc.cbs, evt)
		} else {
			mc.cbs[evt] = cbs
		}
	}
}

func (mc *MsgCbs) AddCallback(evt string, cb MsgCb) {
	mc.mux.Lock()
	defer mc.mux.Unlock()
	cbs, ok := mc.cbs[evt]
	if ok {
		mc.cbs[evt] = append(cbs, cb)
	} else {
		mc.cbs[evt] = []MsgCb{cb}
	}
}