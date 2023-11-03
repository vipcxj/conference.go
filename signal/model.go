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
	id     string
	mu     sync.Mutex
	closed bool
	tracks []*PublishedTrack
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

func (me *PublishedTrack) BindId() string {
	return me.track.BindId
}

func (me *PublishedTrack) Satify(pattern *PublicationPattern) {

}

type SingalContext struct {
	Socket              *socket.Socket
	AuthInfo            *auth.AuthInfo
	Peer                *webrtc.PeerConnection
	polite              bool
	makingOffer         bool
	pendingCandidates   []*CandidateMessage
	cand_mux            sync.Mutex
	pendingTracks       map[string]*webrtc.TrackRemote
	track_mux           sync.Mutex
	peer_mux            sync.Mutex
	sub_mux             sync.RWMutex
	subscriptions       *haxmap.Map[string, *Subscription]
	publications        *haxmap.Map[string, *Publication]
	gid2publishedTracks *haxmap.Map[string, *PublishedTrack]
	gid2pub             map[string]bool
	pub_mux             sync.Mutex
}

func newSignalContext(socket *socket.Socket, authInfo *auth.AuthInfo) *SingalContext {
	return &SingalContext{
		Socket:              socket,
		AuthInfo:            authInfo,
		gid2publishedTracks: haxmap.New[string, *PublishedTrack](),
		gid2pub:             map[string]bool{},
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

func (ctx *SingalContext) findRemoteTrack(gid string) (*webrtc.TrackRemote, error) {
	pub, ok := ctx.gid2publishedTracks.Get(gid)
	if ok {
		peer, err := ctx.MakeSurePeer()
		if err != nil {
			return nil, err
		}
		var track *webrtc.TrackRemote
		for _, _receiver := range peer.GetReceivers() {
			_tr := _receiver.Track()
			if _tr != nil && _tr.ID() == pub.Id && _tr.StreamID() == pub.StreamId {
				track = _tr
				break
			}
		}
		if track == nil {
			return nil, nil
		}
		return track, nil
	} else {
		return nil, nil
	}
}

func (ctx *SingalContext) TryPublish(gid string, transportId string) {
	ctx.pub_mux.Lock()
	defer ctx.pub_mux.Unlock()
	_, ok := ctx.gid2pub[gid]
	if ok {
		return
	}
	ctx.gid2pub[gid] = true
	go func() {
		defer func() {
			ctx.pub_mux.Lock()
			defer ctx.pub_mux.Unlock()
			delete(ctx.gid2pub, gid)
		}()
		track, err := ctx.findRemoteTrack(gid)
		if err != nil {
			fmt.Println(err)
			return
		}
		if track == nil {
			return
		}
		router := GetRouter()
		ch := router.PublishTrack(gid, transportId)
		if transportId != router.id.String() {
			ctx.Socket.To(ctx.Rooms()...).Emit("state", &StateMessage{
				Tracks: []*Track{{
					GlobalId: gid,
					LocalId:  track.ID(),
					StreamId: track.StreamID(),
				}},
				Addr: router.Addr(),
			})
		} else {
			router.SubscribeTrackIfWanted(gid, track.ID(), track.StreamID(), router.Addr())
		}
		buf := make([]byte, PACKET_MAX_SIZE)
		for {
			n, _, err := track.Read(buf)
			if err != nil {
				if err == io.EOF {
					return
				}
				panic(err)
			}
			p, err := NewPacketFromBuf(gid, buf[0:n])
			if err != nil {
				panic(err)
			}
			ch <- p
		}
	}()
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

func (ctx *SingalContext) Publish(message *PublishMessage) error {
	err := message.Validate()
	if err != nil {
		return err
	}
}

func (ctx *SingalContext) SatifyWant(message *WantMessage) {

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
				gid := uuid.NewString()
				if err != nil {
					panic(err)
				}
				st := Track{
					GlobalId: gid,
					LocalId:  tr.ID(),
					StreamId: tr.StreamID(),
				}
				msg := TrackMessage{
					Op:     "add",
					Tracks: []*Track{&st},
				}
				ctx.gid2publishedTracks.Set(gid, &st)
				fmt.Println("On track with stream id ", tr.StreamID(), " and id ", tr.ID())
				ctx.Socket.To(ctx.Rooms()...).Emit("stream", msg)
				ctx.Socket.Emit("stream", msg)
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
