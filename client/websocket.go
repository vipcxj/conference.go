package client

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/lesismal/nbio/nbhttp"
	nbws "github.com/lesismal/nbio/nbhttp/websocket"
	"github.com/mitchellh/mapstructure"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/log"
	"github.com/vipcxj/conference.go/model"
	sg "github.com/vipcxj/conference.go/signal"
	"github.com/vipcxj/conference.go/utils"
	"github.com/vipcxj/conference.go/websocket"
)

type WebSocketSignalConfigure struct {
	Url   string
	Token string
}

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

	msg_cbs                  *sg.MsgCbs
	custom_msg_mux           sync.Mutex
	custom_msg_cbs           map[string][]CustomMsgCb
	custom_ack_msg_mux       sync.Mutex
	custom_ack_msg_notifiers map[AckKey]chan any

	user_info     *model.UserInfo
	user_info_mux sync.Mutex

	ping_msg_id  atomic.Uint32
	ping_chs     map[PingKey][]chan *model.PingMessage
	ping_chs_mux sync.Mutex
}

type RoomedWebsocketSignal struct {
	room   string
	signal *WebsocketSignal
}

func newRoomedWebsocketSignal(room string, signal *WebsocketSignal) *RoomedWebsocketSignal {
	return &RoomedWebsocketSignal{
		room:   room,
		signal: signal,
	}
}

func (s *RoomedWebsocketSignal) GetRoom() string {
	return s.room
}

func (s *RoomedWebsocketSignal) SendMessage(timeout time.Duration, ack bool, evt string, content string, to string) error {
	return s.signal.SendMessage(timeout, ack, evt, content, to, s.GetRoom())
}

func (s *RoomedWebsocketSignal) OnMessage(evt string, cb RoomedCustomMsgCb) {
	s.signal.OnMessage(evt, func(content string, ack func(), room, from, to string) (remained bool) {
		if room == s.GetRoom() {
			return cb(content, ack, from, to)
		} else {
			return true
		}
	})
}

func (s *RoomedWebsocketSignal) KeepAlive(uid string, mode KeepAliveMode, timeout time.Duration, errCb KeepAliveCb) (stopFun func(), err error) {
	return s.signal.KeepAlive(s.GetRoom(), uid, mode, timeout, errCb)
}

