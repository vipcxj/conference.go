package segmenter

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
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
	"github.com/google/uuid"
	"github.com/pion/rtp"
	rtpcodecs "github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
	"github.com/valyala/fasttemplate"
	"github.com/vipcxj/conference.go/pkg/common"
	"github.com/vipcxj/conference.go/pkg/samplebuilder"
)

type augmentedSample struct {
	fmp4.PartSample
	dts time.Duration
}

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
		panic(fmt.Errorf("unsupported media type %s", track.Codec().MimeType))
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
		return NewH264DTSExtractor()
	}
	return nil
}

func ntpToDts(accordingDts time.Duration, accordingNtp time.Time, ntp time.Time) time.Duration {
	return accordingDts + ntp.Sub(accordingNtp)
}

func dtsToNtp(accordingDts time.Duration, accordingNtp time.Time, dts time.Duration) time.Time {
	return accordingNtp.Add(dts - accordingDts)
}

type TrackContext struct {
	First        bool
	Last         bool
	Playlist     *playlist.Media
	InitPath     string
	IndexPath    string
	SegmentPath  string
	Start        time.Time
	End          time.Time
	Codec        string
	Width        int
	Height       int
	FrameRate    float64
	MaxBandwidth int
	AvgBandwidth int
	Audio        bool
}

type Track struct {
	id                int
	segmenter         *Segmenter
	lt                common.LabeledTrack
	ready             bool
	playlist          playlist.Media
	codec             TrackCodec
	meta              codecs.Codec
	sb                *samplebuilder.SampleBuilder
	videoDTSExtractor dtsExtractor
	prefix            string
	indexUri          string
	segUri            string
	initUri           string
	storage           *Storage
	keyFramed         bool
	forceSwitch       bool
	start             bool
	initCreated       bool
	beginNTP          time.Time
	startNTP          time.Time
	startDTS          time.Duration
	lastDTSFilled     bool
	lastDTS           time.Duration
	endNTP            time.Time
	nextVideoSample   *augmentedSample
	currentPart       *fmp4.Part
	width             int
	height            int
	frameRate         float64
	resolution        string
	maxBandwidth      int
	avgBandwidth      int
	closed            bool
}

func NewTrack(id int, segmenter *Segmenter, lt common.LabeledTrack) (*Track, error) {
	track := &Track{
		id:        id,
		segmenter: segmenter,
		lt:        lt,
	}
	return track, nil
}

func (t *Track) CreateContext() *TrackContext {
	first := len(t.playlist.Segments) == 1
	return &TrackContext{
		First:        first,
		Last:         t.closed,
		Playlist:     &t.playlist,
		InitPath:     t.segmenter.fullPath(t.initUri, false),
		IndexPath:    t.segmenter.fullPath(t.indexUri, false),
		SegmentPath:  t.segmenter.fullPath(t.segUri, false),
		Start:        t.beginNTP,
		End:          t.endNTP,
		Codec:        t.RFC6381Codec(),
		Width:        t.width,
		Height:       t.height,
		FrameRate:    t.frameRate,
		AvgBandwidth: t.avgBandwidth,
		MaxBandwidth: t.maxBandwidth,
		Audio:        t.lt.TrackRemote().Kind() == webrtc.RTPCodecTypeAudio,
	}
}

func (t *Track) TryReady() (bool, error) {
	if t.ready {
		return true, nil
	}
	if t.lt.TrackRemote().Codec().MimeType == "" {
		return false, nil
	}
	prefix := generatePrefix(t.lt)
	indexUri := fmt.Sprintf("%s.m3u8", prefix)
	initUri := fmt.Sprintf("%s-init.mp4", prefix)
	segUri := fmt.Sprintf("%s-segments.m4s", prefix)
	depacketizer, meta, codec := AnalyzeTrackCodec(t.lt)
	if codec == TrackCodecNone {
		return false, fmt.Errorf("unsupported codec %v", t.lt.TrackRemote().Codec().MimeType)
	}
	var sb *samplebuilder.SampleBuilder
	if depacketizer != nil {
		sb = samplebuilder.New(16, depacketizer, t.lt.TrackRemote().Codec().ClockRate, samplebuilder.WithPacketReleaseHandler(t.segmenter.packetReleaseHandler))
	}
	t.playlist = playlist.Media{
		Version:             6,
		IndependentSegments: true,
		TargetDuration:      t.segmenter.segmentDuration + t.segmenter.gopSize,
		PlaylistType:        (*playlist.MediaPlaylistType)(MakeStringPtr(playlist.MediaPlaylistTypeEvent)),
		Map: &playlist.MediaMap{
			URI: initUri,
		},
	}
	t.meta = meta
	t.codec = codec
	t.sb = sb
	t.prefix = prefix
	t.indexUri = indexUri
	t.segUri = segUri
	t.initUri = initUri
	t.ready = true
	return true, nil
}

