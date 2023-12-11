package segmenter

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/bluenviron/gohlslib"
	"github.com/bluenviron/gohlslib/pkg/codecs"
	"github.com/bluenviron/gohlslib/pkg/storage"
	"github.com/bluenviron/mediacommon/pkg/codecs/av1"
	"github.com/bluenviron/mediacommon/pkg/codecs/h264"
	"github.com/bluenviron/mediacommon/pkg/codecs/h265"
	"github.com/bluenviron/mediacommon/pkg/codecs/vp9"
	"github.com/pion/rtp"
	rtpcodecs "github.com/pion/rtp/codecs"
	"github.com/pion/rtp/codecs/av1/frame"
	"github.com/pion/webrtc/v4"
	"github.com/valyala/fasttemplate"
	"github.com/vipcxj/conference.go/pkg/common"
	"github.com/vipcxj/conference.go/pkg/samplebuilder"
)

// MuxerVariant is a muxer variant.
type MuxerVariant int

// supported variants.
const (
	MuxerVariantFMP4 MuxerVariant = iota
	MuxerVariantLowLatency
	MuxerVariantMPEGTS
)

type TrackCodec int

const (
	TrackCodecNone TrackCodec = iota
	TrackCodecH264
	TrackCodecH265
	TrackCodecAV1
	TrackCodecVP8
	TrackCodecVP9
	TrackCodecOpus
)

func (c TrackCodec) String() string {
	switch c {
	case TrackCodecNone:
		return "None"
	case TrackCodecH264:
		return "h264"
	case TrackCodecH265:
		return "h265"
	case TrackCodecAV1:
		return "av1"
	case TrackCodecVP8:
		return "vp8"
	case TrackCodecVP9:
		return "vp9"
	case TrackCodecOpus:
		return "opus"
	default:
		return fmt.Sprintf("unknown(%d)", c)
	}
}

func WebrtcTrack2Track(labeledTrack common.LabeledTrack) (rtp.Depacketizer, *gohlslib.Track, TrackCodec) {
	if labeledTrack == nil {
		return nil, nil, TrackCodecNone
	}
	track := labeledTrack.TrackRemote()
	if track == nil {
		return nil, nil, TrackCodecNone
	}
	mimeType := strings.ToLower(track.Codec().MimeType)
	if mimeType == strings.ToLower(webrtc.MimeTypeH264) {
		return &rtpcodecs.H264Packet{}, &gohlslib.Track{
			Codec: codecs.H264{},
		}, TrackCodecH264
	} else if mimeType == strings.ToLower(webrtc.MimeTypeH265) {
		return &rtpcodecs.H265Packet{}, &gohlslib.Track{
			Codec: codecs.H265{},
		}, TrackCodecH265
	} else if mimeType == strings.ToLower(webrtc.MimeTypeVP9) {
		return &rtpcodecs.VP9Packet{}, &gohlslib.Track{
			Codec: codecs.VP9{},
		}, TrackCodecVP9
	} else if mimeType == strings.ToLower(webrtc.MimeTypeAV1) {
		return nil, &gohlslib.Track{
			Codec: codecs.AV1{},
		}, TrackCodecAV1
	} else if mimeType == strings.ToLower(webrtc.MimeTypeOpus) {
		return &rtpcodecs.OpusPacket{}, &gohlslib.Track{
			Codec: codecs.Opus{},
		}, TrackCodecOpus
	} else {
		panic(fmt.Sprintf("Unsupported media type %s", track.Codec().MimeType))
	}
}

