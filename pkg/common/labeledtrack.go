package common

import "github.com/pion/webrtc/v4"

type LabeledTrack interface {
	TrackRemote() *webrtc.TrackRemote
	ID() string
	Labels() map[string]string
}
