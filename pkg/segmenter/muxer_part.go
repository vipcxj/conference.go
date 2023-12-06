package segmenter

import (
	"io"
	"strconv"
	"time"

	"github.com/bluenviron/gohlslib"
	"github.com/bluenviron/gohlslib/pkg/storage"
	"github.com/bluenviron/mediacommon/pkg/formats/fmp4"
)

func partName(prefix string, id uint64) string {
	return prefix + "_part" + strconv.FormatUint(id, 10) + ".mp4"
}

type augmentedSample struct {
	fmp4.PartSample
	dts time.Duration
	ntp time.Time
}

type muxerPart struct {
	startDTS       time.Duration
	videoTrack     *gohlslib.Track
	audioTrack     *gohlslib.Track
	audioTimeScale uint32
	id             uint64
	storage        storage.Part

	name                string
	isIndependent       bool
	videoSamples        []*fmp4.PartSample
	audioSamples        []*fmp4.PartSample
	finalDuration       time.Duration
	videoStartDTSFilled bool
	videoStartDTS       time.Duration
	audioStartDTSFilled bool
	audioStartDTS       time.Duration
}

func newMuxerPart(
	startDTS time.Duration,
	videoTrack *gohlslib.Track,
	audioTrack *gohlslib.Track,
	audioTimeScale uint32,
	prefix string,
	id uint64,
	storage storage.Part,
) *muxerPart {
	p := &muxerPart{
		startDTS:       startDTS,
		videoTrack:     videoTrack,
		audioTrack:     audioTrack,
		audioTimeScale: audioTimeScale,
		id:             id,
		storage:        storage,
		name:           partName(prefix, id),
	}

	if videoTrack == nil {
		p.isIndependent = true
	}

	return p
}

func (p *muxerPart) getName() string {
	return p.name
}

func (p *muxerPart) reader() (io.ReadCloser, error) {
	return p.storage.Reader()
}

func (p *muxerPart) computeDuration(nextDTS time.Duration) time.Duration {
	return nextDTS - p.startDTS
}

func durationGoToMp4(v time.Duration, timeScale uint32) uint64 {
	timeScale64 := uint64(timeScale)
	secs := v / time.Second
	dec := v % time.Second
	return uint64(secs)*timeScale64 + uint64(dec)*timeScale64/uint64(time.Second)
}

func durationMp4ToGo(v uint64, timeScale uint32) time.Duration {
	timeScale64 := uint64(timeScale)
	secs := v / timeScale64
	dec := v % timeScale64
	return time.Duration(secs)*time.Second + time.Duration(dec)*time.Second/time.Duration(timeScale64)
}

func (p *muxerPart) finalize(nextDTS time.Duration) error {
	part := fmp4.Part{}

	if p.videoSamples != nil {
		part.Tracks = append(part.Tracks, &fmp4.PartTrack{
			ID:       1,
			BaseTime: durationGoToMp4(p.videoStartDTS, 90000),
			Samples:  p.videoSamples,
		})
	}

	if p.audioSamples != nil {
		part.Tracks = append(part.Tracks, &fmp4.PartTrack{
			ID:       1 + len(part.Tracks),
			BaseTime: durationGoToMp4(p.audioStartDTS, p.audioTimeScale),
			Samples:  p.audioSamples,
		})
	}

	err := part.Marshal(p.storage.Writer())
	if err != nil {
		return err
	}

	p.finalDuration = p.computeDuration(nextDTS)

	p.videoSamples = nil
	p.audioSamples = nil

	return nil
}

func (p *muxerPart) writeVideo(sample *augmentedSample) {
	if !p.videoStartDTSFilled {
		p.videoStartDTSFilled = true
		p.videoStartDTS = sample.dts
	}

	if !sample.IsNonSyncSample {
		p.isIndependent = true
	}

	p.videoSamples = append(p.videoSamples, &sample.PartSample)
}

func (p *muxerPart) writeAudio(sample *augmentedSample) {
	if !p.audioStartDTSFilled {
		p.audioStartDTSFilled = true
		p.audioStartDTS = sample.dts
	}

	p.audioSamples = append(p.audioSamples, &sample.PartSample)
}