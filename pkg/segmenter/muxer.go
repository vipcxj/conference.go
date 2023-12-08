package segmenter

import (
	"bytes"
	"fmt"
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

func WebrtcTrack2Track(track *webrtc.TrackRemote) (rtp.Depacketizer, *gohlslib.Track, TrackCodec) {
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
	TrackRemote webrtc.TrackRemote
	// Directory in which to save segments.
	Directory string
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
	RTPSamplerMaxLate uint32

	codec          TrackCodec
	segmenter      muxerSegmenter
	storageFactory storage.Factory
	forceSwitch    bool
	sampleBuilder  *samplebuilder.SampleBuilder
	avFrame        frame.AV1
	// track.
	track    *gohlslib.Track
	segments []muxerSegment
}

type MuxerOption func(muxer *Muxer)

func NewMuxer(trackRemote *webrtc.TrackRemote, directory string, options ...MuxerOption) *Muxer {
	depacketizer, track, codec := WebrtcTrack2Track(trackRemote)
	m := &Muxer{
		Directory: directory,
		codec:     codec,
		track:     track,
	}
	for _, opt := range options {
		opt(m)
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
		if m.track != nil {
			if _, ok := m.track.Codec.(*codecs.H264); !ok {
				panic(fmt.Errorf(
					"the MPEG-TS variant of HLS only supports H264 video. Use the fMP4 or Low-Latency variants instead",
				))
			}
		}
	}

	sampleRate := trackRemote.Codec().ClockRate

	if depacketizer != nil {
		sb := samplebuilder.New(uint16(m.SegmentMaxSize), depacketizer, sampleRate)
		m.sampleBuilder = sb
	}
	var videoTrack, audioTrack *gohlslib.Track
	if trackRemote.Kind() == webrtc.RTPCodecTypeVideo {
		videoTrack = track
		audioTrack = nil
	} else if trackRemote.Kind() == webrtc.RTPCodecTypeAudio {
		videoTrack = nil
		audioTrack = track
	} else {
		panic(fmt.Errorf("the kind of track remote is unknown"))
	}

	m.storageFactory = storage.NewFactoryDisk(m.Directory)

	if m.Variant == MuxerVariantMPEGTS {
		m.segmenter = newMuxerSegmenterMPEGTS(
			m.SegmentDuration,
			m.SegmentMaxSize,
			videoTrack,
			audioTrack,
			m.Prefix,
			m.storageFactory,
			m.publishSegment,
		)
	} else {
		m.segmenter = newMuxerSegmenterFMP4(
			m.Variant == MuxerVariantLowLatency,
			m.SegmentDuration,
			m.PartDuration,
			m.SegmentMaxSize,
			videoTrack,
			audioTrack,
			m.Prefix,
			m.storageFactory,
			m.publishSegment,
			m.publishPart,
		)
	}

	return m
}

func (m *Muxer) publishSegment(segment muxerSegment) error {
	m.segments = append(m.segments, segment)
	return nil
}

func (m *Muxer) publishPart(part *muxerPart) {

}

// Close closes a Muxer.
func (m *Muxer) Close() {
	m.segmenter.close()
}

// writeAV1 writes an AV1 temporal unit.
func (m *Muxer) writeAV1(ntp time.Time, pts time.Duration, tu [][]byte) error {
	codec := m.track.Codec.(*codecs.AV1)
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

	return m.segmenter.writeAV1(ntp, pts, tu, randomAccess, forceSwitch)
}

// writeVP9 writes a VP9 frame.
func (m *Muxer) writeVP9(ntp time.Time, pts time.Duration, frame []byte) error {
	var h vp9.Header
	err := h.Unmarshal(frame)
	if err != nil {
		return err
	}

	codec := m.track.Codec.(*codecs.VP9)
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

	return m.segmenter.writeVP9(ntp, pts, frame, randomAccess, forceSwitch)
}

// writeH26x writes an H264 or an H265 access unit.
func (m *Muxer) writeH26x(ntp time.Time, pts time.Duration, au [][]byte) error {
	randomAccess := false

	switch codec := m.track.Codec.(type) {
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

	return m.segmenter.writeH26x(ntp, pts, au, randomAccess, forceSwitch)
}

// writeOpus writes Opus packets.
func (m *Muxer) writeOpus(ntp time.Time, pts time.Duration, packets [][]byte) error {
	return m.segmenter.writeOpus(ntp, pts, packets)
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

func (m *Muxer) WriteRtp(packet *rtp.Packet) {
	if m.codec == TrackCodecAV1 {
		ntp := time.Now()
		pts := durationMp4ToGo(uint64(packet.Timestamp), m.TrackRemote.Codec().ClockRate)
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
		m.sampleBuilder.Push(packet)
		s := m.sampleBuilder.Pop()
		if s != nil {
			ntp := time.Now()
			pts := durationMp4ToGo(uint64(s.PacketTimestamp), m.TrackRemote.Codec().ClockRate)
			switch m.codec {
			case TrackCodecH264:
				m.writeH26x(ntp, pts, split264Nalus(s.Data))
			case TrackCodecVP9:
				m.writeVP9(ntp, pts, s.Data)
			case TrackCodecOpus:
				m.writeOpus(ntp, pts, [][]byte{s.Data})
			default:
				panic(fmt.Errorf("unsupported codec %v", m.codec))
			}
		}
	}
}
