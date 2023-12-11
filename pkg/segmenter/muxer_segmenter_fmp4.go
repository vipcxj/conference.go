package segmenter

import (
	"fmt"
	"time"

	"github.com/bluenviron/mediacommon/pkg/codecs/h264"
	"github.com/bluenviron/mediacommon/pkg/codecs/h265"
	"github.com/bluenviron/mediacommon/pkg/codecs/mpeg4audio"
	"github.com/bluenviron/mediacommon/pkg/codecs/opus"
	"github.com/bluenviron/mediacommon/pkg/formats/fmp4"

	"github.com/bluenviron/gohlslib"
	"github.com/bluenviron/gohlslib/pkg/codecs"
)

const (
	fmp4StartDTS = 10 * time.Second
)

func fmp4TimeScale(c codecs.Codec) uint32 {
	switch codec := c.(type) {
	case *codecs.MPEG4Audio:
		return uint32(codec.SampleRate)

	case *codecs.Opus:
		return 48000
	}

	return 0
}

func partDurationIsCompatible(partDuration time.Duration, sampleDuration time.Duration) bool {
	if sampleDuration > partDuration {
		return false
	}

	f := (partDuration / sampleDuration)
	if (partDuration % sampleDuration) != 0 {
		f++
	}
	f *= sampleDuration

	return partDuration > ((f * 85) / 100)
}

func partDurationIsCompatibleWithAll(partDuration time.Duration, sampleDurations map[time.Duration]struct{}) bool {
	for sd := range sampleDurations {
		if !partDurationIsCompatible(partDuration, sd) {
			return false
		}
	}
	return true
}

func findCompatiblePartDuration(
	minPartDuration time.Duration,
	sampleDurations map[time.Duration]struct{},
) time.Duration {
	i := minPartDuration
	for ; i < 5*time.Second; i += 5 * time.Millisecond {
		if partDurationIsCompatibleWithAll(i, sampleDurations) {
			break
		}
	}
	return i
}

type dtsExtractor interface {
	Extract([][]byte, time.Duration) (time.Duration, error)
}

func allocateDTSExtractor(track *gohlslib.Track) dtsExtractor {
	switch track.Codec.(type) {
	case *codecs.H265:
		return h265.NewDTSExtractor()

	case *codecs.H264:
		return h264.NewDTSExtractor()
	}
	return nil
}

type augmentedSample struct {
	fmp4.PartSample
	dts time.Duration
	ntp time.Time
}

type muxerSegmenterFMP4 struct {
	lowLatency      bool
	segmentDuration time.Duration
	partDuration    time.Duration
	segmentMaxSize  uint64
	videoTrack      *gohlslib.Track
	audioTrack      *gohlslib.Track
	prefix          string
	publishSegment  PublishSegment
	publishPart     func(*muxerPart)

	audioTimeScale                 uint32
	videoFirstRandomAccessReceived bool
	videoDTSExtractor              dtsExtractor
	startDTS                       time.Duration
	currentSegment                 *muxerSegmentFMP4
	nextSegmentID                  uint64
	nextPartID                     uint64
	nextVideoSample                *augmentedSample
	nextAudioSample                *augmentedSample
	firstSegmentFinalized          bool
	sampleDurations                map[time.Duration]struct{}
	adjustedPartDuration           time.Duration
}

func newMuxerSegmenterFMP4(
	lowLatency bool,
	segmentDuration time.Duration,
	partDuration time.Duration,
	segmentMaxSize uint64,
	videoTrack *gohlslib.Track,
	audioTrack *gohlslib.Track,
	prefix string,
	publishSegment PublishSegment,
	publishPart func(*muxerPart),
) *muxerSegmenterFMP4 {
	m := &muxerSegmenterFMP4{
		lowLatency:      lowLatency,
		segmentDuration: segmentDuration,
		partDuration:    partDuration,
		segmentMaxSize:  segmentMaxSize,
		videoTrack:      videoTrack,
		audioTrack:      audioTrack,
		prefix:          prefix,
		publishSegment:  publishSegment,
		publishPart:     publishPart,
		sampleDurations: make(map[time.Duration]struct{}),
	}

	if audioTrack != nil {
		m.audioTimeScale = fmp4TimeScale(audioTrack.Codec)
	}

	// add initial gaps, required by iOS LL-HLS
	if m.lowLatency {
		m.nextSegmentID = 7
	}

	return m
}

func (m *muxerSegmenterFMP4) close() {
	if m.currentSegment != nil {
		m.currentSegment.finalize(0) //nolint:errcheck
		m.currentSegment.close()
	}
}

func (m *muxerSegmenterFMP4) takeSegmentID() uint64 {
	id := m.nextSegmentID
	m.nextSegmentID++
	return id
}

func (m *muxerSegmenterFMP4) takePartID() uint64 {
	id := m.nextPartID
	m.nextPartID++
	return id
}