func NewWebsocketSignal(ctx context.Context, conf *WebSocketSignalConfigure, engine *nbhttp.Engine) *WebsocketSignal {
	return &WebsocketSignal{
		conf:           conf,
		engine:         engine,
		ctx:            ctx,
		msg_cbs:        sg.NewMsgCbs(),
		custom_msg_cbs: make(map[string][]CustomMsgCb),
		ping_chs:       make(map[PingKey][]chan *model.PingMessage),
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

func (signal *WebsocketSignal) MakesureConnect() error {
	_, err := signal.accessSignal()
	return err
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

	signal.signal.On(func(evt string, ack websocket.AckFunc, args ...any) {
		signal.msg_cbs.Run(evt, ack, args...)
	})

	signal.signal.OnCustom(func(evt string, msg *model.CustomMessage) {
		signal.custom_msg_mux.Lock()
		defer signal.custom_msg_mux.Unlock()
		cbs, ok := signal.custom_msg_cbs[evt]
		new_cbs := make([]CustomMsgCb, 0)
		if ok {
			for _, cb := range cbs {
				var ack func()
				router := msg.GetRouter()
				if msg.GetAck() {
					ack = func() {
						signal.signal.SendMsg(0, false, "custom-ack", &model.CustomAckMessage{
							Router: &model.RouterMessage{
								UserTo: router.GetUserFrom(),
							},
							MsgId: msg.GetMsgId(),
						})
					}
				}
				if cb(msg.GetContent(), ack, router.GetRoom(), router.GetUserFrom(), router.GetUserTo()) {
					new_cbs = append(new_cbs, cb)
				}
			}
		}
		if len(new_cbs) > 0 {
			signal.custom_msg_cbs[evt] = new_cbs
		} else {
			delete(signal.custom_msg_cbs, evt)
		}
	})
	signal.signal.OnCustomAck(func(msg *model.CustomAckMessage) {
		ch := signal.PopCustomAckMsgCh(msg.GetRouter().GetUserFrom(), msg.MsgId)
		if ch != nil {
			close(ch)
		}
	})
	signal.onMsg("ping", func(ack AckFunc, arg any) (remained bool) {
		remained = true
		msg := &model.PingMessage{}
		err := mapstructure.Decode(arg, msg)
		if err != nil {
			log.Sugar().Errorf("unable to decode ping msg, %v", err)
			return
		}
		go func() {
			signal.publishPingMsgs(msg)
			router := msg.GetRouter()
			_, err := signal.sendMsg(6*time.Second, false, "pong", &model.PongMessage{
				Router: &model.RouterMessage{
					Room:   router.GetRoom(),
					UserTo: router.GetUserFrom(),
				},
				MsgId: msg.MsgId,
			})
			if err != nil {
				log.Sugar().Errorf("unable to send pong msg to user %s in room %s, %v", router.GetUserFrom(), router.GetRoom(), err)
			}
		}()
		return
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
		if signal.close_cb != nil {
			signal.close_cb(err)
		}
	})
	return signal.signal, nil
}

func (signal *WebsocketSignal) userInfo(timeout time.Duration) (*model.UserInfo, error) {
	if signal.user_info == nil {
		res, err := signal.sendMsg(timeout, true, "user-info", nil)
		if err != nil {
			return nil, fmt.Errorf("unable to send user-info msg, %w", err)
		}
		var info model.UserInfo
		err = mapstructure.Decode(res, &info)
		if err != nil {
			return nil, fmt.Errorf("unable to decode the user-info msg, %w", err)
		}
		signal.user_info = &info
	}
	return signal.user_info, nil
}

func (signal *WebsocketSignal) UserInfo(timeout time.Duration) (*model.UserInfo, error) {
	signal.user_info_mux.Lock()
	defer signal.user_info_mux.Unlock()
	return signal.userInfo(timeout)
}

func (signal *WebsocketSignal) IsInRoom(timeout time.Duration, room string) (bool, error) {
	userInfo, err := signal.UserInfo(timeout)
	if err != nil {
		return false, err
	}
	return slices.Contains(userInfo.Rooms, room), nil
}

func (signal *WebsocketSignal) sendMsg(timeout time.Duration, ack bool, evt string, arg any) (res any, err error) {
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

func (signal *WebsocketSignal) SendMessage(timeout time.Duration, ack bool, evt string, content string, to string, room string) error {
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
			Room:   room,
			UserTo: to,
		},
		MsgId:   custom_msg_id,
		Ack:     ack,
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

func args2arg(args ...any) (arg any) {
	if len(args) == 0 {
		return nil
	} else {
		return args[0]
	}
}

func (signal *WebsocketSignal) onMsg(evt string, cb MsgCb) error {
	signal.msg_cbs.AddCallback(evt, func(ack sg.AckFunc, args ...any) (remained bool) {
		my_ack := func(arg any, err error) {
			ack([]any{arg}, err)
		}
		return cb(my_ack, args2arg(args...))
	})
	return nil
}

func (signal *WebsocketSignal) OnMessage(evt string, cb CustomMsgCb) {
	signal.custom_msg_mux.Lock()
	defer signal.custom_msg_mux.Unlock()
	cbs, ok := signal.custom_msg_cbs[evt]
	if ok {
		cbs = append(cbs, cb)
		signal.custom_msg_cbs[evt] = cbs
	} else {
		signal.custom_msg_cbs[evt] = []CustomMsgCb{cb}
	}
}

func (signal *WebsocketSignal) Join(timeout time.Duration, rooms ...string) error {
	_, err := signal.sendMsg(timeout, true, "join", &model.JoinMessage{
		Rooms: rooms,
	})
	if err != nil {
		return err
	}
	signal.user_info_mux.Lock()
	defer signal.user_info_mux.Unlock()
	uInfo, err := signal.userInfo(timeout)
	if err != nil {
		return err
	}
	uInfo.Rooms = utils.SliceAppendNoRepeat(uInfo.Rooms, rooms...)
	return nil
}

func (signal *WebsocketSignal) Leave(timeout time.Duration, rooms ...string) error {
	_, err := signal.sendMsg(timeout, true, "leave", &model.JoinMessage{
		Rooms: rooms,
	})
	if err != nil {
		return err
	}
	signal.user_info_mux.Lock()
	defer signal.user_info_mux.Unlock()
	uInfo, err := signal.userInfo(timeout)
	if err != nil {
		return err
	}
	uInfo.Rooms = utils.SliceRemoveByValues(uInfo.Rooms, false, rooms...)
	return nil
}

func (c *WebsocketSignal) publishPingMsgs(msg *model.PingMessage) {
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

func (s *WebsocketSignal) subscribePingMsg(room string, uid string) chan *model.PingMessage {
	key := PingKey{
		Uid:  uid,
		Room: room,
	}
	s.ping_chs_mux.Lock()
	defer s.ping_chs_mux.Unlock()
	ch := make(chan *model.PingMessage, 1)
	chs, ok := s.ping_chs[key]
	if ok {
		s.ping_chs[key] = append(chs, ch)
	} else {
		s.ping_chs[key] = []chan *model.PingMessage{ch}
	}
	return ch
}

func (c *WebsocketSignal) unsubscribePingMsg(room string, uid string, ch chan *model.PingMessage) {
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

func (c *WebsocketSignal) KeepAlive(room string, uid string, mode KeepAliveMode, timeout time.Duration, errCb KeepAliveCb) (stopFun func(), err error) {
	err = c.MakesureConnect()
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
				msg_id := c.ping_msg_id.Add(1)
				ch := make(chan any)
				errCh := make(chan error, 1)
				c.onMsg("pong", func(ack AckFunc, arg any) (remained bool) {
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
				_, err := c.sendMsg(timeout, false, "ping", &model.PingMessage{
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
					var leftTimeout time.Duration
					since := time.Since(now)
					if since >= timeout {
						leftTimeout = time.Millisecond
					} else {
						leftTimeout = timeout - since
					}
					select {
					case <-ctx.Done():
						return
					case <-time.After(leftTimeout):
						kaCtx.TimeoutNum++
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
					kaCtx.TimeoutNum++
					kaCtx.TimeoutDuration += time.Since(now)
					if errCb(kaCtx) {
						c.unsubscribePingMsg(room, uid, ch)
						return
					}
				case <-ch:
					kaCtx.TimeoutNum = 0
					kaCtx.TimeoutDuration = 0
				}
			}
		default:
			log.Sugar().Panicf("invalid keep alive mode: %v", mode)
		}
	}()
	return cancel, nil
}

func (c *WebsocketSignal) Roomed(timeout time.Duration, room string) (RoomedSignal, error) {
	inRoom, err := c.IsInRoom(timeout, room)
	if err != nil {
		return nil, err
	}
	if !inRoom {
		return nil, fmt.Errorf("not in room %s", room)
	}
	return newRoomedWebsocketSignal(room, c), nil
}