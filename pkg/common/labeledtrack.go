package common

import "github.com/pion/webrtc/v4"

type LabeledTrack interface {
	TrackRemote() *webrtc.TrackRemote
	Labels() map[string]string
}
