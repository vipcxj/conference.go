package signal

import (
	"github.com/pion/webrtc/v4"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/log"
	"github.com/zishang520/socket.io/v2/socket"
)

func InitSignal(s *socket.Socket) error {
	ctx := GetSingalContext(s)
	if ctx == nil {
		return errors.FatalError("unable to find the signal context")
	}
	auth := ctx.AuthInfo
	if auth == nil {
		return errors.ThisIsImpossible().GenCallStacks()
	}
	if auth.AutoJoin {
		if err := ctx.JoinRoom(); err != nil {
			return err
		}
	}
	s.On("disconnect", func(args ...any) {
		reason := args[0].(string)
		if reason == "server namespace disconnect" || reason == "client namespace disconnect" || reason == "server shutting down" {
			ctx.Close()
		}
	})
	s.On("join", func(args ...any) {
		msg := JoinMessage{}
		ark, err := parseArgs(&msg, args...)
		defer FinallyResponse(ctx.Socket, ark, nil, "join")
		if err != nil {
			panic(err)
		}
		err = ctx.JoinRoom(msg.Rooms...)
		if err != nil {
			panic(err)
		}
	})
	s.On("leave", func(args ...any) {
		msg := LeaveMessage{}
		ark, err := parseArgs(&msg, args...)
		defer FinallyResponse(ctx.Socket, ark, nil, "leave")
		if err != nil {
			panic(err)
		}
		ctx.LeaveRoom(msg.Rooms...)
	})
	s.On("sdp", func(args ...any) {
		msg := SdpMessage{}
		ark, err := parseArgs(&msg, args...)
		defer FinallyResponse(ctx.Socket, ark, nil, "sdp")
		if err != nil {
			panic(err)
		}
		if msg.Type == webrtc.SDPTypeAnswer.String() && ctx.CurrentSdpMsgId() != msg.Mid {
			log.Sugar().Warn("received unmatched sdp answer msg with msg id ", msg.Mid)
			return
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
				Mid:  msg.Mid,
			})
			if err != nil {
				panic(err)
			}
		}
	})
	s.On("candidate", func(args ...any) {
		msg := CandidateMessage{}
		ark, err := parseArgs(&msg, args...)
		defer FinallyResponse(ctx.Socket, ark, nil, "candidate")
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
				ctx.pendingCandidates = append(ctx.pendingCandidates, &msg)
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
	s.On("publish", func(args ...any) {
		msg := PublishMessage{}
		ark, err := parseArgs(&msg, args...)
		arkArgs := make([]any, 1)
		defer FinallyResponse(ctx.Socket, ark, arkArgs, "publish")
		if err != nil {
			panic(err)
		}
		pubId, err := ctx.Publish(&msg)
		if err != nil {
			panic(err)
		}
		arkArgs[0] = &PublishResultMessage{
			Id: pubId,
		}
	})
	s.On("subscribe", func(args ...any) {
		msg := SubscribeMessage{}
		ark, err := parseArgs(&msg, args...)
		arkArgs := make([]any, 1)
		defer FinallyResponse(ctx.Socket, ark, arkArgs, "subscribe")
		if err != nil {
			panic(err)
		}
		subId, err := ctx.Subscribe(&msg)
		if err != nil {
			panic(err)
		}
		arkArgs[0] = &SubscribeResultMessage{
			Id: subId,
		}
	})
	s.On("state", func(args ...any) {
		msg := StateMessage{}
		ark, err := parseArgs(&msg, args...)
		defer FinallyResponse(ctx.Socket, ark, nil, "subscribe")
		if err != nil {
			panic(err)
		}
		ctx.AcceptTrack(&msg)
	})
	s.On("want", func(args ...any) {
		msg := WantMessage{}
		ark, err := parseArgs(&msg, args...)
		defer FinallyResponse(ctx.Socket, ark, nil, "want")
		if err != nil {
			panic(err)
		}
		ctx.StateWant(&msg)
	})
	s.On("select", func(args ...any) {
		msg := SelectMessage{}
		ark, err := parseArgs(&msg, args...)
		defer FinallyResponse(ctx.Socket, ark, nil, "select")
		if err != nil {
			panic(err)
		}
		ctx.SatifySelect(&msg)
	})
	return nil
}

func processPendingCandidateMsg(s *socket.Socket, peer *webrtc.PeerConnection, ctx *SingalContext) error {
	ctx.cand_mux.Lock()
	defer ctx.cand_mux.Unlock()
	var err error
	for _, msg := range ctx.pendingCandidates {
		if e := processCandidateMsg(s, peer, msg); e != nil {
			err = e
		}
	}
	ctx.pendingCandidates = nil
	return err
}

func processCandidateMsg(s *socket.Socket, peer *webrtc.PeerConnection, msg *CandidateMessage) error {
	var err error
	if msg.Op == "add" {
		log.Sugar().Debugf("Received candidate ", msg.Candidate.Candidate)
		err = peer.AddICECandidate(msg.Candidate)
	} else {
		log.Sugar().Debugf("Received candidate completed")
		err = peer.AddICECandidate(webrtc.ICECandidateInit{})
	}
	return err
}