// Muxer is a HLS muxer.
type Muxer struct {
	//
	// parameters (all optional except track or AudioTrack or Directory).
	//
	Video common.LabeledTrack
	Audio common.LabeledTrack
	// IndexTemplate in which to save index file.
	IndexTemplate string
	// IndexTemplate in which to save segments.
	SegmentTemplate string
	// Variant to use.
	// It defaults to MuxerVariantFMP4
	Variant MuxerVariant
	// Minimum duration of each segment.
	// A player usually puts 3 segments in a buffer before reproducing the stream.
	// The final segment duration is also influenced by the interval between IDR frames,
	// since the server changes the duration in order to include at least one IDR frame
	// in each segment.
	// It defaults to 6sec.
	SegmentDuration time.Duration
	// Minimum duration of each part.
	// Parts are used in Low-Latency HLS in place of segments.
	// A player usually puts 3 parts in a buffer before reproducing the stream.
	// Part duration is influenced by the distance between video/audio samples
	// and is adjusted in order to produce segments with a similar duration.
	// It defaults to 1sec.
	PartDuration time.Duration
	// Maximum size of each segment.
	// This prevents RAM exhaustion.
	// It defaults to 50MB.
	SegmentMaxSize uint64
	// Use to generate file name
	Prefix            string
	RTPSamplerMaxLate uint16

	videoCodec                    TrackCodec
	audioCodec                    TrackCodec
	videoSampleBuilder            *samplebuilder.SampleBuilder
	audioSampleBuilder            *samplebuilder.SampleBuilder
	videoSegmenter                muxerSegmenter
	audioSegmenter                muxerSegmenter
	storageFactory                storage.Factory
	forceSwitch                   bool
	avFrame                       frame.AV1
	videoTrack                    *gohlslib.Track
	audioTrack                    *gohlslib.Track
	videoSegments                 []muxerSegment
	audioSegments                 []muxerSegment
	directoryTemplate             *fasttemplate.Template
	indexTemplate                 *fasttemplate.Template
	segmentTemplate               *fasttemplate.Template
	currentMasterIndexSegmentPath string
	currentVideoIndexSegmentPath  string
	currentAudioIndexSegmentPath  string
	currentVideoSegmentPath       string
	currentAudioSegmentPath       string
	currentVideoStorage           storage.File
	currentAudioStorage           storage.File
}

type MuxerOption func(muxer *Muxer)

func WithIndexTemplate(indexTemplate string) MuxerOption {
	it := fasttemplate.New(indexTemplate, "${", "}")
	return func(muxer *Muxer) {
		muxer.indexTemplate = it
	}
}

func WithSegmentTemplate(segmentTemplate string) MuxerOption {
	st := fasttemplate.New(segmentTemplate, "${", "}")
	return func(muxer *Muxer) {
		muxer.segmentTemplate = st
	}
}