func (t *Track) IsAudio() bool {
	return t.lt.TrackRemote().Kind() == webrtc.RTPCodecTypeAudio
}

func (t *Track) RFC6381Codec() string {
	return codecparams.Marshal(t.meta)
}

func (t *Track) writeH26x(pts time.Duration, data []byte) error {
	sampleRate := t.lt.TrackRemote().Codec().ClockRate
	if sampleRate != 90000 {
		return fmt.Errorf("h26x video should be 90000 sample rate, but got %d", sampleRate)
	}
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

					t.width = sps.Width()
					t.height = sps.Height()
					t.resolution = strconv.FormatInt(int64(t.width), 10) + "x" + strconv.FormatInt(int64(t.height), 10)

					f := sps.FPS()
					if f != 0 {
						t.frameRate = f
					}
				}

			case h265.NALUType_PPS_NUT:
				if !bytes.Equal(codec.PPS, nalu) {
					t.forceSwitch = true
					codec.PPS = nalu
				}
			}
		}

	case *codecs.H264:
		nonIDRPresent := false
		hasSps := false
		hasPps := false

		for _, nalu := range au {
			typ := h264.NALUType(nalu[0] & 0x1F)

			switch typ {
			case h264.NALUTypeIDR:
				randomAccess = true

			case h264.NALUTypeNonIDR:
				nonIDRPresent = true

			case h264.NALUTypeSPS:
				hasSps = true
				if !bytes.Equal(codec.SPS, nalu) {
					t.forceSwitch = true
					codec.SPS = nalu

					var sps h264.SPS
					err := sps.Unmarshal(codec.SPS)
					if err != nil {
						return err
					}

					t.width = sps.Width()
					t.height = sps.Height()
					t.resolution = strconv.FormatInt(int64(t.width), 10) + "x" + strconv.FormatInt(int64(t.height), 10)

					f := sps.FPS()
					if f != 0 {
						t.frameRate = f
					}
				}

			case h264.NALUTypePPS:
				hasPps = true
				if !bytes.Equal(codec.PPS, nalu) {
					t.forceSwitch = true
					codec.PPS = nalu
				}
			}
		}

		if !randomAccess && !nonIDRPresent {
			return nil
		} else if !hasSps || !hasPps {
			if !hasSps {
				if codec.SPS == nil {
					return nil
				}
				au = append(au, codec.SPS)
			}
			if !hasPps && codec.PPS != nil {
				au = append(au, codec.PPS)
			}
		}
	}

	// if randomAccess {
	// 	fmt.Printf("write key sample at pts %v\n", pts)
	// } else {
	// 	fmt.Printf("write nonkey sample at pts %v\n", pts)
	// }

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
		fmt.Printf("unable to extract DTS: %v\n", err)
		if t.lastDTSFilled {
			dts = min(t.lastDTS+time.Millisecond, pts)
		} else {
			dts = pts
		}
	}
	t.lastDTSFilled = true
	t.lastDTS = dts

	ps, err := fmp4.NewPartSampleH26x(
		int32(durationGoToMp4(pts-dts, sampleRate)),
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
		})
}

func (t *Track) writeInit() error {
	var init fmp4.Init
	init.Tracks = []*fmp4.InitTrack{{
		ID:        t.id,
		TimeScale: t.lt.TrackRemote().Codec().ClockRate,
		Codec:     codecs.ToFMP4(t.meta),
	}}
	storage := NewStorage(t.segmenter.fullPath(t.initUri, true))
	defer storage.Close()
	err := init.Marshal(storage.Writer())
	return err
}

