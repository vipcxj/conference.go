package client

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lesismal/nbio/nbhttp"
	"github.com/mitchellh/mapstructure"
	"github.com/vipcxj/conference.go/log"
	"github.com/vipcxj/conference.go/model"
	"github.com/vipcxj/conference.go/utils"
	"go.uber.org/zap"
)

type KeepAliveContext struct {
	Err             error
	TimeoutNum      int
	TimeoutDuration time.Duration
}

type ParticipantCb = func(participant *model.Participant)
type KeepAliveCb = func(ctx *KeepAliveContext) (stop bool)
type KeepAliveMode int

const (
	KEEP_ALIVE_MODE_ACTIVE  KeepAliveMode = 1
	KEEP_ALIVE_MODE_PASSIVE KeepAliveMode = 2
)

type Client interface {
	OnParticipantJoin(cb ParticipantCb)
	OnParticipantLeave(cb ParticipantCb)
	OnCustomMsg(evt string, cb CustomMsgCb)
	SendCustomMsg(ack bool, evt string, content string, to string, room string)
	IsInRoom(room string) bool
	Join(rooms ...string) error
	Leave(rooms ...string) error
	KeepAlive(room string, uid string, mode KeepAliveMode, timeout time.Duration, errCb KeepAliveCb) (stopFun func(), err error)
}

type WebsocketClientConfigure struct {
	WebSocketSignalConfigure
	Timeout time.Duration
}

type PingKey struct {
	Uid  string
	Room string
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
	ping_chs             map[PingKey][]chan *model.PingMessage
	ping_chs_mux         sync.Mutex
	logger               *zap.Logger
	sugar                *zap.SugaredLogger
}

func NewWebsocketClient(ctx context.Context, conf *WebsocketClientConfigure, engine *nbhttp.Engine) Client {
	ctx, cancel := context.WithCancelCause(ctx)
	signal := NewWebsocketSignal(ctx, &conf.WebSocketSignalConfigure, engine)
	logger := log.Logger().With(zap.String("tag", "client"))
	var timeout time.Duration
	if conf.Timeout == 0 {
		timeout = time.Second
	}
	client := &WebsocketClient{
		ctx:          ctx,
		cancel:       cancel,
		signal:       signal,
		participants: make(map[string]model.Participant),
		timeout:      timeout,
		ping_chs:     make(map[PingKey][]chan *model.PingMessage),
		logger:       logger,
		sugar:        logger.Sugar(),
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
		msg := &model.PingMessage{}
		err := mapstructure.Decode(arg, msg)
		if err != nil {
			client.sugar.Errorf("unable to decode ping msg, %w", err)
			return
		}
		go func() {
			client.publishPingMsgs(msg)
			router := msg.GetRouter()
			_, err := signal.SendMsg(client.timeout, false, "pong", &model.PongMessage{
				Router: &model.RouterMessage{
					Room:   router.GetRoom(),
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

func (c *WebsocketClient) publishPingMsgs(msg *model.PingMessage) {
	router := msg.GetRouter()
	key := PingKey{
		Uid:  router.GetUserFrom(),
		Room: router.GetRoom(),
	}
	c.ping_chs_mux.Lock()
	defer c.ping_chs_mux.Unlock()
	chs, ok := c.ping_chs[key]
	if ok {
		for _, ch := range chs {
			ch <- msg
		}
	}
}

func (c *WebsocketClient) subscribePingMsg(room string, uid string) chan *model.PingMessage {
	key := PingKey{
		Uid:  uid,
		Room: room,
	}
	c.ping_chs_mux.Lock()
	defer c.ping_chs_mux.Unlock()
	ch := make(chan *model.PingMessage, 1)
	chs, ok := c.ping_chs[key]
	if ok {
		c.ping_chs[key] = append(chs, ch)
	} else {
		c.ping_chs[key] = []chan *model.PingMessage{ch}
	}
	return ch
}

func (c *WebsocketClient) unsubscribePingMsg(room string, uid string, ch chan *model.PingMessage) {
	key := PingKey{
		Uid:  uid,
		Room: room,
	}
	c.ping_chs_mux.Lock()
	defer c.ping_chs_mux.Unlock()
	chs, ok := c.ping_chs[key]
	if ok {
		chs = utils.SliceRemoveByValue(chs, false, ch)
		if len(chs) > 0 {
			c.ping_chs[key] = chs
		} else {
			delete(c.ping_chs, key)
		}
	}
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

func (c *WebsocketClient) KeepAlive(room string, uid string, mode KeepAliveMode, timeout time.Duration, errCb KeepAliveCb) (stopFun func(), err error) {
	err = c.signal.MakesureConnect()
	if err != nil {
		return
	}
	ctx, cancel := context.WithCancel(c.ctx)
	go func() {
		kaCtx := &KeepAliveContext{}
		switch mode {
		case KEEP_ALIVE_MODE_ACTIVE:
			for {
				now := time.Now()
				msg_id := c.msg_id.Add(1)
				ch := make(chan any)
				errCh := make(chan error, 1)
				c.signal.On("pong", func(ack AckFunc, arg any) (remained bool) {
					msg := model.PongMessage{}
					err := mapstructure.Decode(arg, &msg)
					if err != nil {
						errCh <- err
						return false
					}
					if msg.MsgId == msg_id && msg.GetRouter().GetUserFrom() == uid && msg.GetRouter().GetRoom() == room {
						close(ch)
						return false
					} else {
						return true
					}
				})
				c.sugar.Debug("keepalive: send ping msg")
				_, err := c.signal.SendMsg(c.timeout, false, "ping", &model.PingMessage{
					Router: &model.RouterMessage{
						Room:   room,
						UserTo: uid,
					},
					MsgId: msg_id,
				})
				if err != nil {
					kaCtx.Err = fmt.Errorf("failed to send ping msg, %w", err)
					if errCb(kaCtx) {
						return
					} else {
						kaCtx.Err = nil
					}
				} else {
					select {
					case <-ctx.Done():
						return
					case <-time.After(c.timeout):
						kaCtx.TimeoutNum ++
						kaCtx.TimeoutDuration += time.Since(now)
						if errCb(kaCtx) {
							return
						}
					case err = <-errCh:
						kaCtx.Err = err
						if errCb(kaCtx) {
							return
						} else {
							kaCtx.Err = nil
						}
					case <-ch:
						kaCtx.TimeoutNum = 0
						kaCtx.TimeoutDuration = 0
					}
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(timeout):
				}
			}
		case KEEP_ALIVE_MODE_PASSIVE:
			ch := c.subscribePingMsg(room, uid)
			for {
				now := time.Now()
				select {
				case <-ctx.Done():
					c.unsubscribePingMsg(room, uid, ch)
					return
				case <-time.After(timeout):
					kaCtx.TimeoutNum ++
					kaCtx.TimeoutDuration += time.Since(now)
					if errCb(kaCtx) {
						c.unsubscribePingMsg(room, uid, ch)
						return
					}
				case <-ch:
					c.sugar.Debug("keepalive: receive pong msg")
					kaCtx.TimeoutNum = 0
					kaCtx.TimeoutDuration = 0
				}
			}
		default:
			c.sugar.Panicf("invalid keep alive mode: %v", mode)
		}
	}()
	return cancel, nil
}

