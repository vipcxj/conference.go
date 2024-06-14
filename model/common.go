package model

import (
	"github.com/pion/webrtc/v4"
	"github.com/vipcxj/conference.go/proto"
)

type RoomMessage = proto.RoomMessage

// RTCPFeedback signals the connection to use additional RTCP packet types.
// https://draft.ortc.org/#dom-rtcrtcpfeedback
// type RTCPFeedback struct {
// 	// Type is the type of feedback.
// 	// see: https://draft.ortc.org/#dom-rtcrtcpfeedback
// 	// valid: ack, ccm, nack, goog-remb, transport-cc
// 	Type string `json:"type" mapstructure:"type"`

//		// The parameter value depends on the type.
//		// For example, type="nack" parameter="pli" will send Picture Loss Indicator packets.
//		Parameter string `json:"parameter" mapstructure:"parameter"`
//	}
type RTCPFeedback = proto.RTCPFeedback

// RTPCodecCapability provides information about codec capabilities.
//
// https://w3c.github.io/webrtc-pc/#dictionary-rtcrtpcodeccapability-members
// type RTPCodecCapability struct {
// 	MimeType     string         `json:"mimeType" mapstructure:"mimeType"`
// 	ClockRate    uint32         `json:"clockRate" mapstructure:"clockRate"`
// 	Channels     uint16         `json:"channels" mapstructure:"channels"`
// 	SDPFmtpLine  string         `json:"sdpFmtpLine" mapstructure:"sdpFmtpLine"`
// 	RTCPFeedback []RTCPFeedback `json:"rtcpFeedback" mapstructure:"rtcpFeedback"`
// }

// func NewRTPCodecCapability(source *webrtc.RTPCodecCapability) *RTPCodecCapability {
// 	res := &RTPCodecCapability{
// 		MimeType:    source.MimeType,
// 		ClockRate:   source.ClockRate,
// 		Channels:    source.Channels,
// 		SDPFmtpLine: source.SDPFmtpLine,
// 	}
// 	if source.RTCPFeedback != nil {
// 		res.RTCPFeedback = make([]RTCPFeedback, len(source.RTCPFeedback))
// 		for i, feedback := range source.RTCPFeedback {
// 			res.RTCPFeedback[i].Type = feedback.Type
// 			res.RTCPFeedback[i].Parameter = feedback.Parameter
// 		}
// 	}
// 	return res
// }

// func (me *RTPCodecCapability) ToWebrtc() *webrtc.RTPCodecCapability {
// 	res := &webrtc.RTPCodecCapability{
// 		MimeType:    me.MimeType,
// 		ClockRate:   me.ClockRate,
// 		Channels:    me.Channels,
// 		SDPFmtpLine: me.SDPFmtpLine,
// 	}
// 	if me.RTCPFeedback != nil {
// 		res.RTCPFeedback = make([]webrtc.RTCPFeedback, len(me.RTCPFeedback))
// 		for i, feedback := range me.RTCPFeedback {
// 			res.RTCPFeedback[i].Type = feedback.Type
// 			res.RTCPFeedback[i].Parameter = feedback.Parameter
// 		}
// 	}
// 	return res
// }

// RTPCodecParameters is a sequence containing the media codecs that an RtpSender
// will choose from, as well as entries for RTX, RED and FEC mechanisms. This also
// includes the PayloadType that has been negotiated
//
// https://w3c.github.io/webrtc-pc/#rtcrtpcodecparameters
//
//	type RTPCodecParameters struct {
//		RTPCodecCapability `mapstructure:",squash"`
//		PayloadType        webrtc.PayloadType `json:"payloadType" mapstructure:"payloadType"`
//	}
type RTPCodecParameters = proto.RTPCodecParameters

func NewRTPCodecParameters(source *webrtc.RTPCodecParameters) *RTPCodecParameters {
	res := &RTPCodecParameters{
		MimeType:    source.MimeType,
		ClockRate:   source.ClockRate,
		Channels:    uint32(source.Channels),
		SdpFmtpLine: source.SDPFmtpLine,
		PayloadType: uint32(source.PayloadType),
	}
	if source.RTCPFeedback != nil {
		res.RtcpFeedback = make([]*proto.RTCPFeedback, len(source.RTCPFeedback))
		for i, feedback := range source.RTCPFeedback {
			res.RtcpFeedback[i] = &proto.RTCPFeedback{
				Type:      feedback.Type,
				Parameter: feedback.Parameter,
			}
		}
	}
	return res
}

