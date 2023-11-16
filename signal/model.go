package signal

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/alphadose/haxmap"
	"github.com/google/uuid"
	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/intervalpli"
	"github.com/pion/webrtc/v4"
	"github.com/vipcxj/conference.go/auth"
	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/log"
	"github.com/zishang520/socket.io/v2/socket"
	"go.uber.org/zap"
)

type SubscribedTrack struct {
	sub      *Subscription
	pubTrack *Track
	sid      string
	accepter *Accepter
	sender   *webrtc.RTPSender
	bindId   string
	labels   map[string]string
}

func (st *SubscribedTrack) Remove() {
	// ignore error
	st.sub.ctx.Peer.RemoveTrack(st.sender)
	delete(st.sub.acceptedTrack, st.pubTrack.GlobalId)
}

type Subscription struct {
	id            string
	mu            sync.Mutex
	closed        bool
	reqTypes      []string
	pattern       *PublicationPattern
	ctx           *SignalContext
	acceptedPubId string
	acceptedTrack map[string]*SubscribedTrack
}

func (s *Subscription) AcceptTrack(msg *StateMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if s.acceptedPubId != "" && s.acceptedPubId != msg.PubId {
		return
	}
	matcheds, unmatcheds := s.pattern.MatchTracks(msg.Tracks, s.reqTypes)
	if s.acceptedPubId == msg.PubId {
		for _, unmatch := range unmatcheds {
			subscribedTrack, ok := s.acceptedTrack[unmatch.GlobalId]
			if ok {
				subscribedTrack.Remove()
			}
		}
		if len(matcheds) == 0 {
			s.acceptedPubId = ""
			return
		}
	}
	if len(matcheds) == 0 {
		return
	}
	s.ctx.Sugar().Infof("accept pub %v", msg)
	s.acceptedPubId = msg.PubId
	router := GetRouter()
	// accept track from addr
	router.makeSureExternelConn(msg.Addr)
	peer, err := s.ctx.MakeSurePeer()
	if err != nil {
		panic(err)
	}
	if s.acceptedTrack == nil {
		s.acceptedTrack = map[string]*SubscribedTrack{}
	}
	need_neg := false
	for _, matched := range matcheds {
		subscribedTrack, ok := s.acceptedTrack[matched.GlobalId]
		if !ok {
			accepter := router.AcceptTrack(matched)
			sender, err := peer.AddTrack(accepter.track)
			if err != nil {
				panic(err)
			}
			need_neg = true
			// Read incoming RTCP packets
			// Before these packets are returned they are processed by interceptors. For things
			// like NACK this needs to be called.
			go func() {
				rtcpBuf := make([]byte, 1600)
				for {
					if _, _, err := sender.Read(rtcpBuf); err != nil {
						return
					}
				}
			}()
			mid := getMidFromSender(peer, sender)
			sid := accepter.track.StreamID()
			subscribedTrack = &SubscribedTrack{
				sub:      s,
				sid:      sid,
				pubTrack: matched,
				bindId:   mid,
				accepter: accepter,
				sender:   sender,
				labels:   matched.Labels,
			}
			s.acceptedTrack[matched.GlobalId] = subscribedTrack
			accepter.BindSub(s)
			matched.LocalId = accepter.track.ID()
			matched.BindId = mid
			matched.StreamId = sid
		} else {
			matched.LocalId = subscribedTrack.accepter.track.ID()
			matched.BindId = subscribedTrack.bindId
			matched.StreamId = subscribedTrack.sid
		}
	}
	if need_neg {
		sdpId := s.ctx.NextSdpMsgId()
		s.ctx.Sugar().Infof("gen sdp id: %v", sdpId)

		selMsg := &SelectMessage{
			PubId:       msg.PubId,
			Tracks:      matcheds,
			TransportId: router.id.String(),
		}
		s.ctx.Socket.To(s.ctx.Rooms()...).Emit("select", selMsg)
		s.ctx.Socket.Emit("select", selMsg)
		respMsg := &SubscribedMessage{
			SubId:  s.id,
			PubId:  msg.PubId,
			SdpId:  sdpId,
			Tracks: matcheds,
		}
		s.ctx.Socket.Emit("subscribed", respMsg)
		go func() {
			s.ctx.StartNegotiate(peer, sdpId)
		}()
	}
}

