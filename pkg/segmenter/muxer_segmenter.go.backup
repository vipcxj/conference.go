package segmenter

import (
	"time"

	"github.com/bluenviron/gohlslib/pkg/storage"
)

type muxerSegmenter interface {
	close()
	writeAV1(time.Time, time.Duration, [][]byte, bool, bool) error
	writeVP9(time.Time, time.Duration, []byte, bool, bool) error
	writeH26x(time.Time, time.Duration, [][]byte, bool, bool) error
	writeOpus(time.Time, time.Duration, [][]byte) error
	writeMPEG4Audio(time.Time, time.Duration, [][]byte) error
}

type PublishSegment func(segment muxerSegment, dts time.Duration, ntp time.Time, forceSwitch bool) storage.File