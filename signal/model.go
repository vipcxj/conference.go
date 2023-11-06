package signal

import (
	"fmt"
	"io"
	"sync"
	"unsafe"

	"github.com/alphadose/haxmap"
	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
	"github.com/vipcxj/conference.go/auth"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/utils"
	"github.com/zishang520/socket.io/v2/socket"
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
	st.sub.ctx.Peer.RemoveTrack(st.sender)
	delete(st.sub.acceptedTrack, st.pubTrack.GlobalId)
}

type Subscription struct {
	id            string
	idBytes       []byte
	mu            sync.Mutex
	closed        bool
	pattern       *PublicationPattern
	ctx           *SingalContext
	acceptedTrack map[string]*SubscribedTrack
}

func (s *Subscription) newUUID(id string) string {
	newId := uuid.MustParse(id)
	if s.idBytes == nil {
		parsed := uuid.MustParse(s.id)
		s.idBytes = parsed[:]
	}
	utils.XOrBytes(newId[:], s.idBytes, newId[:])
	return newId.String()
}

func (s *Subscription) AcceptTrack(track *Track, addr string) (subscribedTrack *SubscribedTrack, isNew bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, false
	}
	if s.acceptedTrack == nil {
		s.acceptedTrack = map[string]*SubscribedTrack{}
	}
	subscribedTrack, ok := s.acceptedTrack[track.GlobalId]
	if ok {
		if s.pattern.Match(track) {
			return subscribedTrack, false
		} else {
			subscribedTrack.Remove()
			return nil, false
		}
	} else if !s.pattern.Match(track) {
		return nil, false
	}
	router := GetRouter()
	// accept track from addr
	router.makeSureExternelConn(addr)
	accepter := router.AcceptTrack(track)
	// accept track
	peer, err := s.ctx.MakeSurePeer()
	if err != nil {
		panic(err)
	}
	sender, err := peer.AddTrack(accepter.track)
	if err != nil {
		panic(err)
	}
	mid := getMidFromSender(peer, sender)
	sid := s.newUUID(track.StreamId)
	subscribedTrack = &SubscribedTrack{
		sub:      s,
		sid:      sid,
		pubTrack: track,
		bindId:   mid,
		accepter: accepter,
		sender:   sender,
		labels:   track.Labels,
	}
	s.acceptedTrack[track.GlobalId] = subscribedTrack
	accepter.BindSub(s)
	respMsg := &SubscribedMessage{
		SubId: s.id,
		Track: *track,
	}
	respMsg.Track.LocalId = sender.Track().ID()
	respMsg.Track.BindId = mid
	respMsg.Track.StreamId = sid
	s.ctx.Socket.Emit("subscribed", respMsg)
	return subscribedTrack, true
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
	for _, track := range s.acceptedTrack {
		track.Remove()
	}
	s.acceptedTrack = nil
}

type Publication struct {
	id      string
	mu      sync.Mutex
	closeCh chan struct{}
	ctx     *SingalContext
	tracks  *haxmap.Map[string, *PublishedTrack]
}

func (me *Publication) Bind() bool {
	done := false
	me.tracks.ForEach(func(gid string, pt *PublishedTrack) bool {
		if pt.Bind() {
			*&done = true
			return false
		} else {
			return true
		}
	})
	return done
}