func (s *Subscription) UnbindAccepter(accepter *Accepter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// when closed, s.acceptedTrack is empty, so no need check closed
	for _, track := range s.acceptedTrack {
		if track.accepter == accepter {
			track.Remove()
		}
	}
}

func (s *Subscription) UpdatePattern(pattern *PublicationPattern) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// when closed, s.acceptedTrack is empty, so no need check closed
	for _, track := range s.acceptedTrack {
		if !pattern.Match(track.pubTrack) {
			track.Remove()
		}
	}
	s.pattern = pattern
}

func (s *Subscription) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.ctx.subscriptions.Del(s.id)
	for _, track := range s.acceptedTrack {
		track.Remove()
	}
	s.acceptedTrack = nil
}

type Publication struct {
	id      string
	closeCh chan struct{}
	ctx     *SignalContext
	tracks  *haxmap.Map[string, *PublishedTrack]
}

func (me *Publication) isAllBind() bool {
	res := true
	me.tracks.ForEach(func(gid string, pt *PublishedTrack) bool {
		if !pt.isBind() {
			res = false
			return false
		} else {
			return true
		}
	})
	return res
}

func (me *Publication) Bind() bool {
	done := false
	me.tracks.ForEach(func(gid string, pt *PublishedTrack) bool {
		if pt.Bind() {
			done = true
			if me.isAllBind() {
				r := GetRouter()
				var tracks []*Track
				me.tracks.ForEach(func(_ string, pt0 *PublishedTrack) bool {
					tracks = append(tracks, pt0.track)
					return true
				})
				msg := &StateMessage{
					PubId:  me.id,
					Tracks: tracks,
					Addr:   r.Addr(),
				}
				// to all in room include self
				me.ctx.Socket.To(me.ctx.Rooms()...).Emit("state", msg)
				me.ctx.Socket.Emit("state", msg)
			}
			return false
		} else {
			return true
		}
	})
	return done
}

func (me *Publication) State(want *WantMessage) []*Track {
	satifiedMap := map[string]*PublishedTrack{}
	me.tracks.ForEach(func(gid string, pt *PublishedTrack) bool {
		if pt.StateWant(want) {
			satifiedMap[pt.GlobalId()] = pt
		}
		return true
	})
	var satified []*Track
	for _, pt := range satifiedMap {
		satified = append(satified, pt.track)
	}
	return satified
}

func (me *Publication) SatifySelect(sel *SelectMessage) {
	for _, t := range sel.Tracks {
		pt, ok := me.tracks.Get(t.GlobalId)
		if ok {
			pt.SatifySelect(sel)
		}
	}
}

func (me *Publication) IsClosed() bool {
	select {
	case <-me.closeCh:
		return true
	default:
		return false
	}
}

func (me *Publication) Close() {
	me.ctx.publications.Del(me.id)
	close(me.closeCh)
}

type PublishedTrack struct {
	pub    *Publication
	mu     sync.Mutex
	track  *Track
	remote *webrtc.TrackRemote
}

func (me *PublishedTrack) Publication() *Publication {
	return me.pub
}

func (me *PublishedTrack) Track() *Track {
	return me.track
}

func (me *PublishedTrack) GlobalId() string {
	return me.track.GlobalId
}

func (me *PublishedTrack) StreamId() string {
	return me.track.StreamId
}

func (me *PublishedTrack) Remote() *webrtc.TrackRemote {
	return me.remote
}

func (me *PublishedTrack) RId() string {
	return me.track.RId
}

func (me *PublishedTrack) BindId() string {
	return me.track.BindId
}

func (pt *PublishedTrack) isBind() bool {
	return pt.remote != nil
}

