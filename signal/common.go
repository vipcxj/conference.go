package signal

import (
	"github.com/pion/webrtc/v4"
)

// RTCPFeedback signals the connection to use additional RTCP packet types.
// https://draft.ortc.org/#dom-rtcrtcpfeedback
type RTCPFeedback struct {
	// Type is the type of feedback.
	// see: https://draft.ortc.org/#dom-rtcrtcpfeedback
	// valid: ack, ccm, nack, goog-remb, transport-cc
	Type string `json:"type" mapstructure:"type"`

	// The parameter value depends on the type.
	// For example, type="nack" parameter="pli" will send Picture Loss Indicator packets.
	Parameter string `json:"parameter" mapstructure:"parameter"`
}

// RTPCodecCapability provides information about codec capabilities.
//
// https://w3c.github.io/webrtc-pc/#dictionary-rtcrtpcodeccapability-members
type RTPCodecCapability struct {
	MimeType     string         `json:"mimeType" mapstructure:"mimeType"`
	ClockRate    uint32         `json:"clockRate" mapstructure:"clockRate"`
	Channels     uint16         `json:"channels" mapstructure:"channels"`
	SDPFmtpLine  string         `json:"sdpFmtpLine" mapstructure:"sdpFmtpLine"`
	RTCPFeedback []RTCPFeedback `json:"rtcpFeedback" mapstructure:"rtcpFeedback"`
}

func NewRTPCodecCapability(source *webrtc.RTPCodecCapability) *RTPCodecCapability {
	res := &RTPCodecCapability{
		MimeType:    source.MimeType,
		ClockRate:   source.ClockRate,
		Channels:    source.Channels,
		SDPFmtpLine: source.SDPFmtpLine,
	}
	if source.RTCPFeedback != nil {
		res.RTCPFeedback = make([]RTCPFeedback, len(source.RTCPFeedback))
		for i, feedback := range source.RTCPFeedback {
			res.RTCPFeedback[i].Type = feedback.Type
			res.RTCPFeedback[i].Parameter = feedback.Parameter
		}
	}
	return res
}

func (me *RTPCodecCapability) ToWebrtc() *webrtc.RTPCodecCapability {
	res := &webrtc.RTPCodecCapability{
		MimeType:    me.MimeType,
		ClockRate:   me.ClockRate,
		Channels:    me.Channels,
		SDPFmtpLine: me.SDPFmtpLine,
	}
	if me.RTCPFeedback != nil {
		res.RTCPFeedback = make([]webrtc.RTCPFeedback, len(me.RTCPFeedback))
		for i, feedback := range me.RTCPFeedback {
			res.RTCPFeedback[i].Type = feedback.Type
			res.RTCPFeedback[i].Parameter = feedback.Parameter
		}
	}
	return res
}

// RTPCodecParameters is a sequence containing the media codecs that an RtpSender
// will choose from, as well as entries for RTX, RED and FEC mechanisms. This also
// includes the PayloadType that has been negotiated
//
// https://w3c.github.io/webrtc-pc/#rtcrtpcodecparameters
type RTPCodecParameters struct {
	RTPCodecCapability `mapstructure:",squash"`
	PayloadType        webrtc.PayloadType `json:"payloadType" mapstructure:"payloadType"`
}

func NewRTPCodecParameters(source *webrtc.RTPCodecParameters) *RTPCodecParameters {
	return &RTPCodecParameters{
		RTPCodecCapability: *NewRTPCodecCapability(&source.RTPCodecCapability),
		PayloadType:        source.PayloadType,
	}
}

func (me *RTPCodecParameters) ToWebrtc() *webrtc.RTPCodecParameters {
	return &webrtc.RTPCodecParameters{
		RTPCodecCapability: *me.RTPCodecCapability.ToWebrtc(),
		PayloadType:        me.PayloadType,
	}
}

type Track struct {
	Type     string `json:"type" mapstructure:"type"`
	PubId    string `json:"pubId" mapstructure:"pubId"`
	GlobalId string `json:"globalId" mapstructure:"globalId"`
	// only vaild in local
	LocalId string `json:"localId" mapstructure:"localId"`
	// used to bind local and remote track
	BindId   string              `json:"bindId" mapstructure:"bindId"`
	RId      string              `json:"rid" mapstructure:"rid"`
	StreamId string              `json:"streamId" mapstructure:"streamId"`
	Codec    *RTPCodecParameters `json:"codec" mapstructure:"codec"`
	Labels   map[string]string   `json:"labels" mapstructure:"labels"`
}

func (me *Track) MatchLabel(name, value string) bool {
	v, ok := me.Labels[name]
	if ok {
		return v == value
	} else {
		return false
	}
}

func (me *Track) HasLabel(name string) bool {
	if me.Labels == nil {
		return false
	}
	_, ok := me.Labels[name]
	return ok
}
