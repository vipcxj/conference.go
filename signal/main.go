package signal

import (
	"fmt"
	"reflect"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/pion/webrtc/v4"
	"github.com/vipcxj/conference.go/errors"
	"github.com/zishang520/socket.io/v2/socket"
)

func FatalErrorAndClose(s *socket.Socket, msg string) {
	s.Timeout(time.Second*1).EmitWithAck("fatal error", msg)(func(a []any, err error) {
		s.Disconnect(true)
	})
}

func parseArgs[R any, PR *R](out PR, args ...any) (func(...any), error) {
	if len(args) == 0 {
		if out == nil {
			return nil, nil
		} else {
			return nil, errors.FatalError("too little parameter")
		}
	}
	last := args[len(args)-1]
	var ark func(...any) = nil
	if reflect.TypeOf(last).Kind() == reflect.Func {
		ark = last.(func(...any))
		args = args[0 : len(args)-1]
	}
	if len(args) == 0 {
		if out == nil {
			return nil, nil
		} else {
			return nil, errors.FatalError("too little parameter")
		}
	}
	err := mapstructure.Decode(args[0], out)
	return ark, err
}

func InitSignal(s *socket.Socket) error {
	ctx := GetSingalContext(s)
	if ctx == nil {
		return errors.FatalError("unable to find the signal context")
	}
	s.On("offer", func(args ...any) {
		msg := SdpMessage{}
		ark, err := parseArgs(&msg, args...)
		if err != nil {
			FatalErrorAndClose(s, err.Error())
			return
		}
		go OnOffer(s, ctx, &msg)
		if ark != nil {
			ark()
		}
	})
	s.On("candidate", func(args ...any) {
		msg := CandidateMessage{}
		ark, err := parseArgs(&msg, args...)
		if err != nil {
			FatalErrorAndClose(s, err.Error())
			return
		}
		go OnCandidate(s, ctx, &msg)
		if ark != nil {
			ark()
		}
	})
	return nil
}

func OnOffer(s *socket.Socket, ctx *SingalContext, msg *SdpMessage) {
	peer, err := ctx.MakeSurePeer()
	if err != nil {
		FatalErrorAndClose(s, err.Error())
		return
	}
	offer := webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: msg.Sdp}
	err = peer.SetRemoteDescription(offer)
	if err != nil {
		FatalErrorAndClose(s, err.Error())
	}
	answer, err := peer.CreateAnswer(nil)
	if err != nil {
		FatalErrorAndClose(s, err.Error())
	}
	s.Emit("answer", SdpMessage{Sdp: answer.SDP})
	err = peer.SetLocalDescription(answer)
	if err != nil {
		FatalErrorAndClose(s, err.Error())
	}
}

func OnCandidate(s *socket.Socket, ctx *SingalContext, msg *CandidateMessage) {
	peer, err := ctx.MakeSurePeer()
	if err != nil {
		FatalErrorAndClose(s, err.Error())
		return
	}
	if msg.Op == "add" {
		fmt.Println("Received candidate ", msg.Candidate.Candidate)
		err = peer.AddICECandidate(msg.Candidate)
	} else {
		fmt.Println("Received candidate completed")
		err = peer.AddICECandidate(webrtc.ICECandidateInit{})
	}
	if err != nil {
		FatalErrorAndClose(s, err.Error())
	}
}