func (pt *PublishedTrack) Bind() bool {
	ctx := pt.pub.ctx
	peer, err := ctx.MakeSurePeer()
	if err != nil {
		panic(err)
	}
	pt.mu.Lock()
	defer pt.mu.Unlock()
	rt := ctx.findTracksRemoteByMidAndRid(peer, pt.StreamId(), pt.BindId(), pt.RId())
	if rt != nil {
		if rt == pt.remote {
			return false
		}
		pt.remote = rt
		codec := rt.Codec()
		if codec.MimeType != "" {
			pt.track.Codec = NewRTPCodecParameters(&codec)
		}
		pt.track.LocalId = rt.ID()
		ctx.Socket.Emit("published", &PublishedMessage{
			Track: pt.Track(),
		})
		return true
	}
	return false
}

func (me *PublishedTrack) StateWant(want *WantMessage) bool {
	if !want.Pattern.Match(me.track) {
		return false
	}
	me.mu.Lock()
	defer me.mu.Unlock()
	ctx := me.pub.ctx
	peer, err := ctx.MakeSurePeer()
	if err != nil {
		panic(err)
	}
	tr := ctx.findTracksRemoteByMidAndRid(peer, me.StreamId(), me.BindId(), me.RId())
	if tr != nil {
		if me.track.LocalId == "" {
			me.track.LocalId = tr.ID()
		}
		return true
	} else {
		return false
	}
}

func (me *PublishedTrack) SatifySelect(sel *SelectMessage) bool {
	if me.pub.IsClosed() {
		return false
	}
	ctx := me.pub.ctx
	peer, err := ctx.MakeSurePeer()
	if err != nil {
		panic(err)
	}
	me.mu.Lock()
	defer me.mu.Unlock()
	tr := ctx.findTracksRemoteByMidAndRid(peer, me.StreamId(), me.BindId(), me.RId())
	ctx.Sugar().Debugf("Accepting track with codec ", tr.Codec().MimeType)
	if tr != nil {
		go func() {
			r := GetRouter()
			if me.pub.IsClosed() {
				return
			}
			ch := r.PublishTrack(me.GlobalId(), sel.TransportId)
			defer r.RemoveTrack(me.GlobalId())
			buf := make([]byte, PACKET_MAX_SIZE)
			for {
				select {
				case <-me.pub.closeCh:
					p, err := NewEOFPacket(me.GlobalId())
					if err != nil {
						panic(err)
					}
					ch <- p
					return
				default:
					n, _, err := tr.Read(buf)
					if err != nil {
						if err == io.EOF || err == io.ErrClosedPipe || err.Error() == "interceptor is closed" {
							p, err := NewEOFPacket(me.GlobalId())
							if err != nil {
								panic(err)
							}
							ch <- p
							return
						}
						panic(err)
					}
					if n > 0 {
						p, err := NewPacketFromBuf(me.GlobalId(), buf[0:n])
						if err != nil {
							panic(err)
						}
						ch <- p
					}
				}
			}
		}()
		return true
	} else {
		return false
	}
}

type SignalContext struct {
	Socket            *socket.Socket
	AuthInfo          *auth.AuthInfo
	Peer              *webrtc.PeerConnection
	rooms             []socket.Room
	polite            bool
	makingOffer       bool
	pendingCandidates []*CandidateMessage
	cand_mux          sync.Mutex
	peer_mux          sync.Mutex
	neg_mux           sync.Mutex
	subscriptions     *haxmap.Map[string, *Subscription]
	publications      *haxmap.Map[string, *Publication]
	peerClosed        bool
	sdpMsgId          atomic.Int32
	logger            *zap.Logger
	sugar             *zap.SugaredLogger
}

func newSignalContext(socket *socket.Socket, authInfo *auth.AuthInfo) *SignalContext {
	logger := log.MustCreate(log.Logger(), zap.String("tag", "signal-context"), zap.String("id", string(socket.Id())))
	return &SignalContext{
		Socket:        socket,
		AuthInfo:      authInfo,
		subscriptions: haxmap.New[string, *Subscription](),
		publications:  haxmap.New[string, *Publication](),
		logger:        logger,
		sugar:         logger.Sugar(),
	}
}