func (t *Track) updateIndex() error {
	if t.closed {
		t.playlist.Endlist = true
	}
	t.maxBandwidth, t.avgBandwidth = t.bandwidth()
	content, err := t.playlist.Marshal()
	if err != nil {
		return err
	}
	fpath := t.segmenter.fullPath(t.indexUri, true)
	err = MakeSureDirOf(fpath)
	if err != nil {
		return err
	}
	err = os.WriteFile(fpath, content, 0o664)
	return err
}

func (t *Track) ntpToDts(ntp time.Time) time.Duration {
	return ntpToDts(t.startDTS, t.startNTP, ntp)
}

func (t *Track) flushSegment(dts time.Duration, forceUpdateInit bool) error {
	if t.currentPart == nil {
		return nil
	}
	if t.storage == nil {
		t.storage = NewStorage(t.segmenter.fullPath(t.segUri, true))
	}
	writer := t.storage.Writer()
	err := t.currentPart.Marshal(writer)
	if err != nil {
		return err
	}
	offset := t.storage.CurrentPartOffset()
	size := uint64(t.storage.NextPart())
	t.playlist.Segments = append(t.playlist.Segments, &playlist.MediaSegment{
		Duration:        dts - t.startDTS,
		URI:             t.segUri,
		DateTime:        &t.startNTP,
		ByteRangeStart:  &offset,
		ByteRangeLength: &size,
	})
	if forceUpdateInit || !t.initCreated {
		t.initCreated = true
		err = t.writeInit()
		if err != nil {
			return err
		}
	}
	t.endNTP = dtsToNtp(t.startDTS, t.startNTP, dts)
	err = t.updateIndex()
	if err != nil {
		return err
	}
	// when closed, call segmenter.UpdateIndex() in segmenter.Close method
	if !t.closed {
		err = t.segmenter.UpdateIndex(true)
	}
	return err
}

func (t *Track) writeVideo(
	randomAccess bool,
	forceSwitch bool,
	sample *augmentedSample,
) error {
	if sample != nil {
		// add a starting DTS to avoid a negative BaseTime
		sample.dts += fmp4StartDTS

		// BaseTime is still negative, this is not supported by fMP4. Reject the sample silently.
		if (sample.dts - t.startDTS) < 0 {
			return nil
		}

		// the first sample
		if t.nextVideoSample == nil {
			t.startNTP = time.Now().UTC()
			t.beginNTP = t.startNTP
			t.startDTS = sample.dts
			if !t.start {
				t.start = true
				t.segmenter.setStart(t.startDTS, t.startNTP)
			}
		}
	}

	// put samples into a queue in order to
	// - compute sample duration
	// - check if next sample is IDR
	sample, t.nextVideoSample = t.nextVideoSample, sample
	if sample == nil {
		return nil
	}
	var nextDts time.Duration
	if t.nextVideoSample != nil {
		nextDts = t.nextVideoSample.dts
	} else {
		nextDts = t.ntpToDts(time.Now().UTC())
	}
	duration := nextDts - sample.dts
	sampleRate := t.lt.TrackRemote().Codec().ClockRate
	sample.Duration = uint32(durationGoToMp4(duration, sampleRate))
	if t.currentPart == nil {
		t.currentPart = &fmp4.Part{
			Tracks: []*fmp4.PartTrack{{
				ID:       t.id,
				BaseTime: durationGoToMp4(t.startDTS-t.segmenter.start.DTS, sampleRate),
			}},
		}
	}
	t.currentPart.Tracks[0].Samples = append(t.currentPart.Tracks[0].Samples, &sample.PartSample)

	if t.nextVideoSample == nil || (randomAccess && (forceSwitch || nextDts-t.startDTS > time.Duration(t.segmenter.segmentDuration)*time.Second)) {
		// if t.nextVideoSample == nil, we are closing the track
		err := t.flushSegment(nextDts, forceSwitch)
		if err != nil {
			return err
		}

		if t.nextVideoSample != nil {
			t.startNTP = time.Now().UTC()
			t.startDTS = t.nextVideoSample.dts
			t.currentPart = &fmp4.Part{
				Tracks: []*fmp4.PartTrack{{
					ID:       t.id,
					BaseTime: durationGoToMp4(t.startDTS-t.segmenter.start.DTS, sampleRate),
					Samples:  []*fmp4.PartSample{&t.nextVideoSample.PartSample},
				}},
			}
		} else {
			t.startNTP = time.Time{}
			t.startDTS = time.Duration(0)
			t.currentPart = nil
		}
	}
	return nil
}