func (m *muxerSegmenterFMP4) givePartID() {
	m.nextPartID--
}

// iPhone iOS fails if part durations are less than 85% of maximum part duration.
// find a part duration that is compatible with all received sample durations
func (m *muxerSegmenterFMP4) adjustPartDuration(sampleDuration time.Duration) {
	if !m.lowLatency || m.firstSegmentFinalized {
		return
	}

	// avoid a crash by skipping invalid durations
	if sampleDuration == 0 {
		return
	}

	if _, ok := m.sampleDurations[sampleDuration]; !ok {
		m.sampleDurations[sampleDuration] = struct{}{}
		m.adjustedPartDuration = findCompatiblePartDuration(
			m.partDuration,
			m.sampleDurations,
		)
	}
}

func (m *muxerSegmenterFMP4) writeAV1(
	ntp time.Time,
	dts time.Duration,
	tu [][]byte,
	randomAccess bool,
	forceSwitch bool,
) error {
	if !m.videoFirstRandomAccessReceived {
		// skip sample silently until we find one with an IDR
		if !randomAccess {
			return nil
		}

		m.videoFirstRandomAccessReceived = true
	}

	ps, err := fmp4.NewPartSampleAV1(
		randomAccess,
		tu)
	if err != nil {
		return err
	}

	return m.writeVideo(
		randomAccess,
		forceSwitch,
		&augmentedSample{
			PartSample: *ps,
			dts:        dts,
			ntp:        ntp,
		})
}

func (m *muxerSegmenterFMP4) writeVP9(
	ntp time.Time,
	dts time.Duration,
	frame []byte,
	randomAccess bool,
	forceSwitch bool,
) error {
	if !m.videoFirstRandomAccessReceived {
		// skip sample silently until we find one with an IDR
		if !randomAccess {
			return nil
		}

		m.videoFirstRandomAccessReceived = true
	}

	return m.writeVideo(
		randomAccess,
		forceSwitch,
		&augmentedSample{
			PartSample: fmp4.PartSample{
				IsNonSyncSample: !randomAccess,
				Payload:         frame,
			},
			dts: dts,
			ntp: ntp,
		})
}

func (m *muxerSegmenterFMP4) writeH26x(
	ntp time.Time,
	pts time.Duration,
	au [][]byte,
	randomAccess bool,
	forceSwitch bool,
) error {
	var dts time.Duration

	if !m.videoFirstRandomAccessReceived {
		// skip sample silently until we find one with an IDR
		if !randomAccess {
			return nil
		}

		m.videoFirstRandomAccessReceived = true
		m.videoDTSExtractor = allocateDTSExtractor(m.videoTrack)
	}

	var err error
	dts, err = m.videoDTSExtractor.Extract(au, pts)
	if err != nil {
		return fmt.Errorf("unable to extract DTS: %v", err)
	}

	ps, err := fmp4.NewPartSampleH26x(
		int32(durationGoToMp4(pts-dts, 90000)),
		randomAccess,
		au)
	if err != nil {
		return err
	}

	return m.writeVideo(
		randomAccess,
		forceSwitch,
		&augmentedSample{
			PartSample: *ps,
			dts:        dts,
			ntp:        ntp,
		})
}

func (m *muxerSegmenterFMP4) writeOpus(ntp time.Time, pts time.Duration, packets [][]byte) error {
	for _, packet := range packets {
		err := m.writeAudio(&augmentedSample{
			PartSample: fmp4.PartSample{
				Payload: packet,
			},
			dts: pts,
			ntp: ntp,
		})
		if err != nil {
			return err
		}

		duration := opus.PacketDuration(packet)
		ntp = ntp.Add(duration)
		pts += duration
	}

	return nil
}