func (ctx *SignalContext) Logger() *zap.Logger {
	return ctx.logger
}

func (ctx *SignalContext) Sugar() *zap.SugaredLogger {
	return ctx.sugar
}

func (ctx *SignalContext) Authed() bool {
	return ctx.AuthInfo != nil
}

func (ctx *SignalContext) RoomPaterns() []string {
	return ctx.AuthInfo.Rooms
}

func (ctx *SignalContext) Rooms() []socket.Room {
	return ctx.rooms
}

func (ctx *SignalContext) NextSdpMsgId() int {
	return int(ctx.sdpMsgId.Add(2))
}

func (ctx *SignalContext) CurrentSdpMsgId() int {
	return int(ctx.sdpMsgId.Load())
}

func (ctx *SignalContext) Close() {
	ctx.Sugar().Debugf("signal context closing")
	ctx.closePeer()
	ctx.publications.ForEach(func(k string, pub *Publication) bool {
		pub.Close()
		return true
	})
	ctx.subscriptions.ForEach(func(k string, sub *Subscription) bool {
		sub.Close()
		return true
	})
	ctx.Socket.Disconnect(true)
}

func (ctx *SignalContext) AcceptTrack(msg *StateMessage) {
	ctx.subscriptions.ForEach(func(k string, sub *Subscription) bool {
		sub.AcceptTrack(msg)
		return true
	})
}

func findTransiverBySender(peer *webrtc.PeerConnection, sender *webrtc.RTPSender) (*webrtc.RTPTransceiver, int) {
	if transceivers := peer.GetTransceivers(); transceivers != nil {
		for i, transceiver := range transceivers {
			if transceiver.Sender() == sender {
				return transceiver, i
			}
		}
	}
	return nil, -1
}

func getMidFromSender(peer *webrtc.PeerConnection, sender *webrtc.RTPSender) string {
	transceiver, pos := findTransiverBySender(peer, sender)
	if transceiver != nil {
		if transceiver.Mid() != "" {
			return transceiver.Mid()
		} else {
			return fmt.Sprintf("pos:%d", pos)
		}
	} else {
		return ""
	}
}

func (ctx *SignalContext) Subscribe(message *SubscribeMessage) (subId string, err error) {
	err = message.Validate()
	if err != nil {
		return
	}
	switch message.Op {
	case SUB_OP_ADD:
		var loaded = true
		for loaded {
			subId = uuid.NewString()
			_, loaded = ctx.subscriptions.GetOrCompute(subId, func() *Subscription {
				return &Subscription{
					id:       subId,
					reqTypes: message.ReqTypes,
					pattern:  &message.Pattern,
					ctx:      ctx,
				}
			})
		}
	case SUB_OP_UPDATE:
		sub, ok := ctx.subscriptions.Get(message.Id)
		if !ok {
			err = errors.SubNotExist(message.Id)
			return
		}
		sub.UpdatePattern(&message.Pattern)
		subId = sub.id
	case SUB_OP_REMOVE:
		sub, ok := ctx.subscriptions.GetAndDel(message.Id)
		if !ok {
			err = errors.SubNotExist(message.Id)
			return
		}
		sub.Close()
		subId = sub.id
		return
	default:
		panic(errors.ThisIsImpossible().GenCallStacks())
	}

	r := GetRouter()
	want := &WantMessage{
		Pattern:     message.Pattern,
		TransportId: r.id.String(),
	}
	// broadcast to the room to request the sub
	ctx.Socket.To(ctx.Rooms()...).Emit("want", want)
	// broadcast above not include self, the pub in self may satify the sub too. so check self here.
	go ctx.StateWant(want)
	return
}