func NewMuxer(video common.LabeledTrack, audio common.LabeledTrack, directoryTemplate string, options ...MuxerOption) *Muxer {
	videoDepacketizer, videoTrack, videoCodec := WebrtcTrack2Track(video)
	audioDepacketizer, audioTrack, audioCodec := WebrtcTrack2Track(audio)
	m := &Muxer{
		Video:             video,
		videoCodec:        videoCodec,
		videoTrack:        videoTrack,
		Audio:             audio,
		audioCodec:        audioCodec,
		audioTrack:        audioTrack,
		directoryTemplate: fasttemplate.New(directoryTemplate, "${", "}"),
	}
	for _, opt := range options {
		opt(m)
	}
	if m.indexTemplate == nil {
		m.indexTemplate = fasttemplate.New("${hour}-${minute}-${indexType}${ext}", "${", "}")
	}
	if m.segmentTemplate == nil {
		m.segmentTemplate = fasttemplate.New("${hour}-${minute}-${mediaType}-${number}${ext}", "${", "}")
	}
	if m.RTPSamplerMaxLate == 0 {
		m.RTPSamplerMaxLate = 32
	}
	if m.SegmentDuration == 0 {
		m.SegmentDuration = 6 * time.Second
	}
	if m.PartDuration == 0 {
		m.PartDuration = 1 * time.Second
	}
	if m.SegmentMaxSize == 0 {
		m.SegmentMaxSize = 50 * 1024 * 1024
	}

	if m.Variant == MuxerVariantMPEGTS {
		if m.videoTrack != nil {
			if _, ok := m.videoTrack.Codec.(*codecs.H264); !ok {
				panic(fmt.Errorf(
					"the MPEG-TS variant of HLS only supports H264 video. Use the fMP4 or Low-Latency variants instead",
				))
			}
		}
	}

	if videoDepacketizer != nil {
		sampleRate := video.TrackRemote().Codec().ClockRate
		sb := samplebuilder.New(m.RTPSamplerMaxLate, videoDepacketizer, sampleRate)
		m.videoSampleBuilder = sb
	}

	if audioDepacketizer != nil {
		sampleRate := audio.TrackRemote().Codec().ClockRate
		sb := samplebuilder.New(m.RTPSamplerMaxLate, audioDepacketizer, sampleRate)
		m.audioSampleBuilder = sb
	}

	m.storageFactory = storage.NewFactoryDisk(m.IndexTemplate)

	if m.Variant == MuxerVariantMPEGTS {
		m.videoSegmenter = newMuxerSegmenterMPEGTS(
			m.SegmentDuration,
			m.SegmentMaxSize,
			videoTrack,
			nil,
			m.Prefix,
			m.publishVideoSegment,
		)
		m.audioSegmenter = newMuxerSegmenterMPEGTS(
			m.SegmentDuration,
			m.SegmentMaxSize,
			nil,
			audioTrack,
			m.Prefix,
			m.publishAudioSegment,
		)
	} else {
		m.videoSegmenter = newMuxerSegmenterFMP4(
			m.Variant == MuxerVariantLowLatency,
			m.SegmentDuration,
			m.PartDuration,
			m.SegmentMaxSize,
			videoTrack,
			nil,
			m.Prefix,
			m.publishVideoSegment,
			m.publishVideoPart,
		)
		m.audioSegmenter = newMuxerSegmenterFMP4(
			m.Variant == MuxerVariantLowLatency,
			m.SegmentDuration,
			m.PartDuration,
			m.SegmentMaxSize,
			nil,
			audioTrack,
			m.Prefix,
			m.publishAudioSegment,
			m.publishAudioPart,
		)
	}

	return m
}

func calcSuperExpr(v int, postfix string) int {
	postfix = strings.TrimSpace(postfix)
	if len(postfix) == 0 {
		return v
	}
	if strings.HasPrefix(postfix, "|") {
		t, err := strconv.Atoi(strings.TrimSpace(postfix[1:]))
		if err != nil {
			panic(err)
		}
		return v / t * t
	} else {
		panic(fmt.Errorf("invalid expr %d %s", v, postfix))
	}
}

func commonTemplate(w io.Writer, tag string, t time.Time, track common.LabeledTrack) (int, error) {
	t = t.UTC()
	if strings.HasPrefix(tag, "year") {
		return fmt.Fprintf(w, "%04d", calcSuperExpr(t.Year(), tag[4:]))
	} else if strings.HasPrefix(tag, "month") {
		return fmt.Fprintf(w, "%02d", calcSuperExpr(int(t.Month()), tag[5:]))
	} else if strings.HasPrefix(tag, "day") {
		return fmt.Fprintf(w, "%02d", calcSuperExpr(t.Day(), tag[3:]))
	} else if strings.HasPrefix(tag, "hour") {
		return fmt.Fprintf(w, "%02d", calcSuperExpr(t.Hour(), tag[4:]))
	} else if strings.HasPrefix(tag, "minute") {
		return fmt.Fprintf(w, "%02d", calcSuperExpr(t.Minute(), tag[6:]))
	} else if strings.HasPrefix(tag, "second") {
		return fmt.Fprintf(w, "%02d", calcSuperExpr(t.Minute(), tag[6:]))
	} else if strings.HasPrefix(tag, "label:") {
		if track == nil {
			return 0, fmt.Errorf("unsupported tag %v", tag)
		}
		parts := strings.SplitN(tag, ":", 3)
		var labelName string
		if len(parts) > 1 {
			labelName = parts[1]
		} else {
			panic(fmt.Errorf("label name is empty"))
		}
		labelValue, ok := track.Labels()[labelName]
		if !ok {
			if len(parts) > 2 {
				labelValue = parts[2]
			} else {
				panic(fmt.Errorf("invalid label %s", labelName))
			}
		}
		return fmt.Fprint(w, labelValue)
	} else {
		return 0, fmt.Errorf("unsupported tag %v", tag)
	}
}

