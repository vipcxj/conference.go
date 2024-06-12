package client

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lesismal/nbio/nbhttp"
	"github.com/mitchellh/mapstructure"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/log"
	"github.com/vipcxj/conference.go/model"
	"github.com/vipcxj/conference.go/utils"
	"go.uber.org/zap"
)

type ParticipantCb = func(participant *model.Participant)
type KeepAliveCb = func(err error) (stop bool)
type Client interface {
	OnParticipantJoin(cb ParticipantCb)
	OnParticipantLeave(cb ParticipantCb)
	OnCustomMsg(evt string, cb CustomMsgCb)
	SendCustomMsg(ack bool, evt string, content string, to string, room string)
	IsInRoom(room string) bool
	Join(rooms ...string) error
	Leave(rooms ...string) error
	KeepAlive(room string, uid string, timeout time.Duration, errCb KeepAliveCb) (stopFun func())
}

type WebsocketClientConfigure struct {
	WebSocketSignalConfigure
	Timeout time.Duration
}

type WebsocketClient struct {
	ctx                  context.Context
	cancel               context.CancelCauseFunc
	signal               Signal
	participant_mux      sync.Mutex
	participants         map[string]model.Participant
	participant_join_cb  ParticipantCb
	participant_leave_cb ParticipantCb
	timeout              time.Duration
	user_info            *model.UserInfo
	user_info_mux        sync.Mutex
	msg_id               atomic.Uint32
	logger               *zap.Logger
	sugar               *zap.SugaredLogger
}

func NewWebsocketClient(ctx context.Context, conf *WebsocketClientConfigure, engine *nbhttp.Engine) Client {
	ctx, cancel := context.WithCancelCause(ctx)
	signal := NewWebsocketSignal(ctx, &conf.WebSocketSignalConfigure, engine)
	logger := log.Logger().With(zap.String("tag", "client"))
	client := &WebsocketClient{
		ctx:          ctx,
		cancel:       cancel,
		signal:       signal,
		participants: make(map[string]model.Participant),
		timeout:      conf.Timeout,
		logger:       logger,
		sugar:       logger.Sugar(),
	}
	signal.On("participant", func(ack AckFunc, arg any) (remained bool) {
		remained = true
		msg := model.StateParticipantMessage{}
		err := mapstructure.Decode(arg, &msg)
		if err != nil {
			client.sugar.Errorf("unable to decode participant msg, %w", err)
			return
		}
		client.participant_mux.Lock()
		defer client.participant_mux.Unlock()
		_, exist := client.participants[msg.UserId]
		participant := model.Participant{
			Id:   msg.UserId,
			Name: msg.UserName,
		}
		client.participants[msg.UserId] = participant
		if !exist && client.participant_join_cb != nil {
			client.participant_join_cb(&participant)
		}
		return
	})
	signal.On("ping", func(ack AckFunc, arg any) (remained bool) {
		remained = true
		msg := model.PingMessage{}
		err := mapstructure.Decode(arg, &msg)
		if err != nil {
			client.sugar.Errorf("unable to decode ping msg, %w", err)
			return
		}
		go func ()  {
			router := msg.GetRouter()
			_, err := signal.SendMsg(client.timeout, false, "pong", &model.PongMessage{
				Router: &model.RouterMessage{
					Room: router.GetRoom(),
					UserTo: router.GetUserFrom(),
				},
				MsgId: msg.MsgId,
			})
			if err != nil {
				client.sugar.Errorf("unable to send pong msg to user %s in room %s, %w", router.GetUserFrom(), router.GetRoom(), err)
			}
		}()
		return
	})
	return client
}

func (c *WebsocketClient) userInfo() *model.UserInfo {
	if c.user_info == nil {
		res, err := c.signal.SendMsg(c.timeout, true, "user-info", nil)
		if err != nil {
			c.sugar.DPanicf("unable to send user-info msg, %w", err)
		}
		var info model.UserInfo
		err = mapstructure.Decode(res, &info)
		if err != nil {
			c.sugar.DPanicf("unable to decode the user-info msg, %w", err)
		}
		c.user_info = &info
	}
	return c.user_info
}

func (c *WebsocketClient) UserInfo() *model.UserInfo {
	c.user_info_mux.Lock()
	defer c.user_info_mux.Unlock()
	return c.userInfo()
}

func (c *WebsocketClient) IsInRoom(room string) bool {
	c.user_info_mux.Lock()
	defer c.user_info_mux.Unlock()
	return slices.Contains(c.userInfo().Rooms, room)
}

func (c *WebsocketClient) OnParticipantJoin(cb ParticipantCb) {
	c.participant_mux.Lock()
	defer c.participant_mux.Unlock()
	c.participant_join_cb = cb
}

func (c *WebsocketClient) OnParticipantLeave(cb ParticipantCb) {
	c.participant_mux.Lock()
	defer c.participant_mux.Unlock()
	c.participant_leave_cb = cb
}

func (c *WebsocketClient) OnCustomMsg(evt string, cb CustomMsgCb) {
	c.signal.OnCustom(evt, cb)
}

func (c *WebsocketClient) SendCustomMsg(ack bool, evt string, content string, to string, room string) {
	c.signal.SendCustomMsg(c.timeout, ack, evt, content, to, room)
}

func (c *WebsocketClient) Join(rooms ...string) error {
	err := c.signal.Join(c.timeout, rooms...)
	if err != nil {
		return err
	}
	c.user_info_mux.Lock()
	defer c.user_info_mux.Unlock()
	c.userInfo().Rooms = utils.SliceAppendNoRepeat(c.userInfo().Rooms, rooms...)
	return nil
}

func (c *WebsocketClient) Leave(rooms ...string) error {
	err := c.signal.Join(c.timeout, rooms...)
	if err != nil {
		return err
	}
	c.user_info_mux.Lock()
	defer c.user_info_mux.Unlock()
	c.userInfo().Rooms = utils.SliceRemoveByValues(c.userInfo().Rooms, false, rooms...)
	return nil
}

func (c *WebsocketClient) KeepAlive(room string, uid string, timeout time.Duration, errCb KeepAliveCb) (stopFun func()) {
	ctx, cancel := context.WithCancel(c.ctx)
	go func() {
		for {
			msg_id := c.msg_id.Add(1)
			ch := make(chan any)
			c.signal.On("pong", func(ack AckFunc, arg any) (remained bool) {
				msg := model.PongMessage{}
				err := mapstructure.Decode(arg, &msg)
				if err != nil {
					if errCb(err) {
						cancel()
					} else {
						close(ch)
					}
					return false
				}
				if msg.MsgId == msg_id && msg.GetRouter().GetUserFrom() == uid && msg.GetRouter().GetRoom() == room {
					close(ch)
					return false	
				} else {
					return true
				}
			})
			_, err := c.signal.SendMsg(c.timeout, false, "ping", &model.PingMessage{
				Router: &model.RouterMessage{
					Room:   room,
					UserTo: uid,
				},
				MsgId: msg_id,
			})
			if err != nil {
				if errCb(err) {
					return
				}
			} else {
				select {
				case <-ctx.Done():
					return
				case <-time.After(c.timeout):
					if errCb(errors.MsgTimeout("pong msg timeout")) {
						return
					}
				case <-ch:
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(timeout):
			}
		}
	}()
	return cancel
}
