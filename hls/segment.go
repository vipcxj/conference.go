package hls

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media/samplebuilder"
	"github.com/yapingcat/gomedia/go-mp4"
)

type Segment struct {
	duration float32
	uri      string
}

type SegmentContext struct {
	i    int
	time time.Time
}

type WriterFactory func(segCtx *SegmentContext, init bool) (string, io.WriteSeeker, error)

type Segmenter struct {
	started        bool
	baseTime       time.Time
	segments       []Segment
	context        SegmentContext
	writerFactory  WriterFactory
	initUri        string
	uri            string
	initWriter     io.WriteSeeker
	writer         io.WriteSeeker
	maxDuration    uint32
	keyFrameDelay  uint32
	dash           bool
	track          *webrtc.TrackRemote
	trackId        uint32
	muxer          *mp4.Movmuxer
	codecType      mp4.MP4_CODEC_TYPE
	samplerBuilder *samplebuilder.SampleBuilder
	keyFramed      bool
}

func getMediaType(track *webrtc.TrackRemote) (rtp.Depacketizer, mp4.MP4_CODEC_TYPE) {
	mimeType := strings.ToLower(track.Codec().MimeType)
	if mimeType == strings.ToLower(webrtc.MimeTypeH264) {
		return &codecs.H264Packet{}, mp4.MP4_CODEC_H264
	} else if mimeType == strings.ToLower(webrtc.MimeTypeH265) {
		return &codecs.H265Packet{}, mp4.MP4_CODEC_H265
	} else if mimeType == strings.ToLower(webrtc.MimeTypeOpus) {
		return &codecs.OpusPacket{}, mp4.MP4_CODEC_OPUS
	} else {
		panic(fmt.Sprintf("Unsupported media type %s", track.Codec().MimeType))
	}
}

func NewSegmenter(writerFactory WriterFactory, track *webrtc.TrackRemote, dash bool) *Segmenter {
	depacketizer, codecType := getMediaType(track)
	return &Segmenter{
		dash:           dash,
		writerFactory:  writerFactory,
		codecType:      codecType,
		samplerBuilder: samplebuilder.New(12, depacketizer, track.Codec().ClockRate),
		track:          track,
	}
}

func (s *Segmenter) Start() error {
	if s.started {
		return nil
	}
	s.started = true
	_, err := s.replaceWriter()
	s.muxer.OnNewFragment(s.maybeSegment)
	return err
}

func tryClose(writer io.WriteSeeker) (bool, error) {
	if writer != nil {
		if closer, ok := any(writer).(io.Closer); ok {
			err := closer.Close()
			if err != nil {
				return true, nil
			} else {
				return false, err
			}
		}
	}
	return false, nil
}

func (s *Segmenter) replaceWriter() (bool, error) {
	s.context.time = time.Now()
	uri, writer, err := s.writerFactory(&s.context, false)
	if err != nil {
		return false, err
	}
	if writer == s.writer {
		return false, nil
	}
	if s.context.i == 0 {
		s.initUri, s.initWriter, err = s.writerFactory(&s.context, true)
		if err != nil {
			return false, err
		}
	}
	_, err = tryClose(s.writer)
	if err != nil {
		return false, err
	}
	if s.context.i == 1 {
		s.muxer.WriteInitSegment(s.initWriter)
		tryClose(s.initWriter)
	}
	if s.muxer == nil {
		var muxerOption mp4.MuxerOption
		if s.dash {
			muxerOption = mp4.WithMp4Flag(mp4.MP4_FLAG_DASH)
		} else {
			muxerOption = mp4.WithMp4Flag(mp4.MP4_FLAG_FRAGMENT)
		}
		s.muxer, err = mp4.CreateMp4Muxer(writer, muxerOption)
		if err != nil {
			return false, err
		}
		switch s.track.Kind() {
		case webrtc.RTPCodecTypeVideo:
			s.muxer.AddVideoTrack(s.codecType)
		}
	} else {
		s.muxer.ReBindWriter(writer)
	}
	s.context.i++
	s.uri = uri
	s.writer = writer
	return true, nil
}

func (s *Segmenter) maybeSegment(duration uint32, firstPts, firstDts uint64) {
	if duration < s.maxDuration-s.keyFrameDelay {
		return
	}
	uri := s.uri
	replaced, err := s.replaceWriter()
	if err != nil {
		panic(err)
	}
	if !replaced {
		return
	}
	s.segments = append(s.segments, Segment{
		uri:      uri,
		duration: float32(duration) / 1000,
	})
}

func (s *Segmenter) isKeyFrame(data []byte) bool {
	switch s.codecType {
	case mp4.MP4_CODEC_H264:
		naluType := data[4] & 0x1F
		return naluType == 5
	case mp4.MP4_CODEC_H265:
		t := (data[4] >> 1) & 0x3f
		/**
		  IDR = 19 or 20
		  SPS = 33
		  PPS = 34
		*/
		// return t == 32 || t == 33 || t == 34 || t == 16 || t == 17 || t == 18 || t == 19 || t == 20 || t == 21
		return t == 19 || t == 20
	case mp4.MP4_CODEC_OPUS:
		return true
	default:
		return false
	}
}

func (s *Segmenter) WriteRTP(rtpPacket *rtp.Packet) {
	s.samplerBuilder.Push(rtpPacket)
	sample := s.samplerBuilder.Pop()
	if sample != nil {
		if !s.keyFramed {
			if s.isKeyFrame(sample.Data) {
				s.keyFramed = true
			}
		}
		if !s.keyFramed {
			return
		}
		s.muxer.Write(s.trackId, sample.Data, uint64(sample.PacketTimestamp), uint64(sample.PacketTimestamp))
	}
}

func (s *Segmenter) Close() {
	if s.muxer != nil {
		s.muxer.FlushFragment()
	}
	if s.context.i == 1 {
		s.muxer.WriteInitSegment(s.initWriter)
		tryClose(s.initWriter)
	}
}
