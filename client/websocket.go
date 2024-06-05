package client

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/lesismal/nbio/nbhttp"
	nbws "github.com/lesismal/nbio/nbhttp/websocket"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/model"
	"github.com/vipcxj/conference.go/websocket"
)

type WebSocketSignalConfigure struct {
	Url   string
	Token string
}

type MsgCb = func(ack AckFunc, arg any)
type CustomMsgCb = func(content string, ack func(), from string, to string)
type AckKey struct {
	user string
	id   uint32
}

type WebsocketSignal struct {
	conf             *WebSocketSignalConfigure
	signal           *websocket.WebSocketSignal
	signal_mux       sync.Mutex
	signal_init_flag bool
	signal_init_ch   chan any

	engine   *nbhttp.Engine
	ctx      context.Context
	close_cb func(err error)

	next_custom_msg_id atomic.Uint32

	msg_mux                  sync.Mutex
	msg_cbs                  map[string]MsgCb
	custom_msg_mux           sync.Mutex
	custom_msg_cbs           map[string]CustomMsgCb
	custom_ack_msg_mux       sync.Mutex
	custom_ack_msg_notifiers map[AckKey]chan any
}

func NewWebsocketSignal(ctx context.Context, conf *WebSocketSignalConfigure, engine *nbhttp.Engine) *WebsocketSignal {
	return &WebsocketSignal{
		conf:    conf,
		engine:  engine,
		ctx:     ctx,
		msg_cbs: make(map[string]func(ack func(any, error), arg any)),
	}
}

func (signal *WebsocketSignal) installMsgCb(evt string, cb MsgCb) {
	signal.signal.On(evt, func(ack websocket.AckFunc, args ...any) {
		my_ack := func(arg any, err error) {
			ack([]any{arg}, err)
		}
		if len(args) == 0 {
			cb(my_ack, nil)
		} else {
			cb(my_ack, args[0])
		}
	})
}

func (signal *WebsocketSignal) GetCustomMsgCb(evt string) CustomMsgCb {
	signal.custom_msg_mux.Lock()
	defer signal.custom_msg_mux.Unlock()
	cb, ok := signal.custom_msg_cbs[evt]
	if ok {
		return cb
	} else {
		return nil
	}
}

func (signal *WebsocketSignal) PushCustomAckMsgCh(to string, id uint32) chan any {
	signal.custom_ack_msg_mux.Lock()
	defer signal.custom_ack_msg_mux.Unlock()
	key := AckKey{
		user: to,
		id:   id,
	}
	ch := make(chan any)
	signal.custom_ack_msg_notifiers[key] = ch
	return ch
}

func (signal *WebsocketSignal) PopCustomAckMsgCh(from string, id uint32) chan any {
	signal.custom_ack_msg_mux.Lock()
	defer signal.custom_ack_msg_mux.Unlock()
	for _, user := range []string{from, ""} {
		key := AckKey{
			user: user,
			id:   id,
		}
		ch, ok := signal.custom_ack_msg_notifiers[key]
		if ok {
			delete(signal.custom_ack_msg_notifiers, key)
			return ch
		}
	}
	return nil
}

