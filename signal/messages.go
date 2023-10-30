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
	Sdp           string         `json:"sdp" mapstructure:"sdp"`
}

type CandidateMessage struct {
	SignalMessage `mapstructure:",squash"`
	Op            string                  `json:"op" mapstructure:"op"`
	Candidate     webrtc.ICECandidateInit `json:"candidate" mapstructure:"candidate"`
}

type Stream struct {
	Id       string `json:"id" mapstructure:"id"`
	StreamId string `json:"streamId" mapstructure:"streamId"`
}

type StreamMessage struct {
	SignalMessage `mapstructure:",squash"`
	Op            string `json:"op" mapstructure:"op"`
	Stream        Stream `json:"stream" mapstructure:"stream"`
}

type SubscribeMessage struct {
	SignalMessage `mapstructure:",squash"`
	Stream        Stream `json:"stream" mapstructure:"stream"`
}