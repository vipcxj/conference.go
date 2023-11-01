package signal

import (
	"fmt"

	"github.com/pion/webrtc/v4"
	"github.com/vipcxj/conference.go/errors"
	"github.com/zishang520/socket.io/v2/socket"
)

func InitSignal(s *socket.Socket) error {
	ctx := GetSingalContext(s)
	if ctx == nil {
		return errors.FatalError("unable to find the signal context")
	}
	s.On("sdp", func(args ...any) {
		defer CatchFatalAndClose(ctx.Socket, "on sdp")
		msg := SdpMessage{}
		ark, err := parseArgs(&msg, args...)
		doArk(ark, nil)
		if err != nil {
			panic(err)
		}
		peer, err := ctx.MakeSurePeer()
		if err != nil {
			panic(err)
		}
		offerCollision := msg.Type == webrtc.SDPTypeOffer.String() && (ctx.makingOffer || peer.SignalingState() != webrtc.SignalingStateStable)
		if ctx.polite && offerCollision {
			return
		}
		err = peer.SetRemoteDescription(webrtc.SessionDescription{
			Type: webrtc.NewSDPType(msg.Type),
			SDP:  msg.Sdp,
		})
		if err != nil {
			panic(err)
		}
		err = processPendingCandidateMsg(s, peer, ctx)
		if err != nil {
			panic(err)
		}
		if msg.Type == webrtc.SDPTypeOffer.String() {
			answer, err := peer.CreateAnswer(nil)
			if err != nil {
				panic(err)
			}
			err = peer.SetLocalDescription(answer)
			if err != nil {
				panic(err)
			}
			err = s.Emit("sdp", SdpMessage{
				Type: answer.Type.String(),
				Sdp:  answer.SDP,
			})
			if err != nil {
				panic(err)
			}
		}
	})
	s.On("candidate", func(args ...any) {
		defer CatchFatalAndClose(ctx.Socket, "candidate")
		msg := CandidateMessage{}
		ark, err := parseArgs(&msg, args...)
		doArk(ark, nil)
		if err != nil {
			panic(err)
		}
		peer, err := ctx.MakeSurePeer()
		if err != nil {
			panic(err)
		}
		if peer.RemoteDescription() == nil {
			ctx.cand_mux.Lock()
			if peer.RemoteDescription() == nil {
				defer ctx.cand_mux.Unlock()
				ctx.PendingCandidates = append(ctx.PendingCandidates, &msg)
				return
			} else {
				ctx.cand_mux.Unlock()
			}
		}
		err = processCandidateMsg(s, peer, &msg)
		if err != nil {
			panic(err)
		}
	})
	s.On("subscribe", func(args ...any) {
		defer CatchFatalAndClose(ctx.Socket, "subscribe")
		msg := SubscribeMessage{}
		ark, err := parseArgs(&msg, args...)
		doArk(ark, nil)
		if err != nil {
			panic(err)
		}
		ctx.Subscribe(msg.Tracks)
	})
	s.On("state", func(args ...any) {
		defer CatchFatalAndClose(ctx.Socket, "subscribe")
		msg := StateMessage{}
		ark, err := parseArgs(&msg, args...)
		doArk(ark, nil)
		if err != nil {
			panic(err)
		}
		r := GetRouter()
		for _, track := range msg.Tracks {
			r.SubscribeTrackIfWanted(track.GlobalId, track.Id, track.StreamId, msg.Addr)
		}
	})
	s.On("want", func(args ...any) {
		defer CatchFatalAndClose(ctx.Socket, "want")
		msg := WantMessage{}
		ark, err := parseArgs(&msg, args...)
		doArk(ark, nil)
		if err != nil {
			panic(err)
		}
		for _, track := range msg.Tracks {
			ctx.TryPublish(track.GlobalId, msg.TransportId)
		}
	})
	return nil
}

func processPendingCandidateMsg(s *socket.Socket, peer *webrtc.PeerConnection, ctx *SingalContext) error {
	ctx.cand_mux.Lock()
	defer ctx.cand_mux.Unlock()
	var err error
	for _, msg := range ctx.PendingCandidates {
		if e := processCandidateMsg(s, peer, msg); e != nil {
			err = e
		}
	}
	ctx.PendingCandidates = nil
	return err
}

func processCandidateMsg(s *socket.Socket, peer *webrtc.PeerConnection, msg *CandidateMessage) error {
	var err error
	if msg.Op == "add" {
		fmt.Println("Received candidate ", msg.Candidate.Candidate)
		err = peer.AddICECandidate(msg.Candidate)
	} else {
		fmt.Println("Received candidate completed")
		err = peer.AddICECandidate(webrtc.ICECandidateInit{})
	}
	return err
}