func (ctx *SignalContext) findTracksRemoteByMidAndRid(peer *webrtc.PeerConnection, sid string, mid string, rid string) *webrtc.TrackRemote {
	var pos int = -1
	var err error
	if strings.HasPrefix(mid, "pos:") {
		pos, err = strconv.Atoi(mid[4:])
		if err != nil {
			panic(err)
		}
	}
	for i, transceiver := range peer.GetTransceivers() {
		if (pos == -1 && transceiver.Mid() == mid) || pos == i {
			r := transceiver.Receiver()
			if r != nil {
				tracks := r.Tracks()
				if len(tracks) > 1 {
					for _, t := range tracks {
						if t.RID() == rid {
							if t.StreamID() == sid {
								return t
							} else {
								return nil
							}
						}
					}
					return nil
				} else if len(tracks) == 0 {
					return nil
				} else {
					track := tracks[0]
					if rid != "" {
						if track.RID() == rid {
							if track.StreamID() == sid {
								return track
							} else {
								return nil
							}
						} else {
							return nil
						}
					} else {
						if track.StreamID() == sid {
							return track
						} else {
							return nil
						}
					}
				}
			} else {
				return nil
			}
		}
	}
	return nil
}

func (ctx *SignalContext) Publish(message *PublishMessage) (pubId string, err error) {
	err = message.Validate()
	if err != nil {
		return
	}
	if message.Op != PUB_OP_ADD {
		err = errors.ThisIsImpossible().GenCallStacks()
		return
	}
	var pub *Publication
	var loaded bool = true
	for loaded {
		pubId = uuid.NewString()
		pub, loaded = ctx.publications.GetOrCompute(pubId, func() *Publication {
			pub := &Publication{
				id:      pubId,
				ctx:     ctx,
				tracks:  haxmap.New[string, *PublishedTrack](),
				closeCh: make(chan struct{}),
			}
			for _, t := range message.Tracks {
				tid := uuid.NewString()
				pub.tracks.Set(tid, &PublishedTrack{
					pub: pub,
					track: &Track{
						Type:     t.Type,
						PubId:    pubId,
						GlobalId: tid,
						BindId:   t.BindId,
						RId:      t.RId,
						StreamId: t.SId,
						Labels:   t.Labels,
					},
				})
			}
			return pub
		})
	}
	pub.Bind()
	return
}

func (ctx *SignalContext) StateWant(message *WantMessage) {
	r := GetRouter()
	ctx.publications.ForEach(func(pubId string, pub *Publication) bool {
		satified := pub.State(message)
		if len(satified) > 0 {
			msg := &StateMessage{
				PubId:  pub.id,
				Tracks: satified,
				Addr:   r.Addr(),
			}
			// to all in room include self
			ctx.Socket.To(ctx.Rooms()...).Emit("state", msg)
			ctx.Socket.Emit("state", msg)
		}
		return true
	})
}

func (ctx *SignalContext) SatifySelect(message *SelectMessage) {
	ctx.publications.ForEach(func(pubId string, pub *Publication) bool {
		pub.SatifySelect(message)
		return true
	})
}

func (ctx *SignalContext) closePeer() {
	if ctx.peerClosed {
		return
	}
	ctx.peer_mux.Lock()
	defer ctx.peer_mux.Unlock()
	if ctx.peerClosed {
		return
	}
	ctx.peerClosed = true
	if ctx.Peer != nil {
		// ignore error
		ctx.Peer.Close()
	}
}

func (ctx *SignalContext) StartNegotiate(peer *webrtc.PeerConnection, msgId int) (err error) {
	ctx.neg_mux.Lock()
	offer, err := peer.CreateOffer(nil)
	if err != nil {
		return
	}
	err = peer.SetLocalDescription(offer)
	if err != nil {
		return
	}
	desc := peer.LocalDescription()
	err = ctx.Socket.Emit("sdp", SdpMessage{
		Type: desc.Type.String(),
		Sdp:  desc.SDP,
		Mid:  msgId,
	})
	return
}

func createPeer() (*webrtc.PeerConnection, error) {
	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		return nil, err
	}
	i := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		return nil, err
	}
	// Register a intervalpli factory
	// This interceptor sends a PLI every 3 seconds. A PLI causes a video keyframe to be generated by the sender.
	// This makes our video seekable and more error resilent, but at a cost of lower picture quality and higher bitrates
	// A real world application should process incoming RTCP packets from viewers and forward them to senders
	intervalPliFactory, err := intervalpli.NewReceiverInterceptor()
	if err != nil {
		panic(err)
	}
	i.Add(intervalPliFactory)
	// Create the API object with the MediaEngine
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(i))
	config := config.Conf().GetWebrtcConfiguration()
	// Create a new RTCPeerConnection
	peer, err := api.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}
	return peer, err
}

