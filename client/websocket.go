package client

import (
	"context"
	"encoding/json"
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
	Url          string
	Token        string
	ReadyTimeout time.Duration
}

type AckKey struct {
	room     string
	socketId string
	id       uint32
}

type ParticipantKey struct {
	uid string
	sid string
}

type LeavedInfo struct {
	jid       uint32
	timestamp time.Time
}

type Participants struct {
	participants map[ParticipantKey]*model.Participant
	leaves       map[ParticipantKey]*LeavedInfo
}

func (p *Participants) FindOneByUser(uid string) *model.Participant {
	for key, value := range p.participants {
		if key.uid == uid {
			return value
		}
	}
	return nil
}

func (p *Participants) Clean() (empty bool) {
	now := time.Now()
	for key, info := range p.leaves {
		if now.Sub(info.timestamp) > time.Minute {
			delete(p.leaves, key)
		}
	}
	return len(p.participants) == 0 && len(p.leaves) == 0
}

func (p *Participants) Add(participant *model.Participant) bool {
	key := ParticipantKey{
		uid: participant.Id,
		sid: participant.SourceId,
	}
	leaveInfo, ok := p.leaves[key]
	if ok {
		if participant.JoinId > leaveInfo.jid {
			delete(p.leaves, key)
		} else {
			return false
		}
	}
	success := false
	pt, ok := p.participants[key]
	if ok {
		if participant.JoinId > pt.JoinId {
			p.participants[key] = participant
			success = true
		}
	} else {
		p.participants[key] = participant
		success = true
	}
	return success
}

func (p *Participants) Remove(uid string, sid string, jid uint32) *model.Participant {
	key := ParticipantKey{
		uid: uid,
		sid: sid,
	}
	leaveInfo, ok := p.leaves[key]
	if ok {
		if jid > leaveInfo.jid {
			leaveInfo.jid = jid
			leaveInfo.timestamp = time.Now()
		} else {
			return nil
		}
	} else {
		p.leaves[key] = &LeavedInfo{
			jid:       jid,
			timestamp: time.Now(),
		}
	}
	pt, ok := p.participants[key]
	if ok {
		if jid >= pt.JoinId {
			delete(p.participants, key)
			return pt
		}
	}
	return nil
}

type ParticipantsBox struct {
	participants  map[string]*Participants
	next_join_id  int
	join_chs      map[string]map[int]ParticipantCb
	next_leave_id int
	leave_cbs     map[string]map[int]ParticipantCb
	iter          int
	mux           sync.Mutex
}

func mewParticipantsBox() *ParticipantsBox {
	return &ParticipantsBox{
		participants: make(map[string]*Participants),
		join_chs:     make(map[string]map[int]ParticipantCb),
		leave_cbs:    make(map[string]map[int]ParticipantCb),
	}
}

func (box *ParticipantsBox) clean() {
	box.iter++
	if box.iter%10 == 0 {
		for room, participants := range box.participants {
			if participants.Clean() {
				delete(box.participants, room)
			}
		}
	}
}

func (box *ParticipantsBox) ProcessJoinCbs(msg *model.StateParticipantMessage) {
	room := msg.GetRouter().GetRoom()
	participant := &model.Participant{
		Id:       msg.GetUserId(),
		Name:     msg.GetUserName(),
		SourceId: msg.GetSocketId(),
		JoinId:   msg.GetJoinId(),
	}
	box.mux.Lock()
	defer box.mux.Unlock()
	participants, ok := box.participants[room]
	added := false
	if ok {
		added = participants.Add(participant)
	} else {
		ps := &Participants{
			participants: make(map[ParticipantKey]*model.Participant),
			leaves:       make(map[ParticipantKey]*LeavedInfo),
		}
		added = ps.Add(participant)
		// added must be true
		box.participants[room] = ps
	}
	if added {
		join_cbs, ok := box.join_chs[room]
		if ok {
			for id, cb := range join_cbs {
				if !cb(participant) {
					delete(join_cbs, id)
				}
			}
		}
	}
}