func RTPCodecParametersToWebrtc(me *RTPCodecParameters) *webrtc.RTPCodecParameters {
	res := &webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:    me.MimeType,
			ClockRate:   me.ClockRate,
			Channels:    uint16(me.Channels),
			SDPFmtpLine: me.SdpFmtpLine,
		},
		PayloadType: webrtc.PayloadType(me.PayloadType),
	}
	if me.RtcpFeedback != nil {
		res.RTCPFeedback = make([]webrtc.RTCPFeedback, len(me.RtcpFeedback))
		for i, feedback := range me.RtcpFeedback {
			res.RTCPFeedback[i].Type = feedback.Type
			res.RTCPFeedback[i].Parameter = feedback.Parameter
		}
	}
	return res
}

// type ITrack interface {
// 	GetType() string
// 	GetPubId() string
// 	GetGlobalId() string
// 	GetLocalId() string
// 	GetBindId() string
// 	GetRid() string
// 	GetStreamId() string
// 	GetCodec() *RTPCodecParameters
// 	GetLabels() map[string]string
// }

//	type Track struct {
//		Type     string `json:"type" mapstructure:"type"`
//		PubId    string `json:"pubId" mapstructure:"pubId"`
//		GlobalId string `json:"globalId" mapstructure:"globalId"`
//		// only vaild in local
//		LocalId string `json:"localId" mapstructure:"localId"`
//		// used to bind local and remote track
//		BindId   string              `json:"bindId" mapstructure:"bindId"`
//		Rid      string              `json:"rid" mapstructure:"rid"`
//		StreamId string              `json:"streamId" mapstructure:"streamId"`
//		Codec    *RTPCodecParameters `json:"codec" mapstructure:"codec"`
//		Labels   map[string]string   `json:"labels" mapstructure:"labels"`
//	}
type Track = proto.Track

func CopyTrack(src *Track) *Track {
	return &Track{
		Type:     src.Type,
		PubId:    src.PubId,
		GlobalId: src.GlobalId,
		LocalId:  src.LocalId,
		BindId:   src.BindId,
		Rid:      src.Rid,
		StreamId: src.StreamId,
		Codec:    src.Codec,
		Labels:   src.Labels,
	}
}

// func NewTrack(src *proto.Track) *Track {
// 	return &Track{
// 		Type:     src.Type,
// 		PubId:    src.PubId,
// 		GlobalId: src.GlobalId,
// 		LocalId:  src.LocalId,
// 		BindId:   src.BindId,
// 		Rid:      src.Rid,
// 		StreamId: src.StreamId,
// 		Codec:    src.Codec,
// 		Labels:   src.Labels,
// 	}
// }

// func (x *Track) ToProto() *proto.Track {
// 	if x == nil {
// 		return nil
// 	}
// 	return &proto.Track{
// 		Type:     x.Type,
// 		PubId:    x.PubId,
// 		GlobalId: x.GlobalId,
// 		LocalId:  x.LocalId,
// 		BindId:   x.BindId,
// 		Rid:      x.Rid,
// 		StreamId: x.StreamId,
// 		Codec:    x.Codec,
// 		Labels:   x.Labels,
// 	}
// }

// func (x *Track) GetType() string {
// 	if x != nil {
// 		return x.Type
// 	}
// 	return ""
// }

// func (x *Track) GetPubId() string {
// 	if x != nil {
// 		return x.PubId
// 	}
// 	return ""
// }

// func (x *Track) GetGlobalId() string {
// 	if x != nil {
// 		return x.GlobalId
// 	}
// 	return ""
// }

// func (x *Track) GetLocalId() string {
// 	if x != nil {
// 		return x.LocalId
// 	}
// 	return ""
// }

// func (x *Track) GetBindId() string {
// 	if x != nil {
// 		return x.BindId
// 	}
// 	return ""
// }

// func (x *Track) GetRid() string {
// 	if x != nil {
// 		return x.Rid
// 	}
// 	return ""
// }

// func (x *Track) GetStreamId() string {
// 	if x != nil {
// 		return x.StreamId
// 	}
// 	return ""
// }

// func (x *Track) GetCodec() *RTPCodecParameters {
// 	if x != nil {
// 		return x.Codec
// 	}
// 	return nil
// }

// func (x *Track) GetLabels() map[string]string {
// 	if x != nil {
// 		return x.Labels
// 	}
// 	return nil
// }

type Participant struct {
	Id       string
	Name     string
	SourceId string
}
