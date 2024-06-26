package model

import (
	"github.com/pion/webrtc/v4"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/proto"
)

type SignalMessage struct {
	Room string `json:"room" mapstructure:"room"`
	To   string `json:"to" mapstructure:"to"`
}
type ErrorMessage struct {
	SignalMessage `mapstructure:",squash"`
	Msg           string             `json:"msg" mapstructure:"msg"`
	Cause         string             `json:"cause" mapstructure:"cause"`
	Fatal         bool               `json:"fatal" mapstructure:"fatal"`
	CallFrames    []errors.CallFrame `json:"callFrames,omitempty" mapstructure:"callFrames"`
}

type SdpMessage struct {
	SignalMessage `mapstructure:",squash"`
	Type          string `json:"type" mapstructure:"type"`
	Sdp           string `json:"sdp" mapstructure:"sdp"`
	Mid           int    `json:"mid" mapstructure:"mid"`
}

type CandidateMessage struct {
	SignalMessage `mapstructure:",squash"`
	Op            string                  `json:"op" mapstructure:"op"`
	Candidate     webrtc.ICECandidateInit `json:"candidate" mapstructure:"candidate"`
}

type JoinMessage struct {
	SignalMessage `mapstructure:",squash"`
	Rooms         []string `json:"rooms,omitempty" mapstructure:"rooms"`
}

type LeaveMessage struct {
	SignalMessage `mapstructure:",squash"`
	Rooms         []string `json:"rooms,omitempty" mapstructure:"rooms"`
}

type RouterMessage = proto.Router

//	type WantMessage struct {
//		SignalMessage `mapstructure:",squash"`
//		ReqTypes      []string           `json:"reqTypes" mapstructure:"reqTypes"`
//		Pattern       *PublicationPattern `json:"pattern" mapstructure:"pattern"`
//		TransportId   string             `json:"transportId" mapstructure:"transportId"`
//	}
type WantMessage = proto.WantMessage

//	type StateMessage struct {
//		SignalMessage `mapstructure:",squash"`
//		PubId         string   `json:"pubId" mapstructure:"pubId"`
//		Tracks        []*Track `json:"tracks" mapstructure:"tracks"`
//		Addr          string   `json:"addr" mapstructure:"addr"`
//	}
type StateMessage = proto.StateMessage

//	type SelectMessage struct {
//		SignalMessage `mapstructure:",squash"`
//		PubId         string   `json:"pubId" mapstructure:"pubId"`
//		Tracks        []*Track `json:"tracks" mapstructure:"tracks"`
//		TransportId   string   `json:"transportId" mapstructure:"transportId"`
//	}
type SelectMessage = proto.SelectMessage

type WantParticipantMessage = proto.WantParticipantMessage

type StateParticipantMessage = proto.StateParticipantMessage

type StateLeaveMessage = proto.StateLeaveMessage

type PingMessage = proto.PingMessage

type PongMessage = proto.PongMessage

type CustomMessage = proto.CustomMessage

type CustomClusterMessage = proto.CustomClusterMessage

type CustomAckMessage = proto.CustomAckMessage

type PublishOp int

const (
	PUB_OP_ADD = iota
	PUB_OP_REMOVE
)

func (op PublishOp) String() string {
	switch op {
	case PUB_OP_ADD:
		return "add"
	case PUB_OP_REMOVE:
		return "remove"
	default:
		return "unknown"
	}
}

type TrackToPublish struct {
	Type   string            `json:"type" mapstructure:"type"`
	BindId string            `json:"bindId" mapstructure:"bindId"`
	Rid    string            `json:"rid" mapstructure:"rid"`
	Sid    string            `json:"sid" mapstructure:"sid"`
	Labels map[string]string `json:"labels" mapstructure:"labels"`
}

type PublishMessage struct {
	SignalMessage `mapstructure:",squash"`
	Op            PublishOp        `json:"op" mapstructure:"op"`
	Id            string           `json:"id" mapstructure:"id"`
	Tracks        []TrackToPublish `json:"tracks,omitempty" mapstructure:"tracks"`
}

