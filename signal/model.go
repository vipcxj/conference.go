package signal

import (
	"fmt"
	"io"
	"net"
	"sync"
	"unsafe"

	"github.com/alphadose/haxmap"
	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
	"github.com/vipcxj/conference.go/auth"
	"github.com/vipcxj/conference.go/errors"
	"github.com/zishang520/socket.io/v2/socket"
)

type ProxyConn struct {
	Rtp  net.UDPConn
	Rtcp net.TCPConn
}

type SingalContext struct {
	Socket            *socket.Socket
	AuthInfo          *auth.AuthInfo
	Peer              *webrtc.PeerConnection
	polite            bool
	makingOffer       bool
	PendingCandidates []*CandidateMessage
	cand_mux          sync.Mutex
	peer_mux          sync.Mutex
	gid2tracks        *haxmap.Map[string, *Track]
	gid2pub           map[string]bool
	pub_mux           sync.Mutex
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


func (ctx *SingalContext) Subscribed(sub *Subscribation, gid string) {
	defer errors.Ignore(webrtc.ErrConnectionClosed)
	peer, err := ctx.MakeSurePeer()
	if err != nil {
		panic(err)
	}
	sender, err := peer.AddTrack(sub.track)
	track := sender.Track()
	if err != nil {
		panic(err)
	}
	sub.mu.Lock()
	closer := func() {
		defer errors.Ignore(webrtc.ErrConnectionClosed)
		err := peer.RemoveTrack(sender)
		if err != nil {
			panic(err)
		}
	}
	sub.closers = append(sub.closers, closer)
	sub.mu.Unlock()
	err = ctx.Socket.Emit("subscribed", &SubscribedMessage{
		Tracks: []*Track{{
			GlobalId: gid,
			Id: track.ID(),
			StreamId: track.StreamID(),
		}},
	})
	if err != nil {
		panic(err)
	}
}

func (ctx *SingalContext) getReceiver(gid string) (*webrtc.TrackRemote, *webrtc.RTPReceiver, error) {
	tr, ok := ctx.gid2tracks.Get(gid)
	if ok {
		peer, err := ctx.MakeSurePeer()
		if err != nil {
			return nil, nil, err
		}
		var receiver *webrtc.RTPReceiver
		var track *webrtc.TrackRemote
		for _, _receiver := range peer.GetReceivers() {
			_tr := _receiver.Track()
			if _tr != nil && _tr.ID() == tr.Id && _tr.StreamID() == tr.StreamId {
				track = _tr
				receiver = _receiver
				break
			}
		}
		if track == nil {
			return nil, nil, nil
		}
		return track, receiver, nil
	} else {
		return nil, nil, nil
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
		track, _, err := ctx.getReceiver(gid)
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
					Id:       track.ID(),
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

func (ctx *SingalContext) Subscribe(tracks []*Track) error {
	r := GetRouter()
	r.WantTracks(tracks, ctx)
	ctx.Socket.To(ctx.Rooms()...).Emit("want", &WantMessage{
		Tracks:      tracks,
		TransportId: r.id.String(),
	})
	for _, track := range tracks {
		ctx.TryPublish(track.GlobalId, r.id.String())
	}
	return nil
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
					Id:       tr.ID(),
					StreamId: tr.StreamID(),
				}
				msg := TrackMessage{
					Op:     "add",
					Tracks: []*Track{&st},
				}
				ctx.gid2tracks.Set(gid, &st)
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
		ctx := &SingalContext{
			Socket:     s,
			AuthInfo:   authInfo,
			gid2tracks: haxmap.New[string, *Track](),
			gid2pub:    map[string]bool{},
		}
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
