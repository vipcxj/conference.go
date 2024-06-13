package client

import (
	"context"
	"sync"
	"time"

	"github.com/lesismal/nbio/nbhttp"
	"github.com/mitchellh/mapstructure"
	"github.com/vipcxj/conference.go/log"
	"github.com/vipcxj/conference.go/model"
	"go.uber.org/zap"
)

type ParticipantCb = func(participant *model.Participant)

type Client interface {
	OnParticipantJoin(cb ParticipantCb)
	OnParticipantLeave(cb ParticipantCb)
	OnMessage(evt string, cb CustomMsgCb)
	SendMessage(ack bool, evt string, content string, to string, room string) error
	Join(rooms ...string) error
	Leave(rooms ...string) error
	UserInfo() (*model.UserInfo, error)
	IsInRoom(room string) (bool, error)
	KeepAlive(room string, uid string, mode KeepAliveMode, timeout time.Duration, errCb KeepAliveCb) (stopFun func(), err error)
	Roomed(room string) (RoomedClient, error)
}

type RoomedClient interface {
	OnMessage(evt string, cb RoomedCustomMsgCb)
	SendMessage(ack bool, evt string, content string, to string) error
	KeepAlive(uid string, mode KeepAliveMode, timeout time.Duration, errCb KeepAliveCb) (stopFun func(), err error)
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
	logger               *zap.Logger
	sugar                *zap.SugaredLogger
}

type RoomedWebsocketClient struct {
	client *WebsocketClient
	signal RoomedSignal
}

func (c *RoomedWebsocketClient) OnMessage(evt string, cb RoomedCustomMsgCb) {
	c.signal.OnMessage(evt, cb)
}

func (c *RoomedWebsocketClient) SendMessage(ack bool, evt string, content string, to string) error {
	return c.signal.SendMessage(c.client.timeout, ack, evt, content, to)
}

func (c *RoomedWebsocketClient) KeepAlive(uid string, mode KeepAliveMode, timeout time.Duration, errCb KeepAliveCb) (stopFun func(), err error) {
	return c.signal.KeepAlive(uid, mode, timeout, errCb)
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
		logger:       logger,
		sugar:        logger.Sugar(),
	}
	signal.onMsg("participant", func(ack AckFunc, arg any) (remained bool) {
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
	return client
}

func (c *WebsocketClient) IsInRoom(room string) (bool, error) {
	res, err := c.signal.IsInRoom(c.timeout, room)
	if err != nil {
		return false, err
	}
	return res, nil
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

func (c *WebsocketClient) OnMessage(evt string, cb CustomMsgCb) {
	c.signal.OnMessage(evt, cb)
}

func (c *WebsocketClient) SendMessage(ack bool, evt string, content string, to string, room string) error {
	return c.signal.SendMessage(c.timeout, ack, evt, content, to, room)
}

func (c *WebsocketClient) Join(rooms ...string) error {
	return c.signal.Join(c.timeout, rooms...)
}

func (c *WebsocketClient) Leave(rooms ...string) error {
	return c.signal.Leave(c.timeout, rooms...)
}

func (c *WebsocketClient) UserInfo() (*model.UserInfo, error) {
	return c.signal.UserInfo(c.timeout)
}

func (c *WebsocketClient) KeepAlive(room string, uid string, mode KeepAliveMode, timeout time.Duration, errCb KeepAliveCb) (stopFun func(), err error) {
	return c.signal.KeepAlive(room, uid, mode, timeout, errCb)
}

func (c *WebsocketClient)  Roomed(room string) (RoomedClient, error) {
	rs, err := c.signal.Roomed(c.timeout, room);
	if err != nil {
		return nil, err
	}
	return &RoomedWebsocketClient{
		signal: rs,
		client: c,
	}, nil
}