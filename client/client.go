package client

import (
	"context"
	"sync"
	"time"

	"github.com/lesismal/nbio/nbhttp"
	"github.com/mitchellh/mapstructure"
	"github.com/vipcxj/conference.go/model"
)

type Client interface {
	OnCustomMsg(evt string, cb CustomMsgCb)
	SendCustomMsg(timeout time.Duration, ack bool, evt string, content string, to string)
}

type WebsocketClient struct {
	signal Signal
	participant_mux sync.Mutex
	participants map[string] model.Participant
	participant_cb func(participants map[string] model.Participant)
}

func NewWebsocketClient(ctx context.Context, conf *WebSocketSignalConfigure, engine *nbhttp.Engine) Client {
	signal := NewWebsocketSignal(ctx, conf, engine)
	client := &WebsocketClient{
		signal: signal,
	}
	signal.On("participant", func(ack AckFunc, arg any) {
		msg := model.StateParticipantMessage{}
		err := mapstructure.Decode(arg, &msg)
		if err != nil {
			panic(err)
		}
		client.participant_mux.Lock()
		defer client.participant_mux.Unlock()
		client.participants[msg.UserId] = model.Participant{
			Id: msg.UserId,
			Name: msg.UserName,
		}
		if client.participant_cb != nil {
			client.participant_cb(client.participants)
		}
	})
	return client
}

func (c *WebsocketClient) OnCustomMsg(evt string, cb CustomMsgCb) {
	c.signal.OnCustom(evt, cb)
}

func (c *WebsocketClient) SendCustomMsg(timeout time.Duration, ack bool, evt string, content string, to string) {
	c.signal.SendCustomMsg(timeout, ack, evt, content, to)
}