func (ctx *SignalContext) MakeSurePeer() (peer *webrtc.PeerConnection, err error) {
	if ctx.peerClosed {
		err = webrtc.ErrConnectionClosed
		return
	}
	ctx.peer_mux.Lock()
	defer ctx.peer_mux.Unlock()
	if ctx.peerClosed {
		err = webrtc.ErrConnectionClosed
		return
	}
	if ctx.Peer != nil {
		peer = ctx.Peer
	} else {
		// only support impolite because pion webrtc not support rollback.
		ctx.polite = false
		peer, err = createPeer()
		if err != nil {
			return
		}
		peer.OnICECandidate(func(i *webrtc.ICECandidate) {
			var err error
			if i == nil {
				err = ctx.Socket.Emit("candidate", CandidateMessage{
					Op: "end",
				})
			} else {
				err = ctx.Socket.Emit("candidate", &CandidateMessage{
					Op:        "add",
					Candidate: i.ToJSON(),
				})
			}
			if err != nil {
				ctx.Sugar().Error(err)
			}
		})
		peer.OnTrack(func(tr *webrtc.TrackRemote, r *webrtc.RTPReceiver) {
			ctx.publications.ForEach(func(pid string, pub *Publication) bool {
				if pub.Bind() {
					return false
				} else {
					return true
				}
			})
		})

		lastState := peer.ConnectionState()
		peer.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) {
			ctx.Sugar().Info("peer connect state changed from ", lastState, " to ", pcs)
			lastState = pcs
			if pcs == webrtc.PeerConnectionStateClosed {
				ctx.Close()
			}
		})
		lastSignalingState := peer.SignalingState()
		peer.OnSignalingStateChange(func(ss webrtc.SignalingState) {
			ctx.Sugar().Info("peer signaling state changed from ", lastSignalingState, " to ", ss)
			lastSignalingState = ss
			if ss == webrtc.SignalingStateStable {
				ctx.neg_mux.TryLock()
				ctx.neg_mux.Unlock()
			}
		})
		ctx.Peer = peer
	}
	return
}

func (ctx *SignalContext) HasRoomRight(room string) bool {
	for _, p := range ctx.RoomPaterns() {
		if MatchRoom(p, room) {
			return true
		}
	}
	return false
}

func (ctx *SignalContext) JoinRoom(rooms ...string) error {
	var s_rooms []socket.Room
	if len(rooms) == 0 {
		for _, p := range ctx.RoomPaterns() {
			if !strings.Contains(p, "*") {
				s_rooms = append(s_rooms, socket.Room(p))
			}
		}
		if len(s_rooms) == 0 {
			return errors.InvalidParam("room is not provided")
		}
	} else {
		for _, room := range rooms {
			if !ctx.HasRoomRight(room) {
				return errors.RoomNoRight(room)
			}
		}
		s_rooms = make([]socket.Room, len(rooms))
		for i, room := range rooms {
			s_rooms[i] = socket.Room(room)
		}
	}
	ctx.Socket.Join(s_rooms...)
	return nil
}

func (ctx *SignalContext) LeaveRoom(rooms ...string) {
	for _, room := range rooms {
		ctx.Socket.Leave(socket.Room(room))
	}
}

func GetSingalContext(s *socket.Socket) *SignalContext {
	d := s.Data()
	if d != nil {
		return s.Data().(*SignalContext)
	} else {
		return nil
	}
}

func SetAuthInfo(s *socket.Socket, authInfo *auth.AuthInfo) {
	raw := s.Data()
	if raw == nil {
		ctx := newSignalContext(s, authInfo)
		s.SetData(ctx)
	} else {
		raw.(*SignalContext).AuthInfo = authInfo
	}
}