func (me *Publication) Satify(want *WantMessage) []*Track {
	satifiedMap := map[string]*PublishedTrack{}
	me.tracks.ForEach(func(gid string, pt *PublishedTrack) bool {
		if pt.Satify(want) {
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
		ctx.Socket.Emit("published", &PublishedMessage{
			Track: pt.Track(),
		})
		return true
	}
	return false
}

func (me *PublishedTrack) Satify(want *WantMessage) bool {
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
		go func() {
			r := GetRouter()
			ch := r.PublishTrack(me.GlobalId(), want.TransportId)
			defer r.RemoveTrack(me.GlobalId())
			buf := make([]byte, PACKET_MAX_SIZE)
			for {
				select {
				case <-me.pub.closeCh:
					return
				default:
					n, _, err := tr.Read(buf)
					if err != nil {
						if err == io.EOF {
							return
						}
						panic(err)
					}
					p, err := NewPacketFromBuf(me.GlobalId(), buf[0:n])
					if err != nil {
						panic(err)
					}
					ch <- p
				}
			}
		}()
		return true
	} else {
		return false
	}
}

type SingalContext struct {
	Socket            *socket.Socket
	AuthInfo          *auth.AuthInfo
	Peer              *webrtc.PeerConnection
	polite            bool
	makingOffer       bool
	pendingCandidates []*CandidateMessage
	cand_mux          sync.Mutex
	pendingTracks     map[string]*webrtc.TrackRemote
	track_mux         sync.Mutex
	peer_mux          sync.Mutex
	sub_mux           sync.RWMutex
	subscriptions     *haxmap.Map[string, *Subscription]
	publications      *haxmap.Map[string, *Publication]
	gid2pub           map[string]bool
	pub_mux           sync.Mutex
}

func newSignalContext(socket *socket.Socket, authInfo *auth.AuthInfo) *SingalContext {
	return &SingalContext{
		Socket:   socket,
		AuthInfo: authInfo,
		gid2pub:  map[string]bool{},
	}
}

func (ctx *SingalContext) Authed() bool {
	return ctx.AuthInfo != nil
}

func (ctx *SingalContext) Rooms() []socket.Room {
	if ctx.AuthInfo != nil {
		if rooms := ctx.AuthInfo.Rooms; len(rooms) > 0 {
			return unsafe.Slice((*socket.Room)(unsafe.Pointer(&rooms[0])), len(rooms))
		} else {
			return []socket.Room{}
		}
	} else {
		return []socket.Room{}
	}
}

func (ctx *SingalContext) AcceptTrack(track *Track, addr string) {
	ctx.subscriptions.ForEach(func(k string, sub *Subscription) bool {
		sub.AcceptTrack(track, addr)
		return true
	})
}

func findTransiverBySender(peer *webrtc.PeerConnection, sender *webrtc.RTPSender) *webrtc.RTPTransceiver {
	if transceivers := peer.GetTransceivers(); transceivers != nil {
		for _, transceiver := range transceivers {
			if transceiver.Sender() == sender {
				return transceiver
			}
		}
	}
	return nil
}

func getMidFromSender(peer *webrtc.PeerConnection, sender *webrtc.RTPSender) string {
	transceiver := findTransiverBySender(peer, sender)
	if transceiver != nil {
		return transceiver.Mid()
	} else {
		return ""
	}
}

func (ctx *SingalContext) Subscribe(message *SubscribeMessage) error {
	err := message.Validate()
	if err != nil {
		return err
	}
	switch message.Op {
	case SUB_OP_ADD:
		var loaded = true
		for loaded {
			subId := uuid.NewString()
			_, loaded = ctx.subscriptions.GetOrCompute(subId, func() *Subscription {
				return &Subscription{
					id:      subId,
					pattern: &message.Pattern,
					ctx:     ctx,
				}
			})
		}
	case SUB_OP_UPDATE:
		sub, ok := ctx.subscriptions.Get(message.Id)
		if !ok {
			return errors.SubNotExist(message.Id)
		}
		sub.UpdatePattern(&message.Pattern)
	case SUB_OP_REMOVE:
		sub, ok := ctx.subscriptions.GetAndDel(message.Id)
		if !ok {
			return errors.SubNotExist(message.Id)
		}
		sub.Close()
		return nil
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
	ctx.SatifyWant(want)
	return nil
}

func (ctx *SingalContext) findTracksRemoteByMidAndRid(peer *webrtc.PeerConnection, sid string, mid string, rid string) *webrtc.TrackRemote {
	for _, transceiver := range peer.GetTransceivers() {
		if transceiver.Mid() == mid {
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

func (ctx *SingalContext) Publish(message *PublishMessage) error {
	err := message.Validate()
	if err != nil {
		return err
	}
	if message.Op != PUB_OP_ADD {
		return errors.ThisIsImpossible().GenCallStacks()
	}
	var pub *Publication
	var loaded bool = true
	for loaded {
		pubId := uuid.NewString()
		pub, loaded = ctx.publications.GetOrCompute(pubId, func() *Publication {
			pub := &Publication{
				id:     pubId,
				ctx:    ctx,
				tracks: haxmap.New[string, *PublishedTrack](),
			}
			for _, t := range message.Tracks {
				tid := uuid.NewString()
				pub.tracks.Set(tid, &PublishedTrack{
					pub: pub,
					track: &Track{
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
	return nil
}

func (ctx *SingalContext) SatifyWant(message *WantMessage) {
	var satified []*Track
	ctx.publications.ForEach(func(pubId string, pub *Publication) bool {
		satified = append(satified, pub.Satify(message)...)
		return true
	})
	if len(satified) > 0 {
		r := GetRouter()
		msg := &StateMessage{
			Tracks: satified,
			Addr:   r.Addr(),
		}
		// to all in room include self
		ctx.Socket.To(ctx.Rooms()...).Emit("state", msg)
		ctx.Socket.Emit("state", msg)
	}
}

func (ctx *SingalContext) MakeSurePeer() (*webrtc.PeerConnection, error) {
	if ctx.Peer != nil {
		return ctx.Peer, nil
	} else {
		ctx.peer_mux.Lock()
		defer ctx.peer_mux.Unlock()
		var peer *webrtc.PeerConnection
		var err error
		if peer = ctx.Peer; peer == nil {
			// only support impolite because pion webrtc not support rollback.
			ctx.polite = false
			peer, err = webrtc.NewPeerConnection(webrtc.Configuration{})
			if err != nil {
				return nil, err
			}
			peer.OnNegotiationNeeded(func() {
				defer CatchFatalAndClose(ctx.Socket, "on negotiation")
				ctx.makingOffer = true
				defer func() {
					ctx.makingOffer = false
				}()
				offer, err := peer.CreateOffer(nil)
				if err != nil {
					panic(err)
				}
				err = peer.SetLocalDescription(offer)
				if err != nil {
					panic(err)
				}
				desc := peer.LocalDescription()
				ctx.Socket.Emit("sdp", SdpMessage{
					Type: desc.Type.String(),
					Sdp:  desc.SDP,
				})
			})
			peer.OnICECandidate(func(i *webrtc.ICECandidate) {
				defer CatchFatalAndClose(ctx.Socket, "on candidate")
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
					panic(err)
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

			lastState := []webrtc.PeerConnectionState{peer.ConnectionState()}
			peer.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) {
				fmt.Println("peer connect state changed to ", pcs, " from ", lastState[0])
				lastState[0] = pcs
			})
			ctx.Peer = peer
		}
		return peer, nil
	}
}

func GetSingalContext(s *socket.Socket) *SingalContext {
	d := s.Data()
	if d != nil {
		return s.Data().(*SingalContext)
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
		raw.(*SingalContext).AuthInfo = authInfo
	}
}

func JoinRoom(s *socket.Socket) error {
	ctx := GetSingalContext(s)
	if ctx == nil {
		return errors.Unauthorized("")
	}
	rooms := ctx.Rooms()
	if len(rooms) == 0 {
		return errors.InvalidParam("room is not provided")
	}
	s.Join(rooms...)
	return nil
}