func (t *Track) WriteData(pts time.Duration, data []byte) error {
	if t.closed {
		return fmt.Errorf("track closed")
	}
	switch t.codec {
	case TrackCodecH264:
		return t.writeH26x(pts, data)
	default:
		return fmt.Errorf("unsupport codec %v", t.codec)
	}
}

func (t *Track) Close() error {
	if t.closed || !t.ready {
		return nil
	}
	t.closed = true
	err := t.writeVideo(false, false, nil)
	if err != nil {
		return err
	}
	if t.storage != nil {
		t.storage.Close()
	}
	return nil
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

type Instant struct {
	DTS time.Duration
	NPT time.Time
}

type Option func(o *Segmenter)

type SegmentContext struct {
	First    bool
	Last     bool
	Playlist *playlist.Multivariant
	Base     string
	Path     string
	Start    *Instant
	End      *Instant
	Tracks   []*TrackContext
}

type Segmenter struct {
	ready                bool
	tracks               []*Track
	multivariantPlaylist playlist.Multivariant
	baseTemplate         *fasttemplate.Template
	base                 string
	indexUriTemplate     *fasttemplate.Template
	indexUri             string
	segmentDuration      int
	gopSize              int
	start                *Instant
	end                  *Instant
	directoryTemplate    *fasttemplate.Template
	directory            string
	indexCreated         bool
	closing              bool
	closed               bool
	packetReleaseHandler func(*rtp.Packet, []byte)
	segmentHandler       func(*SegmentContext)
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

func extractLabels(track *Track) map[string]string {
	return track.lt.Labels()
}

func GetCommonLabel[T any, TS []T](label string, thingsWithLabels TS, labelsExtractor func(t T) map[string]string) (string, bool) {
	start := false
	var commonValue string
	for _, thingWithLabel := range thingsWithLabels {
		labels := labelsExtractor(thingWithLabel)
		if labels == nil {
			return "", false
		}
		value, ok := labels[label]
		if !ok {
			return "", false
		}
		if !start {
			start = true
			commonValue = value
		} else if value != commonValue {
			return "", false
		}
	}
	return commonValue, start
}

func commonTemplate(w io.Writer, tag string, t time.Time, tracks []*Track) (int, error) {
	var err error
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
		if tracks == nil {
			return 0, fmt.Errorf("unsupported tag %v", tag)
		}
		parts := strings.SplitN(tag, ":", 3)
		var labelName string
		if len(parts) > 1 {
			labelName = parts[1]
		} else {
			return 0, fmt.Errorf("label name is empty")
		}

		labelValue, ok := GetCommonLabel(labelName, tracks, extractLabels)
		if !ok {
			if len(parts) > 2 {
				labelValue = parts[2]
			} else {
				return 0, fmt.Errorf("invalid label %s", labelName)
			}
		}
		return fmt.Fprint(w, labelValue)
	} else if strings.HasPrefix(tag, "uuid") {
		parts := strings.SplitN(tag, ":", 2)
		l := 32
		if len(parts) > 1 {
			l, err = strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return 0, fmt.Errorf("invalid uuid expr, %s", tag)
			}
			if l < 0 || l > 32 {
				return 0, fmt.Errorf("invalid uuid expr, uuid len out of boundary, %s", tag)
			}
		}
		u := uuid.Must(uuid.NewRandom())
		s := hex.EncodeToString(u[:])
		return fmt.Fprintf(w, s[0:l])
	} else {
		return 0, fmt.Errorf("unsupported tag %v", tag)
	}
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
	return fmt.Sprintf("%s-%v", kind, lt.TrackRemote().SSRC())
}