func (box *ParticipantsBox) ProcessLeaveCbs(msg *model.StateLeaveMessage) {
	room := msg.GetRouter().GetRoom()
	box.mux.Lock()
	defer box.mux.Unlock()
	participants, ok := box.participants[room]
	var participant *model.Participant
	if ok {
		participant = participants.Remove(msg.UserId, msg.SocketId, msg.JoinId)
	} else {
		ps := &Participants{
			participants: make(map[ParticipantKey]*model.Participant),
			leaves:       make(map[ParticipantKey]*LeavedInfo),
		}
		participant = ps.Remove(msg.UserId, msg.SocketId, msg.JoinId)
		box.participants[room] = ps
	}
	box.clean()
	leave_cbs, ok := box.leave_cbs[room]
	if ok {
		for id, cb := range leave_cbs {
			if !cb(participant) {
				delete(leave_cbs, id)
			}
		}
	}
}

func (ps *ParticipantsBox) AddJoinCallback(room string, cb ParticipantCb) int {
	ps.mux.Lock()
	defer ps.mux.Unlock()
	join_cbs, ok := ps.join_chs[room]
	id := ps.next_join_id
	ps.next_join_id++
	if ok {
		join_cbs[id] = cb
	} else {
		ps.join_chs[room] = map[int]ParticipantCb{
			id: cb,
		}
	}
	return id
}

func (ps *ParticipantsBox) RemoveJoinCallback(room string, id int) {
	ps.mux.Lock()
	defer ps.mux.Unlock()
	join_cbs, ok := ps.join_chs[room]
	if ok {
		delete(join_cbs, id)
	}
}

func (ps *ParticipantsBox) AddLeaveCallback(room string, cb ParticipantCb) int {
	ps.mux.Lock()
	defer ps.mux.Unlock()
	leave_cbs, ok := ps.leave_cbs[room]
	id := ps.next_leave_id
	ps.next_leave_id++
	if ok {
		leave_cbs[id] = cb
	} else {
		ps.leave_cbs[room] = map[int]ParticipantCb{
			id: cb,
		}
	}
	return id
}

func (ps *ParticipantsBox) RemoveLeaveCallback(room string, id int) {
	ps.mux.Lock()
	defer ps.mux.Unlock()
	leave_cbs, ok := ps.leave_cbs[room]
	if ok {
		delete(leave_cbs, id)
	}
}

func (box *ParticipantsBox) FindOneByUser(room string, uid string) *model.Participant {
	box.mux.Lock()
	defer box.mux.Unlock()
	participants, ok := box.participants[room]
	if ok {
		return participants.FindOneByUser(uid)
	} else {
		return nil
	}
}

