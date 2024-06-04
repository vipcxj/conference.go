package signal

import (
	"fmt"
	"net/http"
	nu "net/url"
	"strings"

	"github.com/pion/webrtc/v4"
	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/errors"
)

const CLOSE_CALLBACK_PREFIX = "/conference/close"

func CloseCallback(conf *config.ConferenceConfigure, id string) string {
	return fmt.Sprintf("%v%v/%v", conf.SignalExternalAddress(), CLOSE_CALLBACK_PREFIX, nu.QueryEscape(id))
}

func InitSignal(s Signal) (*SignalContext, error) {
	ctx := s.GetContext()
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
	ctx.Sugar().Debugf("Auth info: key=%v; uid=%v; uname=%v; room=%v; nonce=%v", auth.Key, auth.UID, auth.UName, strings.Join(auth.Rooms, ","), auth.Nonce)
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
		ctx.SelfClose(true)
		return nil, errors.FatalError(msg)
	}
	if st != 0 && st != http.StatusOK {
		msg := fmt.Sprintf("start callback return status code %v, so close the singal context.", st)
		ctx.Sugar().Warn(msg)
		ctx.SelfClose(true)
		return nil, errors.FatalError(msg)
	}
	ctx.Metrics().OnSignalConnectStart(ctx)
	ctx.SetCloseCallback(NewConferenceCallback("close", ctx.Global.Conf().Callback.OnClose, ctx))
	ctx.Global.RegisterSignalContext(ctx)
	ctx.Messager().OnState(ctx.Id, ctx.AcceptTrack, ctx.RoomPaterns()...)
	ctx.Messager().OnWant(ctx.Id, ctx.StateWant, ctx.RoomPaterns()...)
	ctx.Messager().OnSelect(ctx.Id, ctx.SatifySelect, ctx.RoomPaterns()...)
	ctx.Messager().OnWantParticipant(ctx.Id, ctx.StateParticipants, ctx.RoomPaterns()...)
	ctx.Messager().OnStateParticipant(ctx.Id, ctx.AcceptParticipants, ctx.RoomPaterns()...)
	ctx.Messager().OnUser(ctx.Id, ctx.OnUserMessage, ctx.RoomPaterns()...)
	ctx.Messager().OnUserAck(ctx.Id, ctx.OnUserAckMessage, ctx.RoomPaterns()...)
	s.On("disconnect", func(ack AckFunc, args ...any) {
		ctx.Metrics().OnSignalConnectClose(ctx)
		reason := args[0].(string)
		ctx.Sugar().Infof("socket disconnect because %s", reason)
		go func() {
			ctx.Sugar().Infof("close the socket")
			ctx.Close()
		}()
	})
	s.On("join", func(ack AckFunc, args ...any) {
		ctx.Sugar().Debugf("receive join msg")
		msg := JoinMessage{}
		err := parseArgs(&msg, args...)
		defer FinallyResponse(ctx, ack, nil, "join", false)
		if err != nil {
			panic(err)
		}
		err = ctx.JoinRoom(msg.Rooms...)
		if err != nil {
			panic(err)
		}
	})
	s.On("leave", func(ack AckFunc, args ...any) {
		ctx.Sugar().Debugf("receive leave msg")
		msg := LeaveMessage{}
		err := parseArgs(&msg, args...)
		defer FinallyResponse(ctx, ack, nil, "leave", false)
		if err != nil {
			panic(err)
		}
		ctx.LeaveRoom(msg.Rooms...)
	})
	s.On("sdp", func(ack AckFunc, args ...any) {
		ctx.Sugar().Debugf("receive sdp msg")
		msg := SdpMessage{}
		err := parseArgs(&msg, args...)
		defer FinallyResponse(ctx, ack, nil, "sdp", true)
		if err != nil {
			panic(err)
		}
		peer, err := ctx.MakeSurePeer()
		if err != nil {
			panic(err)
		}
		ctx.Sugar().Infof("accept %s sdp msg with id %d", msg.Type, msg.Mid)
		go func() {
			defer FinallyResponse(ctx, ack, nil, "sdp", false)
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
			err = processPendingCandidateMsg(ctx)
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
				err = ctx.emit("sdp", SdpMessage{
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
	s.On("candidate", func(ack AckFunc, args ...any) {
		ctx.Sugar().Debugf("receive candidate msg")
		msg := CandidateMessage{}
		err := parseArgs(&msg, args...)
		defer FinallyResponse(ctx, ack, nil, "candidate", false)
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
		err = _processCandidateMsg(ctx, &msg)
		if err != nil {
			panic(err)
		}
	})
	s.On("publish", func(ack AckFunc, args ...any) {
		ctx.Sugar().Debugf("receive publish msg")
		msg := PublishMessage{}
		err := parseArgs(&msg, args...)
		arkArgs := make([]any, 1)
		defer FinallyResponse(ctx, ack, arkArgs, "publish", true)
		if err != nil {
			panic(err)
		}
		go func() {
			defer FinallyResponse(ctx, ack, arkArgs, "publish", false)
			pubId, err := ctx.Publish(&msg)
			if err != nil {
				panic(err)
			}
			arkArgs[0] = &PublishResultMessage{
				Id: pubId,
			}
		}()
	})
	s.On("subscribe", func(ack AckFunc, args ...any) {
		ctx.Sugar().Debugf("receive subscribe msg")
		msg := SubscribeMessage{}
		err := parseArgs(&msg, args...)
		arkArgs := make([]any, 1)
		defer FinallyResponse(ctx, ack, arkArgs, "subscribe", true)
		if err != nil {
			panic(err)
		}
		go func() {
			defer FinallyResponse(ctx, ack, arkArgs, "subscribe", false)
			subId, err := ctx.Subscribe(&msg)
			if err != nil {
				panic(err)
			}
			arkArgs[0] = &SubscribeResultMessage{
				Id: subId,
			}
		}()
	})
	s.On("user", func(ack AckFunc, args ...any) {
		ctx.Sugar().Debugf("receive user msg")
		msg := UserMessage{}
		err := parseArgs(&msg, args...)
		arkArgs := make([]any, 1)
		defer FinallyResponse(ctx, ack, arkArgs, "user", false)
		if err != nil {
			panic(err)
		}
		err = ctx.ClusterEmit(&msg)
		if err != nil {
			panic(err)
		}
	})
	s.On("user-ack", func(ack AckFunc, args ...any) {
		ctx.Sugar().Debugf("receive user-ack msg")
		msg := UserAckMessage{}
		err := parseArgs(&msg, args...)
		arkArgs := make([]any, 1)
		defer FinallyResponse(ctx, ack, arkArgs, "user", false)
		if err != nil {
			panic(err)
		}
		err = ctx.ClusterEmit(&msg)
		if err != nil {
			panic(err)
		}
	})
	ctx.ClusterEmit(&WantParticipantMessage{})
	ctx.Sugar().Debugf("the signal context initialized")
	return ctx, nil
}

func processPendingCandidateMsg(ctx *SignalContext) error {
	ctx.cand_mux.Lock()
	defer ctx.cand_mux.Unlock()
	var err error
	for _, msg := range ctx.pendingCandidates {
		if e := _processCandidateMsg(ctx, msg); e != nil {
			err = e
		}
	}
	ctx.pendingCandidates = nil
	return err
}

func _processCandidateMsg(ctx *SignalContext, msg *CandidateMessage) error {
	var err error
	if msg.Op == "add" {
		ctx.Sugar().Debugf("received candidate ", msg.Candidate.Candidate)
		err = ctx.Peer.AddICECandidate(msg.Candidate)
	} else {
		ctx.Sugar().Debugf("received candidate completed")
		err = ctx.Peer.AddICECandidate(webrtc.ICECandidateInit{})
	}
	return err
}