func NewSegmenter(
	labeledTracks []common.LabeledTrack,
	directoryTemplate string,
	indexTemplate string,
	segmentDuration int,
	gopSize int,
	options ...Option,
) (*Segmenter, error) {
	segmenter := &Segmenter{
		indexUriTemplate: fasttemplate.New(indexTemplate, "{{", "}}"),
		multivariantPlaylist: playlist.Multivariant{
			Version:             6,
			IndependentSegments: true,
		},
		segmentDuration:   segmentDuration,
		gopSize:           gopSize,
		directoryTemplate: fasttemplate.New(directoryTemplate, "{{", "}}"),
	}
	for _, opt := range options {
		if opt != nil {
			opt(segmenter)
		}
	}
	var tracks []*Track
	for _, lt := range labeledTracks {
		track, err := NewTrack(1, segmenter, lt)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, track)
	}
	segmenter.tracks = tracks
	return segmenter, nil
}

func WithBaseTemplate(template string) Option {
	return func(o *Segmenter) {
		o.baseTemplate = fasttemplate.New(template, "{{", "}}")
	}
}

// WithPacketReleaseHandler sets a callback that is called when the
// builder is about to release some packet.
func WithPacketReleaseHandler(h func(*rtp.Packet, []byte)) Option {
	return func(s *Segmenter) {
		s.packetReleaseHandler = h
	}
}

func WithSegmentHandler(h func(*SegmentContext)) Option {
	return func(s *Segmenter) {
		s.segmentHandler = h
	}
}

func (s *Segmenter) CreateContext(first bool) *SegmentContext {
	tracks := make([]*TrackContext, len(s.tracks))
	for i, t := range s.tracks {
		tracks[i] = t.CreateContext()
	}
	return &SegmentContext{
		First:    first,
		Last:     s.closing || s.closed,
		Playlist: &s.multivariantPlaylist,
		Base:     s.Base(),
		Path:     s.fullPath(s.IndexUri(), false),
		Start:    s.start,
		End:      s.end,
		Tracks:   tracks,
	}
}

func (s *Segmenter) TryReady() (bool, error) {
	if s.ready {
		return true, nil
	}
	readys := 0
	for _, track := range s.tracks {
		ready, err := track.TryReady()
		if err != nil {
			return false, err
		}
		if ready {
			readys++
		}
	}
	if readys == len(s.tracks) {
		s.ready = true
		for _, track := range s.tracks {
			if track.IsAudio() {
				var defaultAudio bool
				if s.multivariantPlaylist.Renditions == nil {
					defaultAudio = true
				}
				s.multivariantPlaylist.Renditions = append(s.multivariantPlaylist.Renditions, &playlist.MultivariantRendition{
					Type:       playlist.MultivariantRenditionTypeAudio,
					GroupID:    "audio",
					URI:        track.indexUri,
					Name:       track.prefix,
					Autoselect: true,
					Default:    defaultAudio,
				})
			} else {
				s.multivariantPlaylist.Variants = append(s.multivariantPlaylist.Variants, &playlist.MultivariantVariant{
					URI: track.indexUri,
				})
			}
		}
		if s.multivariantPlaylist.Renditions != nil {
			for _, variant := range s.multivariantPlaylist.Variants {
				variant.Audio = "audio"
			}
		}
		return true, nil
	} else {
		return false, nil
	}
}

func (s *Segmenter) IsReady() bool {
	return s.ready
}

func (s *Segmenter) calcBaseTemplate(t time.Time) string {
	if s.baseTemplate == nil {
		return ""
	} else {
		return s.baseTemplate.ExecuteFuncString(func(w io.Writer, tag string) (int, error) {
			tag = strings.ToLower(strings.TrimSpace(tag))
			return commonTemplate(w, tag, t, s.tracks)
		})
	}
}

func (s *Segmenter) calcDirectoryTemplate(t time.Time) string {
	return s.directoryTemplate.ExecuteFuncString(func(w io.Writer, tag string) (int, error) {
		tag = strings.ToLower(strings.TrimSpace(tag))
		return commonTemplate(w, tag, t, s.tracks)
	})
}

func (s *Segmenter) calcIndexTemplate(t time.Time) string {
	return s.indexUriTemplate.ExecuteFuncString(func(w io.Writer, tag string) (int, error) {
		tag = strings.ToLower(strings.TrimSpace(tag))
		switch tag {
		case "ext":
			return fmt.Fprint(w, ".m3u8")
		default:
			return commonTemplate(w, tag, t, s.tracks)
		}
	})
}