type ackMsg struct {
	ch  chan struct{}
	msg *model.CustomAckMessage
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
	custom_ack_msg_notifiers map[AckKey]*ackMsg

	user_info *model.UserInfo
	rooms     []string
	rooms_mux sync.Mutex

	ping_msg_id  atomic.Uint32
	ping_chs     map[PingKey][]chan *model.PingMessage
	ping_chs_mux sync.Mutex

	participants *ParticipantsBox
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

func (s *RoomedWebsocketSignal) SendMessage(ctx context.Context, ack bool, evt string, content string, to string) (res string, err error) {
	return s.signal.SendMessage(ctx, ack, evt, content, to, s.GetRoom())
}

func (s *RoomedWebsocketSignal) OnMessage(evt string, cb RoomedCustomMsgCb) {
	s.signal.OnMessage(evt, func(content string, ack CustomAckFunc, room, from, to string) (remained bool) {
		if room == s.GetRoom() {
			return cb(content, ack, from, to)
		} else {
			return true
		}
	})
}

func (s *RoomedWebsocketSignal) OnParticipantJoin(cb ParticipantCb) int {
	return s.signal.participants.AddJoinCallback(s.GetRoom(), cb)
}

func (s *RoomedWebsocketSignal) OffParticipantJoin(id int) {
	s.signal.participants.RemoveJoinCallback(s.GetRoom(), id)
}

func (s *RoomedWebsocketSignal) OnParticipantLeave(cb ParticipantCb) int {
	return s.signal.participants.AddLeaveCallback(s.GetRoom(), cb)
}

func (s *RoomedWebsocketSignal) OffParticipantLeave(id int) {
	s.signal.participants.RemoveLeaveCallback(s.GetRoom(), id)
}

func (s *RoomedWebsocketSignal) WaitParticipant(ctx context.Context, uid string) *model.Participant {
	ch := make(chan struct{})
	cbId := s.OnParticipantJoin(func(participant *model.Participant) (remained bool) {
		if participant.Id == uid {
			close(ch)
			return false
		}
		return true
	})
	defer s.OffParticipantJoin(cbId)
	participant := s.signal.participants.FindOneByUser(s.GetRoom(), uid)
	if participant != nil {
		return participant
	}
	select {
	case <-ctx.Done():
	case <-ch:
	}
	return s.signal.participants.FindOneByUser(s.GetRoom(), uid)
}

func (s *RoomedWebsocketSignal) KeepAlive(ctx context.Context, uid string, mode KeepAliveMode, timeout time.Duration, errCb KeepAliveCb) (stopFun func(), err error) {
	return s.signal.KeepAlive(ctx, s.GetRoom(), uid, mode, timeout, errCb)
}

func NewWebsocketSignal(ctx context.Context, conf *WebSocketSignalConfigure, engine *nbhttp.Engine) *WebsocketSignal {
	return &WebsocketSignal{
		conf:                     conf,
		engine:                   engine,
		ctx:                      ctx,
		msg_cbs:                  sg.NewMsgCbs(),
		custom_msg_cbs:           make(map[string][]CustomMsgCb),
		custom_ack_msg_notifiers: make(map[AckKey]*ackMsg),
		ping_chs:                 make(map[PingKey][]chan *model.PingMessage),
		participants:             mewParticipantsBox(),
	}
}

func (signal *WebsocketSignal) Id() string {
	u_info := signal.user_info
	if u_info != nil {
		return u_info.SocketId
	} else {
		return ""
	}
}

func (signal *WebsocketSignal) PushCustomAckMsgCh(room string, to string, id uint32) *ackMsg {
	signal.custom_ack_msg_mux.Lock()
	defer signal.custom_ack_msg_mux.Unlock()
	key := AckKey{
		room:     room,
		socketId: to,
		id:       id,
	}
	ack_msg := &ackMsg{
		ch: make(chan struct{}),
	}
	signal.custom_ack_msg_notifiers[key] = ack_msg
	return ack_msg
}

func (signal *WebsocketSignal) PopCustomAckMsgCh(room string, from string, id uint32) *ackMsg {
	signal.custom_ack_msg_mux.Lock()
	defer signal.custom_ack_msg_mux.Unlock()
	for _, socketId := range []string{from, ""} {
		key := AckKey{
			room:     room,
			socketId: socketId,
			id:       id,
		}
		ch, ok := signal.custom_ack_msg_notifiers[key]
		if ok {
			delete(signal.custom_ack_msg_notifiers, key)
			return ch
		}
	}
	return nil
}

func (signal *WebsocketSignal) MakesureConnect(ctx context.Context) error {
	_, err := signal.accessSignal(ctx)
	return err
}

func (signal *WebsocketSignal) accessSignal(ctx context.Context) (*websocket.WebSocketSignal, error) {
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
				var ack CustomAckFunc
				router := msg.GetRouter()
				if msg.GetAck() {
					ack = func(res string, err *errors.ConferenceError) {
						if err != nil {
							js_err, ms_err := json.Marshal(err)
							if ms_err != nil {
								res = fmt.Sprintf("unable to encode the custom ack err, %v", ms_err.Error())
							} else {
								res = string(js_err)
							}
						}
						signal.signal.SendMsg(signal.ctx, false, "custom-ack", &model.CustomAckMessage{
							Router: &model.RouterMessage{
								SocketTo: router.GetSocketFrom(),
							},
							MsgId:   msg.GetMsgId(),
							Content: res,
							Err:     err != nil,
						})
					}
				}
				if cb(msg.GetContent(), ack, router.GetRoom(), router.GetSocketFrom(), router.GetSocketTo()) {
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
		ack_msg := signal.PopCustomAckMsgCh(msg.GetRouter().GetRoom(), msg.GetRouter().GetSocketFrom(), msg.MsgId)
		if ack_msg != nil {
			ack_msg.msg = msg
			close(ack_msg.ch)
		}
	})
	ready_ch := make(chan struct{})
	signal.onMsg("ready", func(ack AckFunc, arg any) (remained bool) {
		defer close(ready_ch)
		msg := model.UserInfo{}
		err := mapstructure.Decode(arg, &msg)
		if err != nil {
			log.Sugar().Errorf("unable to decode ready msg, %v", err)
			return
		}
		signal.rooms_mux.Lock()
		signal.rooms = msg.Rooms
		signal.rooms_mux.Unlock()
		msg.Rooms = nil
		signal.user_info = &msg
		return false
	})
	signal.onMsg("participant-join", func(ack AckFunc, arg any) (remained bool) {
		remained = true
		msg := model.StateParticipantMessage{}
		err := mapstructure.Decode(arg, &msg)
		if err != nil {
			log.Sugar().Errorf("unable to decode participant join msg, %v", err)
			return
		}
		signal.participants.ProcessJoinCbs(&msg)
		return
	})
	signal.onMsg("participant-leave", func(ack AckFunc, arg any) (remained bool) {
		remained = true
		msg := model.StateLeaveMessage{}
		err := mapstructure.Decode(arg, &msg)
		if err != nil {
			log.Sugar().Errorf("unable to decode participant leave msg, %v", err)
			return
		}
		signal.participants.ProcessLeaveCbs(&msg)
		return
	})
	signal.signal.OnClose(func(err error) {
		signal.signal_mux.Lock()
		signal.signal = nil
		signal.signal_mux.Unlock()
		if signal.close_cb != nil {
			signal.close_cb(err)
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
			_, err := signal.sendMsg(signal.ctx, false, "pong", &model.PongMessage{
				Router: &model.RouterMessage{
					Room:     router.GetRoom(),
					SocketTo: router.GetSocketFrom(),
				},
				MsgId: msg.MsgId,
			})
			if err != nil {
				log.Sugar().Errorf("unable to send pong msg to socket %s in room %s, %v", router.GetSocketFrom(), router.GetRoom(), err)
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
	toCh, stopToCh := utils.MakeTimeoutChan(signal.conf.ReadyTimeout)
	defer stopToCh()
	select {
	case <-toCh:
		return nil, fmt.Errorf("ready timeout")
	case <-ready_ch:
		if signal.user_info != nil {
			return signal.signal, nil
		} else {
			return nil, fmt.Errorf("failed to get user info")
		}
	case <-ctx.Done():
		return nil, context.Cause(ctx)
	case <-signal.ctx.Done():
		return nil, context.Cause(signal.ctx)
	}
}

func (signal *WebsocketSignal) UserInfo(ctx context.Context) (*model.UserInfo, error) {
	_, err := signal.accessSignal(ctx)
	if err != nil {
		return nil, err
	}
	return signal.user_info, nil
}

func (signal *WebsocketSignal) GetRooms(ctx context.Context) ([]string, error) {
	_, err := signal.accessSignal(ctx)
	if err != nil {
		return nil, err
	}
	signal.rooms_mux.Lock()
	defer signal.rooms_mux.Unlock()
	return slices.Clone(signal.rooms), nil
}

func (signal *WebsocketSignal) IsInRoom(ctx context.Context, room string) (bool, error) {
	_, err := signal.accessSignal(ctx)
	if err != nil {
		return false, err
	}
	signal.rooms_mux.Lock()
	defer signal.rooms_mux.Unlock()
	return slices.Contains(signal.rooms, room), nil
}

func (signal *WebsocketSignal) sendMsg(ctx context.Context, ack bool, evt string, arg any) (res any, err error) {
	s, err := signal.accessSignal(ctx)
	if err != nil {
		return nil, err
	}
	res_arr, err := s.SendMsg(ctx, ack, evt, arg)
	if err != nil {
		return nil, err
	}
	if len(res_arr) == 0 {
		return nil, nil
	} else {
		return res_arr[0], nil
	}
}

func (signal *WebsocketSignal) SendMessage(ctx context.Context, ack bool, evt string, content string, to string, room string) (res string, err error) {
	s, err := signal.accessSignal(ctx)
	if err != nil {
		return "", err
	}
	custom_msg_id := signal.next_custom_msg_id.Add(1)
	var ack_msg *ackMsg
	if ack {
		ack_msg = signal.PushCustomAckMsgCh(room, to, custom_msg_id)
	}
	s.SendMsg(ctx, false, fmt.Sprintf("custom:%s", evt), &model.CustomMessage{
		Router: &model.RouterMessage{
			Room:     room,
			SocketTo: to,
		},
		MsgId:   custom_msg_id,
		Ack:     ack,
		Content: content,
	})
	if ack_msg != nil {
		select {
		case <-ctx.Done():
			return "", context.Cause(ctx)
		case <-signal.ctx.Done():
			return "", context.Cause(signal.ctx)
		case <-ack_msg.ch:
			msg := ack_msg.msg
			content := msg.GetContent()
			if msg.GetErr() {
				var ce errors.ConferenceError
				err = json.Unmarshal([]byte(content), &ce)
				if err != nil {
					return "", errors.FatalError("unable to decode custom ack err \"%s\", %w", content, err)
				} else {
					return "", &ce
				}
			} else {
				return content, nil
			}
		}
	} else {
		return "", nil
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
		my_ack := func(arg any, err *errors.ConferenceError) {
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

func (signal *WebsocketSignal) Join(ctx context.Context, rooms ...string) error {
	_, err := signal.sendMsg(ctx, true, "join", &model.JoinMessage{
		Rooms: rooms,
	})
	if err != nil {
		return err
	}
	signal.rooms_mux.Lock()
	defer signal.rooms_mux.Unlock()
	signal.rooms = utils.SliceAppendNoRepeat(signal.rooms, rooms...)
	return nil
}

func (signal *WebsocketSignal) Leave(ctx context.Context, rooms ...string) error {
	_, err := signal.sendMsg(ctx, true, "leave", &model.JoinMessage{
		Rooms: rooms,
	})
	if err != nil {
		return err
	}
	signal.rooms_mux.Lock()
	defer signal.rooms_mux.Unlock()
	signal.rooms = utils.SliceRemoveByValues(signal.rooms, false, rooms...)
	return nil
}

func (c *WebsocketSignal) publishPingMsgs(msg *model.PingMessage) {
	router := msg.GetRouter()
	key := PingKey{
		Sid:  router.GetSocketFrom(),
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

func (s *WebsocketSignal) subscribePingMsg(room string, sid string) chan *model.PingMessage {
	key := PingKey{
		Sid:  sid,
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

func (c *WebsocketSignal) unsubscribePingMsg(room string, sid string, ch chan *model.PingMessage) {
	key := PingKey{
		Sid:  sid,
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

func (c *WebsocketSignal) KeepAlive(ctx context.Context, room string, sid string, mode KeepAliveMode, timeout time.Duration, errCb KeepAliveCb) (stopFun func(), err error) {
	err = c.MakesureConnect(ctx)
	if err != nil {
		return
	}
	ctx, cancel := context.WithCancel(ctx)
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
					if msg.MsgId == msg_id && msg.GetRouter().GetSocketFrom() == sid && msg.GetRouter().GetRoom() == room {
						close(ch)
						return false
					} else {
						return true
					}
				})
				_, err := c.sendMsg(ctx, false, "ping", &model.PingMessage{
					Router: &model.RouterMessage{
						Room:     room,
						SocketTo: sid,
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
					case <-c.ctx.Done():
						return
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
				case <-c.ctx.Done():
				case <-ctx.Done():
					return
				case <-time.After(timeout):
				}
			}
		case KEEP_ALIVE_MODE_PASSIVE:
			ch := c.subscribePingMsg(room, sid)
			for {
				now := time.Now()
				select {
				case <-c.ctx.Done():
					c.unsubscribePingMsg(room, sid, ch)
					return
				case <-ctx.Done():
					c.unsubscribePingMsg(room, sid, ch)
					return
				case <-time.After(timeout):
					kaCtx.TimeoutNum++
					kaCtx.TimeoutDuration += time.Since(now)
					if errCb(kaCtx) {
						c.unsubscribePingMsg(room, sid, ch)
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

func (s *WebsocketSignal) Roomed(ctx context.Context, room string) (RoomedSignal, error) {
	inRoom, err := s.IsInRoom(ctx, room)
	if err != nil {
		return nil, err
	}
	if !inRoom {
		return nil, fmt.Errorf("not in room %s", room)
	}
	return newRoomedWebsocketSignal(room, s), nil
}
