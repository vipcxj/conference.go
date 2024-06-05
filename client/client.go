package client

import (
	"context"
	"sync"
	"time"

	"github.com/lesismal/nbio/nbhttp"
	"github.com/mitchellh/mapstructure"
	"github.com/vipcxj/conference.go/model"
)

type ParticipantCb = func(participant *model.Participant)

type Client interface {
	OnParticipantJoin(cb ParticipantCb)
	OnParticipantLeave(cb ParticipantCb)
	OnCustomMsg(evt string, cb CustomMsgCb)
	SendCustomMsg(timeout time.Duration, ack bool, evt string, content string, to string)
}

type WebsocketClient struct {
	signal               Signal
	participant_mux      sync.Mutex
	participants         map[string]model.Participant
	participant_join_cb  ParticipantCb
	participant_leave_cb ParticipantCb
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
		_, exist := client.participants[msg.UserId]
		participant := model.Participant{
			Id:   msg.UserId,
			Name: msg.UserName,
		}
		client.participants[msg.UserId] = participant
		if !exist && client.participant_join_cb != nil {
			client.participant_join_cb(&participant)
		}
	})
	return client
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

func (c *WebsocketClient) SendCustomMsg(timeout time.Duration, ack bool, evt string, content string, to string) {
	c.signal.SendCustomMsg(timeout, ack, evt, content, to)
}