func (m *Muxer) calcIndexTemplate(t time.Time, hls bool, track common.LabeledTrack) string {
	return m.indexTemplate.ExecuteFuncString(func(w io.Writer, tag string) (int, error) {
		tag = strings.ToLower(strings.TrimSpace(tag))
		switch tag {
		case "ext":
			if hls {
				return fmt.Fprint(w, ".m3u8")
			} else {
				return fmt.Fprint(w, ".mpd")
			}
		case "indextype":
			if track == nil {
				return fmt.Fprint(w, "master")
			} else if track == m.Video {
				return fmt.Fprint(w, "video")
			} else if track == m.Audio {
				return fmt.Fprint(w, "audio")
			} else {
				panic(fmt.Errorf("invalid track"))
			}
		default:
			if track == nil {
				if m.Video != nil {
					track = m.Video
				} else {
					track = m.Audio
				}
			}
			return commonTemplate(w, tag, t, track)
		}
	})
}

func (m *Muxer) calcSegmentTemplate(t time.Time, track common.LabeledTrack) string {
	return m.segmentTemplate.ExecuteFuncString(func(w io.Writer, tag string) (int, error) {
		tag = strings.ToLower(strings.TrimSpace(tag))
		switch tag {
		case "ext":
			if m.Variant == MuxerVariantMPEGTS {
				return fmt.Fprintf(w, ".ts")
			} else {
				return fmt.Fprintf(w, "m4s")
			}
		case "mediatype":
			switch track.TrackRemote().Kind() {
			case webrtc.RTPCodecTypeVideo:
				return fmt.Fprintf(w, "video")
			case webrtc.RTPCodecTypeAudio:
				return fmt.Fprintf(w, "audio")
			default:
				return fmt.Fprintf(w, "unknown")
			}
		default:
			return commonTemplate(w, tag, t, track)
		}
	})
}

func (m *Muxer) nextVideoStorage(ntp time.Time) (storage.File, storage.File) {
	var err error
	old := m.currentVideoStorage
	path := m.calcSegmentTemplate(ntp, m.Video)
	if m.currentVideoStorage == nil {
		m.currentVideoSegmentPath = path
		m.currentVideoStorage, err = m.storageFactory.NewFile(m.currentVideoSegmentPath)
		if err != nil {
			panic(err)
		}
	} else {
		if path != m.currentVideoSegmentPath {
			m.currentVideoSegmentPath = path
			m.currentVideoStorage, err = m.storageFactory.NewFile(m.currentVideoSegmentPath)
			if err != nil {
				panic(err)
			}
		}
	}
	return m.currentVideoStorage, old
}

func (m *Muxer) nextAudioStorage(ntp time.Time) (storage.File, storage.File) {
	var err error
	old := m.currentAudioStorage
	path := m.calcSegmentTemplate(ntp, m.Audio)
	if m.currentAudioStorage == nil {
		m.currentAudioSegmentPath = path
		m.currentAudioStorage, err = m.storageFactory.NewFile(m.currentAudioSegmentPath)
		if err != nil {
			panic(err)
		}
	} else {
		if path != m.currentAudioSegmentPath {
			m.currentAudioSegmentPath = path
			m.currentAudioStorage, err = m.storageFactory.NewFile(m.currentAudioSegmentPath)
			if err != nil {
				panic(err)
			}
		}
	}
	return m.currentAudioStorage, old
}

func (m *Muxer) publishVideoSegment(segment muxerSegment, dts time.Duration, ntp time.Time, forceSwitch bool) storage.File {
	if segment == nil || forceSwitch || (dts-segment.getStartDts()) >= m.SegmentDuration {
		newStorage, oldStorage := m.nextVideoStorage(ntp)
		if newStorage != oldStorage {
			if segment != nil {
				m.videoSegments = append(m.videoSegments, segment)
			}
			return newStorage
		}
	}
	return nil
}

