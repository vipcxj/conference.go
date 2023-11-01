package signal

import (
	"github.com/pion/webrtc/v4"
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

type Track struct {
	GlobalId string `json:"globalId" mapstructure:"globalId"`
	Id       string `json:"id" mapstructure:"id"`
	StreamId string `json:"streamId" mapstructure:"streamId"`
}

type TrackMessage struct {
	SignalMessage `mapstructure:",squash"`
	Op            string   `json:"op" mapstructure:"op"`
	Tracks        []*Track `json:"tracks" mapstructure:"tracks"`
}

type WantMessage struct {
	SignalMessage `mapstructure:",squash"`
	Tracks        []*Track `json:"tracks" mapstructure:"tracks"`
	TransportId   string
}

type StateMessage struct {
	SignalMessage `mapstructure:",squash"`
	Tracks        []*Track `json:"tracks" mapstructure:"tracks"`
	Addr          string   `json:"addr" mapstructure:"addr"`
}

type SubscribeMessage struct {
	SignalMessage `mapstructure:",squash"`
	Tracks        []*Track `json:"tracks" mapstructure:"tracks"`
}

type SubscribedMessage struct {
	SignalMessage `mapstructure:",squash"`
	Tracks        []*Track `json:"tracks" mapstructure:"tracks"`
}
