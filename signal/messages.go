package signal

import (
	"github.com/pion/webrtc/v4"
	"github.com/vipcxj/conference.go/errors"
)

type SignalMessage struct {
	To string `json:"to" mapstructure:"to"`
}

type ErrorMessage struct {
	SignalMessage `mapstructure:",squash"`
	Msg           string `json:"msg" mapstructure:"msg"`
	Cause         string `json:"cause" mapstructure:"cause"`
	Fatal         bool   `json:"fatal" mapstructure:"fatal"`
}

type SdpMessage struct {
	SignalMessage `mapstructure:",squash"`
	Type          string `json:"type" mapstructure:"type"`
	Sdp           string `json:"sdp" mapstructure:"sdp"`
}

type CandidateMessage struct {
	SignalMessage `mapstructure:",squash"`
	Op            string                  `json:"op" mapstructure:"op"`
	Candidate     webrtc.ICECandidateInit `json:"candidate" mapstructure:"candidate"`
}

type TrackMessage struct {
	SignalMessage `mapstructure:",squash"`
	Op            string   `json:"op" mapstructure:"op"`
	Tracks        []*Track `json:"tracks" mapstructure:"tracks"`
}

type WantMessage struct {
	SignalMessage `mapstructure:",squash"`
	Pattern       PublicationPattern `json:"pattern" mapstructure:"pattern"`
	TransportId   string             `json:"transportId" mapstructure:"transportId"`
}

type StateMessage struct {
	SignalMessage `mapstructure:",squash"`
	Tracks        []*Track `json:"tracks" mapstructure:"tracks"`
	Addr          string   `json:"addr" mapstructure:"addr"`
}

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

type PublishMessage struct {
	SignalMessage `mapstructure:",squash"`
	Op            PublishOp `json:"op" mapstructure:"op"`
	Id            string    `json:"id" mapstructure:"id"`
	Tracks        []struct {
		BindId string            `json:"bindId" mapstructure:"bindId"`
		Labels map[string]string `json:"labels" mapstructure:"labels"`
	} `json:"tracks" mapstructure:"tracks"`
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

type PublishedMessage struct {
	SignalMessage `mapstructure:",squash"`
	Id            string   `json:"id" mapstructure:"id"`
	Tracks        []*Track `json:"tracks" mapstructure:"tracks"`
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
	Op            SubscribeOp        `json:"op" mapstructure:"op"`
	Id            string             `json:"id" mapstructure:"id"`
	Pattern       PublicationPattern `json:"pattern" mapstructure:"pattern"`
}

func (m *SubscribeMessage) Validate() error {
	switch m.Op {
	case SUB_OP_ADD:
		if m.Id != "" {
			return errors.InvalidParam("the subscribe message does not need id param when the op is \"%v\"", m.Op)
		}
		err := m.Pattern.Validate()
		if err != nil {
			return err
		}
	case SUB_OP_UPDATE:
		if m.Id == "" {
			return errors.InvalidParam("the subscribe message need id param when the op is \"%v\"", m.Op)
		}
		err := m.Pattern.Validate()
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

type SubscribedMessage struct {
	SignalMessage `mapstructure:",squash"`
	SubId         string `json:"subId" mapstructure:"subId"`
	Track         Track  `json:"track" mapstructure:"track"`
}
