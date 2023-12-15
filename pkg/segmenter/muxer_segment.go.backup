package segmenter

import (
	"fmt"
	"io"
	"strconv"
	"time"
)

func segmentName(prefix string, id uint64, mp4 bool) string {
	if mp4 {
		return prefix + "_seg" + strconv.FormatUint(id, 10) + ".mp4"
	}
	return prefix + "_seg" + strconv.FormatUint(id, 10) + ".ts"
}

type muxerSegment interface {
	close()
	finalize(nextDTS time.Duration) error
	getName() string
	getStartNtp() time.Time
	getStartDts() time.Duration
	getDuration() time.Duration
	getSize() uint64
	isForceSwitched() bool
	reader() (io.ReadCloser, error)
}

type muxerGap struct {
	duration time.Duration
	ntp time.Time
	dts time.Duration
}

func (g muxerGap) close() {
}

func (g muxerGap) getName() string {
	return ""
}

func (g muxerGap) getStartNtp() time.Time {
	return g.ntp
}

func (g muxerGap) getStartDts() time.Duration {
	return g.dts
}

func (g muxerGap) getDuration() time.Duration {
	return g.duration
}

func (muxerGap) getSize() uint64 {
	return 0
}

func (muxerGap) isForceSwitched() bool {
	return false
}

func (muxerGap) reader() (io.ReadCloser, error) {
	return nil, fmt.Errorf("unimplemented")
}