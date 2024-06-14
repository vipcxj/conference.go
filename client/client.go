package client

import (
	"context"
	"time"

	"github.com/lesismal/nbio/nbhttp"
	"github.com/vipcxj/conference.go/log"
	"github.com/vipcxj/conference.go/model"
	"go.uber.org/zap"
)

type Client interface {
	OnMessage(evt string, cb CustomMsgCb)
	SendMessage(ctx context.Context, ack bool, evt string, content string, to string, room string) error
	Join(ctx context.Context, rooms ...string) error
	Leave(ctx context.Context, rooms ...string) error
	UserInfo(ctx context.Context) (*model.UserInfo, error)
	IsInRoom(ctx context.Context, room string) (bool, error)
	KeepAlive(ctx context.Context, room string, uid string, mode KeepAliveMode, timeout time.Duration, errCb KeepAliveCb) (stopFun func(), err error)
	Roomed(ctx context.Context, room string) (RoomedSignal, error)
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
	ctx     context.Context
	cancel  context.CancelCauseFunc
	signal  Signal
	timeout time.Duration
	logger  *zap.Logger
	sugar   *zap.SugaredLogger
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
		ctx:     ctx,
		cancel:  cancel,
		signal:  signal,
		timeout: timeout,
		logger:  logger,
		sugar:   logger.Sugar(),
	}
	return client
}

func (c *WebsocketClient) IsInRoom(ctx context.Context, room string) (bool, error) {
	res, err := c.signal.IsInRoom(ctx, room)
	if err != nil {
		return false, err
	}
	return res, nil
}

func (c *WebsocketClient) OnMessage(evt string, cb CustomMsgCb) {
	c.signal.OnMessage(evt, cb)
}

func (c *WebsocketClient) SendMessage(ctx context.Context, ack bool, evt string, content string, to string, room string) error {
	return c.signal.SendMessage(ctx, ack, evt, content, to, room)
}

func (c *WebsocketClient) Join(ctx context.Context, rooms ...string) error {
	return c.signal.Join(ctx, rooms...)
}

func (c *WebsocketClient) Leave(ctx context.Context, rooms ...string) error {
	return c.signal.Leave(ctx, rooms...)
}

func (c *WebsocketClient) UserInfo(ctx context.Context, ) (*model.UserInfo, error) {
	return c.signal.UserInfo(ctx)
}

func (c *WebsocketClient) KeepAlive(ctx context.Context, room string, uid string, mode KeepAliveMode, timeout time.Duration, errCb KeepAliveCb) (stopFun func(), err error) {
	return c.signal.KeepAlive(ctx, room, uid, mode, timeout, errCb)
}

func (c *WebsocketClient) Roomed(ctx context.Context, room string) (RoomedSignal, error) {
	rs, err := c.signal.Roomed(ctx, room)
	if err != nil {
		return nil, err
	}
	return rs, nil
}