func (s *Segmenter) setStart(dts time.Duration, ntp time.Time) {
	if s.start == nil {
		s.start = &Instant{
			DTS: dts,
			NPT: ntp,
		}
		s.base = s.calcBaseTemplate(ntp)
		s.indexUri = s.calcIndexTemplate(ntp)
		s.directory = s.calcDirectoryTemplate(ntp)
	}
}

func (s *Segmenter) FindTrackBySSID(ssrc uint32) *Track {
	for _, track := range s.tracks {
		if track.lt.TrackRemote().SSRC() == webrtc.SSRC(ssrc) || track.lt.TrackRemote().RtxSSRC() == webrtc.SSRC(ssrc) {
			if track.lt.TrackRemote().SSRC() != webrtc.SSRC(ssrc) {
				fmt.Print("find packet from rtx.\n")
			}
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

func (s *Segmenter) IndexUri() string {
	if s.start == nil {
		panic(fmt.Errorf("index uri not ready yet"))
	}
	return s.indexUri
}

func (s *Segmenter) Base() string {
	if s.start == nil {
		panic(fmt.Errorf("base not ready yet"))
	}
	return s.base
}

func (s *Segmenter) Directory() string {
	if s.start == nil {
		panic(fmt.Errorf("directory not ready yet"))
	}
	return s.directory
}

func (s *Segmenter) fullPath(rPath string, withBase bool) string {
	if withBase {
		return path.Join(s.Base(), s.Directory(), rPath)
	} else {
		return path.Join(s.Directory(), rPath)
	}
}

func (s *Segmenter) UpdateIndex(forceCreate bool) error {
	first := !s.indexCreated
	if !forceCreate && first {
		// used in Close method. Haven't write anything, so need not crate index.
		return nil
	}
	var masterChanged bool
	for _, track := range s.tracks {
		variant := s.FindVariantByTrack(track)
		if variant == nil {
			continue
		}
		if variant.Bandwidth != track.maxBandwidth {
			variant.Bandwidth = track.maxBandwidth
			masterChanged = true
		}
		if variant.AverageBandwidth == nil || *variant.AverageBandwidth != track.avgBandwidth {
			variant.AverageBandwidth = &track.avgBandwidth
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
			if track.frameRate != 0 {
				variant.FrameRate = &track.frameRate
				masterChanged = true
			}
		}
	}
	if !s.indexCreated || masterChanged {
		content, err := s.multivariantPlaylist.Marshal()
		if err != nil {
			return err
		}
		fpath := s.fullPath(s.IndexUri(), true)
		err = MakeSureDirOf(fpath)
		if err != nil {
			return err
		}
		err = os.WriteFile(fpath, content, 0o664)
		if err != nil {
			return err
		}
		if !s.indexCreated {
			s.indexCreated = true
		}
	}
	if s.segmentHandler != nil {
		context := s.CreateContext(first)
		s.segmentHandler(context)
	}
	return nil
}

func (s *Segmenter) WriteRtp(packet *rtp.Packet, buf []byte) error {
	if s.closed {
		return fmt.Errorf("segmenter closed")
	}
	track := s.FindTrackBySSID(packet.SSRC)
	if track == nil {
		return fmt.Errorf("ssrc not found: %d", packet.SSRC)
	}
	track.sb.Push(packet, buf)
	sample := track.sb.Pop()
	if sample != nil {
		pts := durationMp4ToGo(uint64(sample.PacketTimestamp), track.lt.TrackRemote().Codec().ClockRate)
		return track.WriteData(pts, sample.Data)
	}
	return nil
}

func (s *Segmenter) StartInstant() *Instant {
	return s.start
}

func (s *Segmenter) EndInstant() *Instant {
	return s.end
}

func (s *Segmenter) Close() error {
	if s.closed {
		return nil
	}
	s.closing = true
	for _, track := range s.tracks {
		err := track.Close()
		if err != nil {
			return err
		}
	}
	if s.start != nil {
		ntp := time.Now().UTC()
		dts := ntpToDts(s.start.DTS, s.start.NPT, ntp)
		s.end = &Instant{
			DTS: dts,
			NPT: ntp,
		}
		s.UpdateIndex(false)
	}
	s.closing = false
	s.closed = true
	return nil
}