func (m *Muxer) publishVideoPart(part *muxerPart) {

}

func (m *Muxer) publishAudioSegment(segment muxerSegment, dts time.Duration, ntp time.Time, forceSwitch bool) storage.File {
	if segment == nil || forceSwitch || (dts - segment.getStartDts()) >= m.SegmentDuration {
		newStorage, oldStorage := m.nextAudioStorage(ntp)
		if newStorage != oldStorage {
			if segment != nil {
				m.audioSegments = append(m.audioSegments, segment)
			}
			return newStorage
		}
	}
	return nil
}

func (m *Muxer) publishAudioPart(part *muxerPart) {

}

// Close closes a Muxer.
func (m *Muxer) Close() {
	m.videoSegmenter.close()
}

// writeAV1 writes an AV1 temporal unit.
func (m *Muxer) writeAV1(ntp time.Time, pts time.Duration, tu [][]byte) error {
	codec := m.videoTrack.Codec.(*codecs.AV1)
	randomAccess := false

	for _, obu := range tu {
		var h av1.OBUHeader
		err := h.Unmarshal(obu)
		if err != nil {
			return err
		}

		if h.Type == av1.OBUTypeSequenceHeader {
			randomAccess = true

			if !bytes.Equal(codec.SequenceHeader, obu) {
				m.forceSwitch = true
				codec.SequenceHeader = obu
			}
		}
	}

	forceSwitch := false
	if randomAccess && m.forceSwitch {
		m.forceSwitch = false
		forceSwitch = true
	}

	return m.videoSegmenter.writeAV1(ntp, pts, tu, randomAccess, forceSwitch)
}

// writeVP9 writes a VP9 frame.
func (m *Muxer) writeVP9(ntp time.Time, pts time.Duration, frame []byte) error {
	var h vp9.Header
	err := h.Unmarshal(frame)
	if err != nil {
		return err
	}

	codec := m.videoTrack.Codec.(*codecs.VP9)
	randomAccess := false

	if h.FrameType == vp9.FrameTypeKeyFrame {
		randomAccess = true

		if v := h.Width(); v != codec.Width {
			m.forceSwitch = true
			codec.Width = v
		}
		if v := h.Height(); v != codec.Height {
			m.forceSwitch = true
			codec.Height = v
		}
		if h.Profile != codec.Profile {
			m.forceSwitch = true
			codec.Profile = h.Profile
		}
		if h.ColorConfig.BitDepth != codec.BitDepth {
			m.forceSwitch = true
			codec.BitDepth = h.ColorConfig.BitDepth
		}
		if v := h.ChromaSubsampling(); v != codec.ChromaSubsampling {
			m.forceSwitch = true
			codec.ChromaSubsampling = v
		}
		if h.ColorConfig.ColorRange != codec.ColorRange {
			m.forceSwitch = true
			codec.ColorRange = h.ColorConfig.ColorRange
		}
	}

	forceSwitch := false
	if randomAccess && m.forceSwitch {
		m.forceSwitch = false
		forceSwitch = true
	}

	return m.videoSegmenter.writeVP9(ntp, pts, frame, randomAccess, forceSwitch)
}

