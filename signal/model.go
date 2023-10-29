package signal

import (
	"fmt"
	"net"
	"sync"
	"unsafe"

	"github.com/pion/webrtc/v4"
	"github.com/vipcxj/conference.go/auth"
	"github.com/vipcxj/conference.go/errors"
	"github.com/zishang520/socket.io/v2/socket"
)

type ProxyConn struct {
	Rtp net.UDPConn
	Rtcp net.TCPConn
}

type SingalContext struct {
	Socket   *socket.Socket
	AuthInfo *auth.AuthInfo
	Peer     *webrtc.PeerConnection
	Conns    map[socket.SocketId]*ProxyConn
	peer_mux sync.Mutex
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

func (ctx *SingalContext) MakeSurePeer() (*webrtc.PeerConnection, error) {
	if ctx.Peer != nil {
		return ctx.Peer, nil
	} else {
		ctx.peer_mux.Lock()
		defer ctx.peer_mux.Unlock()
		var peer *webrtc.PeerConnection
		var err error
		if peer = ctx.Peer; peer == nil {
			peer, err = webrtc.NewPeerConnection(webrtc.Configuration{})
			if err != nil {
				return nil, err
			}
			peer.OnTrack(func(tr *webrtc.TrackRemote, r *webrtc.RTPReceiver) {
				msg := StreamMessage{
					Op: "add",
					Stream: Stream{
						Id:       tr.ID(),
						StreamId: tr.StreamID(),
					},
				}
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
			Socket:   s,
			AuthInfo: authInfo,
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