func (signal *WebsocketSignal) accessSignal() (*websocket.WebSocketSignal, error) {
	signal.signal_mux.Lock()
	for {
		if signal.signal != nil {
			signal.signal_mux.Unlock()
			return signal.signal, nil
		} else if signal.signal_init_flag {
			signal.signal_mux.Unlock()
			select {
			case <-signal.signal_init_ch:
				signal.signal_mux.Lock()
			case <-signal.ctx.Done():
				return nil, signal.ctx.Err()
			}
		} else {
			signal.signal_init_flag = true
			signal.signal_init_ch = make(chan any)
			signal.signal_mux.Unlock()
			break
		}
	}
	defer func() {
		signal.signal_init_flag = false
		ch := signal.signal_init_ch
		signal.signal_init_ch = nil
		close(ch)
	}()
	u := nbws.NewUpgrader()
	signal.signal = websocket.NewWebSocketSignal(websocket.WS_SIGNAL_MODE_CLIENT, u)
	func() {
		signal.msg_mux.Lock()
		defer signal.msg_mux.Unlock()
		for evt, cb := range signal.msg_cbs {
			signal.installMsgCb(evt, cb)
		}
	}()
	signal.signal.OnCustom(func(evt string, msg *model.CustomMessage) {
		cb := signal.GetCustomMsgCb(evt)
		if cb != nil {
			var ack func()
			if msg.GetAck() {
				ack = func() {
					signal.signal.SendMsg(0, false, "custom-ack", &model.CustomAckMessage{
						Router: &model.RouterMessage{
							UserTo: msg.GetRouter().GetUserFrom(),
						},
						MsgId: msg.GetMsgId(),
					})
				}
			}
			cb(msg.GetContent(), ack, msg.GetRouter().GetUserFrom(), msg.GetRouter().GetUserTo())
		}
	})
	signal.signal.OnCustomAck(func(msg *model.CustomAckMessage) {
		ch := signal.PopCustomAckMsgCh(msg.GetRouter().GetUserFrom(), msg.MsgId)
		if ch != nil {
			close(ch)
		}
	})
	dialer := &nbws.Dialer{
		Engine:      signal.engine,
		Upgrader:    u,
		DialTimeout: time.Second * 6,
	}
	_, _, err := dialer.DialContext(signal.ctx, signal.conf.Url, http.Header{
		"Authorization": {signal.conf.Token},
		"Signal-Id":     {uuid.NewString()},
	})
	if err != nil {
		return nil, err
	}
	signal.signal.OnClose(func(err error) {
		signal.signal_mux.Lock()
		signal.signal = nil
		signal.signal_mux.Unlock()
		signal.close_cb(err)
	})
	return signal.signal, nil
}

func (signal *WebsocketSignal) SendMsg(timeout time.Duration, ack bool, evt string, arg any) (res any, err error) {
	s, err := signal.accessSignal()
	if err != nil {
		return nil, err
	}
	res_arr, err := s.SendMsg(timeout, ack, evt, arg)
	if err != nil {
		return nil, err
	}
	if len(res_arr) == 0 {
		return nil, nil
	} else {
		return res_arr[0], nil
	}
}

func (signal *WebsocketSignal) SendCustomMsg(timeout time.Duration, ack bool, evt string, content string, to string) error {
	s, err := signal.accessSignal()
	if err != nil {
		return err
	}
	custom_msg_id := signal.next_custom_msg_id.Add(1)
	var ch chan any
	if ack {
		ch = signal.PushCustomAckMsgCh(to, custom_msg_id)
	}
	s.SendMsg(0, false, fmt.Sprintf("custom:%s", evt), &model.CustomMessage{
		Router: &model.RouterMessage{
			UserTo: to,
		},
		MsgId: custom_msg_id,
		Ack: ack,
		Content: content,
	})
	if ch != nil {
		if timeout > 0 {
			select {
			case <-time.After(timeout):
				return errors.MsgTimeout("send custom msg with evt %s timeout", evt)
			case <-signal.ctx.Done():
				return signal.ctx.Err()
			case <-ch:
				return nil
			}
		} else {
			select {
			case <-signal.ctx.Done():
				return signal.ctx.Err()
			case <-ch:
				return nil
			}
		}
	} else {
		return nil
	}
}

func (signal *WebsocketSignal) On(evt string, cb MsgCb) error {
	signal.msg_mux.Lock()
	signal.msg_cbs[evt] = cb
	signal.msg_mux.Unlock()
	signal.signal_mux.Lock()
	defer signal.signal_mux.Unlock()
	if signal.signal != nil {
		signal.installMsgCb(evt, cb)
	}
	return nil
}

func (signal *WebsocketSignal) OnCustom(evt string, cb CustomMsgCb) {
	signal.custom_msg_mux.Lock()
	defer signal.custom_msg_mux.Unlock()
	signal.custom_msg_cbs[evt] = cb
}
