package segmenter

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/bluenviron/gohlslib/pkg/codecparams"
	"github.com/bluenviron/gohlslib/pkg/codecs"
	"github.com/bluenviron/gohlslib/pkg/playlist"
	"github.com/bluenviron/mediacommon/pkg/codecs/h264"
	"github.com/bluenviron/mediacommon/pkg/codecs/h265"
	"github.com/bluenviron/mediacommon/pkg/formats/fmp4"
	"github.com/pion/rtp"
	rtpcodecs "github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
	"github.com/vipcxj/conference.go/pkg/common"
	"github.com/vipcxj/conference.go/pkg/samplebuilder"
)

const (
	fmp4StartDTS = 10 * time.Second
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

func AnalyzeTrackCodec(labeledTrack common.LabeledTrack) (rtp.Depacketizer, codecs.Codec, TrackCodec) {
	if labeledTrack == nil {
		return nil, nil, TrackCodecNone
	}
	track := labeledTrack.TrackRemote()
	if track == nil {
		return nil, nil, TrackCodecNone
	}
	mimeType := strings.ToLower(track.Codec().MimeType)
	if mimeType == strings.ToLower(webrtc.MimeTypeH264) {
		return &rtpcodecs.H264Packet{}, &codecs.H264{}, TrackCodecH264
	} else if mimeType == strings.ToLower(webrtc.MimeTypeH265) {
		return &rtpcodecs.H265Packet{}, &codecs.H265{}, TrackCodecH265
	} else if mimeType == strings.ToLower(webrtc.MimeTypeVP9) {
		return &rtpcodecs.VP9Packet{}, &codecs.VP9{}, TrackCodecVP9
	} else if mimeType == strings.ToLower(webrtc.MimeTypeAV1) {
		return nil, &codecs.AV1{}, TrackCodecAV1
	} else if mimeType == strings.ToLower(webrtc.MimeTypeOpus) {
		return &rtpcodecs.OpusPacket{}, &codecs.Opus{}, TrackCodecOpus
	} else {
		panic(fmt.Errorf("Unsupported media type %s", track.Codec().MimeType))
	}
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

type dtsExtractor interface {
	Extract([][]byte, time.Duration) (time.Duration, error)
}

func allocateDTSExtractor(codec codecs.Codec) dtsExtractor {
	switch codec.(type) {
	case *codecs.H265:
		return h265.NewDTSExtractor()

	case *codecs.H264:
		return h264.NewDTSExtractor()
	}
	return nil
}

type Track struct {
	id                int
	segmenter         *Segmenter
	lt                common.LabeledTrack
	playlist          playlist.Media
	codec             TrackCodec
	meta              codecs.Codec
	sb                *samplebuilder.SampleBuilder
	videoDTSExtractor dtsExtractor
	indexUri          string
	segUri            string
	initUri           string
	storage           *Storage
	keyFramed         bool
	forceSwitch       bool
	startNTP          time.Time
	startDTS          time.Duration
	nextVideoSample   *augmentedSample
	currentPart       *fmp4.Part
	resolution        string
	frameRate         float64
}

func NewTrack(id int, segmenter *Segmenter, lt common.LabeledTrack, indexUri string, segUri string, initUri string) (*Track, error) {
	if indexUri == "" {
		return nil, fmt.Errorf("indexUri can't be empty")
	}
	if segUri == "" {
		return nil, fmt.Errorf("segUri can't be empty")
	}
	depacketizer, meta, codec := AnalyzeTrackCodec(lt)
	if codec == TrackCodecNone {
		return nil, fmt.Errorf("unsupported codec %v", lt.TrackRemote().Codec().MimeType)
	}
	var sb *samplebuilder.SampleBuilder
	if depacketizer != nil {
		sb = samplebuilder.New(16, depacketizer, lt.TrackRemote().Codec().ClockRate)
	}
	track := &Track{
		id:        id,
		segmenter: segmenter,
		lt:        lt,
		playlist: playlist.Media{
			Version:             6,
			IndependentSegments: true,
			TargetDuration:      segmenter.segmentDuration,
			PlaylistType:        (*playlist.MediaPlaylistType)(MakeStringPtr(playlist.MediaPlaylistTypeEvent)),
			Map: &playlist.MediaMap{
				URI: initUri,
			},
		},
		meta:     meta,
		codec:    codec,
		sb:       sb,
		indexUri: indexUri,
		segUri:   segUri,
		initUri:  initUri,
		storage:  NewStorage(path.Join(segmenter.directory, segUri)),
	}
	return track, nil
}

func (t *Track) IsAudio() bool {
	return t.lt.TrackRemote().Kind() == webrtc.RTPCodecTypeAudio
}

func (t *Track) RFC6381Codec() string {
	return codecparams.Marshal(t.meta)
}

func (t *Track) writeH26x(ntp time.Time, pts time.Duration, data []byte) error {
	au := split264Nalus(data)
	randomAccess := false

	switch codec := t.meta.(type) {
	case *codecs.H265:
		for _, nalu := range au {
			typ := h265.NALUType((nalu[0] >> 1) & 0b111111)

			switch typ {
			case h265.NALUType_IDR_W_RADL, h265.NALUType_IDR_N_LP, h265.NALUType_CRA_NUT:
				randomAccess = true

			case h265.NALUType_VPS_NUT:
				if !bytes.Equal(codec.VPS, nalu) {
					t.forceSwitch = true
					codec.VPS = nalu
				}

			case h265.NALUType_SPS_NUT:
				if !bytes.Equal(codec.SPS, nalu) {
					t.forceSwitch = true
					codec.SPS = nalu

					var sps h265.SPS
					err := sps.Unmarshal(codec.SPS)
					if err != nil {
						return err
					}

					t.resolution = strconv.FormatInt(int64(sps.Width()), 10) + "x" + strconv.FormatInt(int64(sps.Height()), 10)

					f := sps.FPS()
					if f != 0 {
						t.frameRate = f
					}
				}

			case h265.NALUType_PPS_NUT:
				if !bytes.Equal(codec.PPS, nalu) {
					t.forceSwitch = true
					codec.PPS = nalu

					var sps h264.SPS
					err := sps.Unmarshal(codec.SPS)
					if err != nil {
						return err
					}

					t.resolution = strconv.FormatInt(int64(sps.Width()), 10) + "x" + strconv.FormatInt(int64(sps.Height()), 10)

					f := sps.FPS()
					if f != 0 {
						t.frameRate = f
					}
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
					t.forceSwitch = true
					codec.SPS = nalu
				}

			case h264.NALUTypePPS:
				if !bytes.Equal(codec.PPS, nalu) {
					t.forceSwitch = true
					codec.PPS = nalu
				}
			}
		}

		if !randomAccess && !nonIDRPresent {
			return nil
		}
	}

	forceSwitch := false
	if randomAccess && t.forceSwitch {
		t.forceSwitch = false
		forceSwitch = true
	}

	var dts time.Duration

	if !t.keyFramed {
		// skip sample silently until we find one with an IDR
		if !randomAccess {
			return nil
		}

		t.keyFramed = true
		t.videoDTSExtractor = allocateDTSExtractor(t.meta)
	}

	var err error
	dts, err = t.videoDTSExtractor.Extract(au, pts)
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

	return t.writeVideo(
		randomAccess,
		forceSwitch,
		&augmentedSample{
			PartSample: *ps,
			dts:        dts,
			ntp:        ntp,
		})
}

func (t *Track) updateIndex() error {

}

func (t *Track) writeVideo(
	randomAccess bool,
	forceSwitch bool,
	sample *augmentedSample,
) error {
	// add a starting DTS to avoid a negative BaseTime
	sample.dts += fmp4StartDTS

	// BaseTime is still negative, this is not supported by fMP4. Reject the sample silently.
	if (sample.dts - t.startDTS) < 0 {
		return nil
	}

	// the first sample
	if t.nextVideoSample == nil {
		t.startNTP = sample.ntp
		t.startDTS = sample.dts
	}

	// put samples into a queue in order to
	// - compute sample duration
	// - check if next sample is IDR
	sample, t.nextVideoSample = t.nextVideoSample, sample
	if sample == nil {
		return nil
	}
	duration := t.nextVideoSample.dts - sample.dts
	sample.Duration = uint32(durationGoToMp4(duration, 90000))
	if t.currentPart == nil {
		t.currentPart = &fmp4.Part{
			Tracks: []*fmp4.PartTrack{{
				ID:       t.id,
				BaseTime: durationGoToMp4(t.startDTS, 90000),
			}},
		}
	}
	t.currentPart.Tracks[0].Samples = append(t.currentPart.Tracks[0].Samples, &sample.PartSample)

	if randomAccess && duration > time.Duration(t.segmenter.segmentDuration)*time.Second {
		writer := t.storage.Writer()
		err := t.currentPart.Marshal(writer)
		if err != nil {
			return err
		}
		offset := t.storage.CurrentPartOffset()
		size := uint64(t.storage.NextPart())
		t.playlist.Segments = append(t.playlist.Segments, &playlist.MediaSegment{
			Duration:        sample.dts - t.startDTS,
			URI:             t.segUri,
			DateTime:        &t.startNTP,
			ByteRangeStart:  &offset,
			ByteRangeLength: &size,
		})
		err = t.segmenter.UpdateIndex()
		if err != nil {
			return err
		}
		err = t.updateIndex()
		if err != nil {
			return err
		}

		t.startNTP = t.nextVideoSample.ntp
		t.startDTS = t.nextVideoSample.dts
		t.currentPart = &fmp4.Part{
			Tracks: []*fmp4.PartTrack{{
				ID:       t.id,
				BaseTime: durationGoToMp4(t.startDTS, 90000),
				Samples:  []*fmp4.PartSample{&t.nextVideoSample.PartSample},
			}},
		}
	}
	return nil
}

func (t *Track) writeData(ntp time.Time, pts time.Duration, data []byte) error {
	switch t.codec {
	case TrackCodecH264:
		return t.writeH26x(ntp, pts, data)
	default:
		return fmt.Errorf("unsupport codec %v", t.codec)
	}
}

func (t *Track) bandwidth() (int, int) {
	if len(t.playlist.Segments) == 0 {
		return 0, 0
	}

	var maxBandwidth uint64
	var sizes uint64
	var durations time.Duration

	for _, seg := range t.playlist.Segments {
		bandwidth := 8 * *seg.ByteRangeLength * uint64(time.Second) / uint64(seg.Duration)
		if bandwidth > maxBandwidth {
			maxBandwidth = bandwidth
		}
		sizes += *seg.ByteRangeLength
		durations += seg.Duration
	}

	averageBandwidth := 8 * sizes * uint64(time.Second) / uint64(durations)

	return int(maxBandwidth), int(averageBandwidth)
}

type Segmenter struct {
	tracks               []*Track
	multivariantPlaylist playlist.Multivariant
	indexUri             string
	segmentDuration      int
	directory            string
	indexCreated         bool
}

func generatePrefix(lt common.LabeledTrack) string {
	var kind string
	switch lt.TrackRemote().Kind() {
	case webrtc.RTPCodecTypeVideo:
		kind = "vid"
	case webrtc.RTPCodecTypeAudio:
		kind = "aud"
	default:
		kind = "unk"
	}
	return fmt.Sprintf("%s-%s", kind, lt.ID())
}

func NewSegmenter(labeledTracks []common.LabeledTrack, directory string, multivariantPlaylistName string, segmentDuration int) (*Segmenter, error) {
	segmenter := &Segmenter{
		indexUri: multivariantPlaylistName,
		multivariantPlaylist: playlist.Multivariant{
			Version:             6,
			IndependentSegments: true,
		},
		segmentDuration: segmentDuration,
		directory:       directory,
	}
	var tracks []*Track
	for _, lt := range labeledTracks {
		prefix := generatePrefix(lt)
		indexUri := fmt.Sprintf("%s.m3u8", prefix)
		initUri := fmt.Sprintf("%s-init.mp4", prefix)
		segUri := fmt.Sprintf("%s-segments.m4s", prefix)
		track, err := NewTrack(1, segmenter, lt, indexUri, segUri, initUri)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, track)
		if track.IsAudio() {
			var defaultAudio bool
			if segmenter.multivariantPlaylist.Renditions == nil {
				defaultAudio = true
			}
			segmenter.multivariantPlaylist.Renditions = append(segmenter.multivariantPlaylist.Renditions, &playlist.MultivariantRendition{
				Type:       playlist.MultivariantRenditionTypeAudio,
				GroupID:    "audio",
				URI:        indexUri,
				Name:       prefix,
				Autoselect: true,
				Default:    defaultAudio,
			})
		} else {
			segmenter.multivariantPlaylist.Variants = append(segmenter.multivariantPlaylist.Variants, &playlist.MultivariantVariant{
				URI: indexUri,
			})
		}
	}
	if segmenter.multivariantPlaylist.Renditions != nil {
		for _, variant := range segmenter.multivariantPlaylist.Variants {
			variant.Audio = "audio"
		}
	}
	segmenter.tracks = tracks
	return segmenter, nil
}

func (s *Segmenter) FindTrackBySSID(ssrc uint32) *Track {
	for _, track := range s.tracks {
		if track.lt.TrackRemote().SSRC() == webrtc.SSRC(ssrc) || track.lt.TrackRemote().RtxSSRC() == webrtc.SSRC(ssrc) {
			return track
		}
	}
	return nil
}

func (s *Segmenter) FindVariantByTrack(track *Track) *playlist.MultivariantVariant {
	for _, variant := range s.multivariantPlaylist.Variants {
		if variant.URI == track.indexUri {
			return variant
		}
	}
	return nil
}

func (s *Segmenter) UpdateIndex() error {
	var masterChanged bool
	for _, track := range s.tracks {
		variant := s.FindVariantByTrack(track)
		if variant == nil {
			continue
		}
		maxBandwidth, avgBindwidth := track.bandwidth()
		if variant.Bandwidth != maxBandwidth {
			variant.Bandwidth = maxBandwidth
			masterChanged = true
		}
		if variant.AverageBandwidth == nil || *variant.AverageBandwidth != avgBindwidth {
			variant.AverageBandwidth = &avgBindwidth
			masterChanged = true
		}
		if len(variant.Codecs) == 0 {
			codecs := track.RFC6381Codec()
			variant.Codecs = []string{codecs}
			masterChanged = true
		}
		if variant.Resolution != track.resolution {
			variant.Resolution = track.resolution
			masterChanged = true
		}
		if variant.FrameRate == nil || *variant.FrameRate != track.frameRate {
			variant.FrameRate = &track.frameRate
			masterChanged = true
		}
	}
	if !s.indexCreated || masterChanged {
		s.indexCreated = true
		content, err := s.multivariantPlaylist.Marshal()
		if err != nil {
			return err
		}
		err = os.WriteFile(path.Join(s.directory, s.indexUri), content, 0)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Segmenter) WriteRtp(packet *rtp.Packet) error {
	track := s.FindTrackBySSID(packet.SSRC)
	if track == nil {
		return fmt.Errorf("ssrc not found: %d", packet.SSRC)
	}
	track.sb.Push(packet)
	sample := track.sb.Pop()
	if sample != nil {
		ntp := time.Now()
		pts := durationMp4ToGo(uint64(sample.PacketTimestamp), track.lt.TrackRemote().Codec().ClockRate)
		return track.writeData(ntp, pts, sample.Data)
	}
	return nil
}
