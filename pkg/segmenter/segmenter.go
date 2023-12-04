package segmenter

import (
	"time"

	"github.com/Eyevinn/mp4ff/mp4"
)

type ScaledDuration int64

func (s ScaledDuration) ToDuration(sampleRate int64) time.Duration {
	return time.Duration(int64(s) * int64(time.Second) / sampleRate)
}

type Segment struct {
	Start int64
	Size int32
	TimeStart int64
	Duration ScaledDuration
}

type Segmenter struct {
	Prefix string
}

func (s *Segmenter) Init() {
	mp4.NewFile()
}