// writeH26x writes an H264 or an H265 access unit.
func (m *Muxer) writeH26x(ntp time.Time, pts time.Duration, au [][]byte) error {
	randomAccess := false

	switch codec := m.videoTrack.Codec.(type) {
	case *codecs.H265:
		for _, nalu := range au {
			typ := h265.NALUType((nalu[0] >> 1) & 0b111111)

			switch typ {
			case h265.NALUType_IDR_W_RADL, h265.NALUType_IDR_N_LP, h265.NALUType_CRA_NUT:
				randomAccess = true

			case h265.NALUType_VPS_NUT:
				if !bytes.Equal(codec.VPS, nalu) {
					m.forceSwitch = true
					codec.VPS = nalu
				}

			case h265.NALUType_SPS_NUT:
				if !bytes.Equal(codec.SPS, nalu) {
					m.forceSwitch = true
					codec.SPS = nalu
				}

			case h265.NALUType_PPS_NUT:
				if !bytes.Equal(codec.PPS, nalu) {
					m.forceSwitch = true
					codec.PPS = nalu
				}
			}
		}

	case *codecs.H264:
		nonIDRPresent := false

		for _, nalu := range au {
			typ := h264.NALUType(nalu[0] & 0x1F)

			switch typ {
			case h264.NALUTypeIDR:
				randomAccess = true

			case h264.NALUTypeNonIDR:
				nonIDRPresent = true

			case h264.NALUTypeSPS:
				if !bytes.Equal(codec.SPS, nalu) {
					m.forceSwitch = true
					codec.SPS = nalu
				}

			case h264.NALUTypePPS:
				if !bytes.Equal(codec.PPS, nalu) {
					m.forceSwitch = true
					codec.PPS = nalu
				}
			}
		}

		if !randomAccess && !nonIDRPresent {
			return nil
		}
	}

	forceSwitch := false
	if randomAccess && m.forceSwitch {
		m.forceSwitch = false
		forceSwitch = true
	}

	return m.videoSegmenter.writeH26x(ntp, pts, au, randomAccess, forceSwitch)
}

// writeOpus writes Opus packets.
func (m *Muxer) writeOpus(ntp time.Time, pts time.Duration, packets [][]byte) error {
	return m.videoSegmenter.writeOpus(ntp, pts, packets)
}

var (
	naluStartCode       = []byte{0x00, 0x00, 0x01}
	annexbNALUStartCode = []byte{0x00, 0x00, 0x00, 0x01}
)

func split264Nalus(nals []byte) [][]byte {
	start := 0
	length := len(nals)

	var au [][]byte
	for start < length {
		end := bytes.Index(nals[start:], annexbNALUStartCode)
		offset := 4
		if end == -1 {
			end = bytes.Index(nals[start:], naluStartCode)
			offset = 3
		}
		if end == -1 {
			au = append(au, nals[start:])
			break
		}
		if end > 0 {
			au = append(au, nals[start:start+end])
		}
		// next NAL start position
		start += end + offset
	}
	return au
}

func (m *Muxer) WriteVideoRtp(packet *rtp.Packet) {
	if m.videoCodec == TrackCodecAV1 {
		ntp := time.Now()
		pts := durationMp4ToGo(uint64(packet.Timestamp), m.Video.TrackRemote().Codec().ClockRate)
		av1Packet := &rtpcodecs.AV1Packet{}
		if _, err := av1Packet.Unmarshal(packet.Payload); err != nil {
			panic(err)
		}
		frame, err := m.avFrame.ReadFrames(av1Packet)
		if err != nil {
			panic(err)
		}
		if len(frame) > 0 {
			m.writeAV1(ntp, pts, frame)
		}
	} else {
		m.videoSampleBuilder.Push(packet)
		s := m.videoSampleBuilder.Pop()
		if s != nil {
			ntp := time.Now()
			pts := durationMp4ToGo(uint64(s.PacketTimestamp), m.Video.TrackRemote().Codec().ClockRate)
			switch m.videoCodec {
			case TrackCodecH264:
				m.writeH26x(ntp, pts, split264Nalus(s.Data))
			case TrackCodecVP9:
				m.writeVP9(ntp, pts, s.Data)
			default:
				panic(fmt.Errorf("unsupported codec %v", m.videoCodec))
			}
		}
	}
}

func (m *Muxer) WriteAudioRtp(packet *rtp.Packet) {
	m.audioSampleBuilder.Push(packet)
	s := m.audioSampleBuilder.Pop()
	if s != nil {
		ntp := time.Now()
		pts := durationMp4ToGo(uint64(s.PacketTimestamp), m.Audio.TrackRemote().Codec().ClockRate)
		switch m.audioCodec {
		case TrackCodecOpus:
			m.writeOpus(ntp, pts, [][]byte{s.Data})
		default:
			panic(fmt.Errorf("unsupported codec %v", m.audioCodec))
		}
	}
}