func (m *muxerSegmenterFMP4) writeMPEG4Audio(ntp time.Time, pts time.Duration, aus [][]byte) error {
	sampleRate := time.Duration(m.audioTrack.Codec.(*codecs.MPEG4Audio).Config.SampleRate)

	for i, au := range aus {
		auNTP := ntp.Add(time.Duration(i) * mpeg4audio.SamplesPerAccessUnit *
			time.Second / sampleRate)
		auPTS := pts + time.Duration(i)*mpeg4audio.SamplesPerAccessUnit*
			time.Second/sampleRate

		err := m.writeAudio(&augmentedSample{
			PartSample: fmp4.PartSample{
				Payload: au,
			},
			dts: auPTS,
			ntp: auNTP,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *muxerSegmenterFMP4) writeVideo(
	randomAccess bool,
	forceSwitch bool,
	sample *augmentedSample,
) error {
	// add a starting DTS to avoid a negative BaseTime
	sample.dts += fmp4StartDTS

	// BaseTime is still negative, this is not supported by fMP4. Reject the sample silently.
	if (sample.dts - m.startDTS) < 0 {
		return nil
	}

	// put samples into a queue in order to
	// - compute sample duration
	// - check if next sample is IDR
	sample, m.nextVideoSample = m.nextVideoSample, sample
	if sample == nil {
		return nil
	}
	sample.Duration = uint32(durationGoToMp4(m.nextVideoSample.dts-sample.dts, 90000))

	// create first segment
	if m.currentSegment == nil {
		m.startDTS = sample.dts
		file := m.publishSegment(nil, m.nextVideoSample.dts, m.nextAudioSample.ntp, true)
		if file == nil {
			panic(fmt.Errorf("must return a vaild file"))
		}

		var err error
		m.currentSegment, err = newMuxerSegmentFMP4(
			m.lowLatency,
			m.takeSegmentID(),
			sample.ntp,
			sample.dts,
			m.segmentMaxSize,
			m.videoTrack,
			m.audioTrack,
			m.audioTimeScale,
			m.prefix,
			false,
			file,
			m.takePartID,
			m.givePartID,
			m.publishPart,
		)
		if err != nil {
			return err
		}
	}

	m.adjustPartDuration(m.nextVideoSample.dts - sample.dts)

	err := m.currentSegment.writeVideo(sample, m.nextVideoSample.dts, m.adjustedPartDuration)
	if err != nil {
		return err
	}

	// switch segment
	if randomAccess {
		file := m.publishSegment(m.currentSegment, m.nextVideoSample.dts, m.nextAudioSample.ntp, forceSwitch)
		if file == nil {
			return nil
		}
		err := m.currentSegment.finalize(m.nextVideoSample.dts)
		if err != nil {
			return err
		}

		m.firstSegmentFinalized = true

		m.currentSegment, err = newMuxerSegmentFMP4(
			m.lowLatency,
			m.takeSegmentID(),
			m.nextVideoSample.ntp,
			m.nextVideoSample.dts,
			m.segmentMaxSize,
			m.videoTrack,
			m.audioTrack,
			m.audioTimeScale,
			m.prefix,
			forceSwitch,
			file,
			m.takePartID,
			m.givePartID,
			m.publishPart,
		)
		if err != nil {
			return err
		}

		if forceSwitch {
			m.firstSegmentFinalized = false

			// reset adjusted part duration
			m.sampleDurations = make(map[time.Duration]struct{})
		}
	}

	return nil
}

func (m *muxerSegmenterFMP4) writeAudio(sample *augmentedSample) error {
	// add a starting DTS to avoid a negative BaseTime
	sample.dts += fmp4StartDTS

	// BaseTime is still negative, this is not supported by fMP4. Reject the sample silently.
	if (sample.dts - m.startDTS) < 0 {
		return nil
	}

	if m.videoTrack != nil {
		// wait for the video track
		if !m.videoFirstRandomAccessReceived {
			return nil
		}
	}

	// put samples into a queue in order to compute the sample duration
	sample, m.nextAudioSample = m.nextAudioSample, sample
	if sample == nil {
		return nil
	}
	sample.Duration = uint32(durationGoToMp4(m.nextAudioSample.dts-sample.dts, m.audioTimeScale))

	if m.videoTrack == nil {
		// create first segment
		if m.currentSegment == nil {
			m.startDTS = sample.dts

			file := m.publishSegment(nil, m.nextAudioSample.dts, m.nextAudioSample.ntp, true)
			if file == nil {
				panic(fmt.Errorf("must return a vaild file"))
			}

			var err error
			m.currentSegment, err = newMuxerSegmentFMP4(
				m.lowLatency,
				m.takeSegmentID(),
				sample.ntp,
				sample.dts,
				m.segmentMaxSize,
				m.videoTrack,
				m.audioTrack,
				m.audioTimeScale,
				m.prefix,
				false,
				file,
				m.takePartID,
				m.givePartID,
				m.publishPart,
			)
			if err != nil {
				return err
			}
		}
	} else {
		// wait for the video track
		if m.currentSegment == nil {
			return nil
		}
	}

	err := m.currentSegment.writeAudio(sample, m.nextAudioSample.dts, m.partDuration)
	if err != nil {
		return err
	}

	// switch segment
	if m.videoTrack == nil {
		file := m.publishSegment(m.currentSegment, m.nextAudioSample.dts, m.nextAudioSample.ntp, false)
		if file == nil {
			return nil
		}
		err := m.currentSegment.finalize(m.nextAudioSample.dts)
		if err != nil {
			return err
		}

		m.firstSegmentFinalized = true

		m.currentSegment, err = newMuxerSegmentFMP4(
			m.lowLatency,
			m.takeSegmentID(),
			m.nextAudioSample.ntp,
			m.nextAudioSample.dts,
			m.segmentMaxSize,
			m.videoTrack,
			m.audioTrack,
			m.audioTimeScale,
			m.prefix,
			false,
			file,
			m.takePartID,
			m.givePartID,
			m.publishPart,
		)
		if err != nil {
			return err
		}
	}

	return nil
}