func (m *PublishMessage) Validate() error {
	switch m.Op {
	case PUB_OP_ADD:
		if m.Id != "" {
			return errors.InvalidParam("the publish message does not need id param when the op is \"%v\"", m.Op)
		}
		nTracks := len(m.Tracks)
		if nTracks == 0 {
			return errors.InvalidParam("the publish message need at least 1 track when the op is \"%v\"", m.Op)
		}
		for i := 0; i < nTracks; i++ {
			if m.Tracks[i].BindId == "" {
				return errors.InvalidParam("the publish message's track need a valid bind id when the op is \"%v\", the problem track is tracks[%d]", m.Op, i)
			}
		}
	case PUB_OP_REMOVE:
		if m.Id == "" {
			return errors.InvalidParam("the publish message need id param when the op is \"%v\"", m.Op)
		}
		if len(m.Tracks) > 0 {
			return errors.InvalidParam("the publish message does not need tracks param when the op is \"%v\"", m.Op)
		}
	default:
		return errors.InvalidParam("the publish message has an invalid op %d", m.Op)
	}
	return nil
}

type PublishResultMessage struct {
	Id string `json:"id" mapstructure:"id"`
}

type PublishedMessage struct {
	SignalMessage `mapstructure:",squash"`
	Track         *Track `json:"track" mapstructure:"track"`
}

type SubscribeOp int

const (
	SUB_OP_ADD = iota
	SUB_OP_UPDATE
	SUB_OP_REMOVE
)

func (op SubscribeOp) String() string {
	switch op {
	case SUB_OP_ADD:
		return "add"
	case SUB_OP_UPDATE:
		return "update"
	case SUB_OP_REMOVE:
		return "remove"
	default:
		return "unknown"
	}
}

type SubscribeMessage struct {
	SignalMessage `mapstructure:",squash"`
	Op            SubscribeOp         `json:"op" mapstructure:"op"`
	Id            string              `json:"id" mapstructure:"id"`
	ReqTypes      []string            `json:"reqTypes,omitempty" mapstructure:"reqTypes"`
	Pattern       *PublicationPattern `json:"pattern" mapstructure:"pattern"`
}

func (m *SubscribeMessage) Validate() error {
	switch m.Op {
	case SUB_OP_ADD:
		if m.Id != "" {
			return errors.InvalidParam("the subscribe message does not need id param when the op is \"%v\"", m.Op)
		}
		err := Validate(m.Pattern)
		if err != nil {
			return err
		}
	case SUB_OP_UPDATE:
		if m.Id == "" {
			return errors.InvalidParam("the subscribe message need id param when the op is \"%v\"", m.Op)
		}
		err := Validate(m.Pattern)
		if err != nil {
			return err
		}
	case SUB_OP_REMOVE:
		if m.Id == "" {
			return errors.InvalidParam("the subscribe message need id param when the op is \"%v\"", m.Op)
		}
	default:
		return errors.InvalidParam("the subscribe message has an invalid op %d", m.Op)
	}
	return nil
}

type SubscribeResultMessage struct {
	Id string `json:"id" mapstructure:"id"`
}

type SubscribedMessage struct {
	SignalMessage `mapstructure:",squash"`
	SubId         string         `json:"subId" mapstructure:"subId"`
	PubId         string         `json:"pubId" mapstructure:"pubId"`
	SdpId         int            `json:"sdpId" mapstructure:"sdpId"`
	Tracks        []*proto.Track `json:"tracks,omitempty" mapstructure:"tracks"`
}

type UserInfo struct {
	Key           string   `json:"key" mapstructure:"key"`
	UserId        string   `json:"userId" mapstructure:"userId"`
	UserName      string   `json:"userName" mapstructure:"userName"`
	Role          string   `json:"role" mapstructure:"role"`
	Rooms         []string `json:"rooms,omitempty" mapstructure:"rooms"`
}
