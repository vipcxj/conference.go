package signal

import (
	"fmt"
	"net/http"

	"github.com/pion/webrtc/v4"
	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/errors"
	"github.com/zishang520/socket.io/v2/socket"
)

const CLOSE_CALLBACK_PREFIX = "/conference/close"

func CloseCallback(conf *config.ConferenceConfigure, id string) string {
	return fmt.Sprintf("%v%v/%v", conf.SignalExternalAddress(), CLOSE_CALLBACK_PREFIX, id)
}

func InitSignal(s *socket.Socket) (*SignalContext, error) {
	ctx := GetSingalContext(s)
	if ctx == nil {
		return ctx, errors.FatalError("unable to find the signal context")
	}
	ctx.Sugar().Debugf("initializing the signal context")
	if ctx.inited {
		ctx.Sugar().Debugf("the signal context already initialized, return directly")
		return ctx, nil
	}
	ctx.inited_mux.Lock()
	if ctx.inited {
		ctx.inited_mux.Unlock()
		ctx.Sugar().Debugf("the signal context already initialized, return directly")
		return ctx, nil
	}
	ctx.inited = true
	defer ctx.inited_mux.Unlock()
	auth := ctx.AuthInfo
	if auth == nil {
		return ctx, errors.ThisIsImpossible().GenCallStacks(0)
	}
	if auth.AutoJoin {
		if err := ctx.JoinRoom(); err != nil {
			return ctx, err
		}
	}
	cbOnStart := NewConferenceCallback("setup", ctx.Global.Conf().Callback.OnStart, ctx)
	st, err := cbOnStart.Call(ctx)
	if err != nil {
		msg := fmt.Sprintf("start callback invoked failed with error %v, so close the singal context.", err)
		ctx.Sugar().Warn(msg)
		ctx.Close(true)
		return nil, errors.FatalError(msg)
	}
	if st != 0 && st != http.StatusOK {
		msg := fmt.Sprintf("start callback return status code %v, so close the singal context.", st)
		ctx.Sugar().Warn(msg)
		ctx.Close(true)
		return nil, errors.FatalError(msg)
	}
	ctx.Metrics().OnSignalConnectStart(ctx)
	ctx.SetCloseCallback(NewConferenceCallback("close", ctx.Global.Conf().Callback.OnClose, ctx))
	ctx.Global.RegisterSignalContext(ctx)
	s.On("disconnect", func(args ...any) {
		ctx.Metrics().OnSignalConnectClose(ctx)
		reason := args[0].(string)
		ctx.Sugar().Infof("socket disconnect because %s", reason)
		if reason != "ping timeout" {
			ctx.Sugar().Infof("close the socket")
			ctx.Close(false)
		}
	})
	s.On("join", func(args ...any) {
		msg := JoinMessage{}
		ark, err := parseArgs(&msg, args...)
		defer FinallyResponse(ctx.Socket, ark, nil, "join", false)
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
		defer FinallyResponse(ctx.Socket, ark, nil, "leave", false)
		if err != nil {
			panic(err)
		}
		ctx.LeaveRoom(msg.Rooms...)
	})
	s.On("sdp", func(args ...any) {
		msg := SdpMessage{}
		ark, err := parseArgs(&msg, args...)
		defer FinallyResponse(ctx.Socket, ark, nil, "sdp", true)
		if err != nil {
			panic(err)
		}
		peer, err := ctx.MakeSurePeer()
		if err != nil {
			panic(err)
		}
		ctx.Sugar().Infof("accept %s sdp msg with id %d", msg.Type, msg.Mid)
		go func() {
			defer FinallyResponse(ctx.Socket, ark, nil, "sdp", false)
			locked := ctx.neg_mux.TryLock()
			if locked {
				ctx.Sugar().Debug("neg mux locked")
			}
			if msg.Type == webrtc.SDPTypeAnswer.String() && ctx.CurrentSdpMsgId() != msg.Mid {
				ctx.Sugar().Warn("received unmatched sdp answer msg with msg id ", msg.Mid)
				return
			}
			err = peer.SetRemoteDescription(webrtc.SessionDescription{
				Type: webrtc.NewSDPType(msg.Type),
				SDP:  msg.Sdp,
			})
			if err != nil {
				ctx.Sugar().Warn(err)
				return
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
				desc := peer.LocalDescription()
				err = s.Emit("sdp", SdpMessage{
					Type: desc.Type.String(),
					Sdp:  desc.SDP,
					Mid:  msg.Mid,
				})
				if err != nil {
					panic(err)
				}
			} else if msg.Type != webrtc.SDPTypeAnswer.String() {
				panic(errors.FatalError("the sdp type %v is not supported", msg.Type))
			}
		}()
	})
	s.On("candidate", func(args ...any) {
		msg := CandidateMessage{}
		ark, err := parseArgs(&msg, args...)
		defer FinallyResponse(ctx.Socket, ark, nil, "candidate", false)
		if err != nil {
			panic(err)
		}
		peer, err := ctx.MakeSurePeer()
		if err != nil {
			panic(err)
		}
		ctx.Sugar().Infof("received candidate %v", msg.Candidate.Candidate)
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
		defer FinallyResponse(ctx.Socket, ark, arkArgs, "publish", false)
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
		defer FinallyResponse(ctx.Socket, ark, arkArgs, "subscribe", false)
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
	ctx.Messager().OnState(ctx.Id, ctx.AcceptTrack, ctx.RoomPaterns()...)
	ctx.Messager().OnWant(ctx.Id, ctx.StateWant, ctx.RoomPaterns()...)
	ctx.Messager().OnSelect(ctx.Id, ctx.SatifySelect, ctx.RoomPaterns()...)
	ctx.Sugar().Debugf("the signal context initialized")
	return ctx, nil
}

func processPendingCandidateMsg(s *socket.Socket, peer *webrtc.PeerConnection, ctx *SignalContext) error {
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
	ctx := GetSingalContext(s)
	if msg.Op == "add" {
		ctx.Sugar().Debugf("received candidate ", msg.Candidate.Candidate)
		err = peer.AddICECandidate(msg.Candidate)
	} else {
		ctx.Sugar().Debugf("received candidate completed")
		err = peer.AddICECandidate(webrtc.ICECandidateInit{})
	}
	return err
}
