package hls

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path"

	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/yapingcat/gomedia/go-mp4"
)

type hlsSegment struct {
	duration float32
	uri      string
}

type H265Frame []byte

func newH265Frame(header codecs.H265NALUHeader, payload []byte) H265Frame {
	f := make([]byte, 2, len(payload)+2)
	f[0] = byte(header >> 8)
	f[1] = byte(header)
	f = append(f, payload...)
	return f
}

func (f H265Frame) Type() uint8 {
	const mask = 0b01111110
	return (f[0] * mask) >> 1
}

func (f H265Frame) SetType(t uint8) {
	const mask = 0b01111110
	f[0] |= (t & mask) >> 1
}

func (f H265Frame) AppendPlayload(p []byte) H265Frame {
	return append([]byte(f), p...)
}

func (f H265Frame) IsKeyFrame() bool {
	t := f.Type()
	return t == 32 || t == 33 || t == 34 || t == 16 || t == 17 || t == 18 || t == 19 || t == 20 || t == 21
}

type HLSMuxer struct {
	initUri       string
	segments      []hlsSegment
	muxer         *mp4.Movmuxer
	vedio_type    mp4.MP4_CODEC_TYPE
	keyFramed     bool
	i             int
	dir           string
	filename      string
	file          *os.File
	vtid          uint32
	cached_frames []H265Frame
}

func NewMuxer(vedio_type mp4.MP4_CODEC_TYPE, dir string) *HLSMuxer {
	return &HLSMuxer{
		vedio_type: vedio_type,
		dir:        dir,
	}
}

func (h *HLSMuxer) Init() {
	h.filename = path.Join(h.dir, fmt.Sprintf("hevcstream-%d.mp4", h.i))
	mp4file, err := os.OpenFile(h.filename, os.O_CREATE|os.O_RDWR, 0666)
	h.file = mp4file
	if err != nil {
		fmt.Println(err)
		return
	}
	h.muxer, err = mp4.CreateMp4Muxer(mp4file, mp4.WithMp4Flag(mp4.MP4_FLAG_FRAGMENT))
	if err != nil {
		fmt.Println(err)
		return
	}
	h.muxer.OnNewFragment(func(duration uint32, firstPts, firstDts uint64) {
		if duration < 8000 {
			return
		}
		fmt.Println("on segment", duration)
		h.segments = append(h.segments, hlsSegment{
			uri:      h.filename,
			duration: float32(duration) / 1000,
		})

		mp4file.Close()
		if h.i == 0 {
			initFile, _ := os.OpenFile(path.Join(h.dir, "hevcinit.mp4"), os.O_CREATE|os.O_RDWR, 0666)
			h.muxer.WriteInitSegment(initFile)
			initFile.Close()
			h.initUri = "hevcinit.mp4"
		}

		h.i++
		h.filename = path.Join(h.dir, fmt.Sprintf("hevcstream-%d.mp4", h.i))
		mp4file, err = os.OpenFile(h.filename, os.O_CREATE|os.O_RDWR, 0666)
		h.file = mp4file
		if err != nil {
			fmt.Println(err)
			return
		}
		h.muxer.ReBindWriter(mp4file)
	})
	h.vtid = h.muxer.AddVideoTrack(h.vedio_type)
}

func (h *HLSMuxer) WriteH264(packet *rtp.Packet) {
}

func (h *HLSMuxer) WriteH265(packet *rtp.Packet) {
	h265Packet := codecs.H265Packet{}
	_, err := h265Packet.Unmarshal(packet.Payload)
	if err != nil {
		panic(err)
	}
	rp := h265Packet.Packet()
	switch p := rp.(type) {
	case *codecs.H265PACIPacket:
		panic("unsupported")
	case *codecs.H265SingleNALUnitPacket:
		if h.cached_frames != nil {
			panic("invalid state")
		}
		h.cached_frames = []H265Frame{newH265Frame(p.PayloadHeader(), p.Payload())}
	case *codecs.H265FragmentationUnitPacket:
		if h.cached_frames == nil {
			if !p.FuHeader().S() {
				panic("expect start")
			}
			h.cached_frames = []H265Frame{newH265Frame(p.PayloadHeader(), p.Payload())}
			h.cached_frames[0].SetType(p.FuHeader().FuType())
			return
		} else {
			h.cached_frames[0].AppendPlayload(p.Payload())
			if !p.FuHeader().E() {
				return
			}
		}
	case *codecs.H265AggregationPacket:
		if h.cached_frames != nil {
			panic("invalid state")
		}
		h.cached_frames = make([]H265Frame, 1, 1+len(p.OtherUnits()))
		h.cached_frames[0] = H265Frame(p.FirstUnit().NalUnit())
		for _, unit := range p.OtherUnits() {
			h.cached_frames = append(h.cached_frames, unit.NalUnit())
		}
	default:
		panic("unsupport h256 packet")
	}
	for _, data := range h.cached_frames {
		if data.IsKeyFrame() {
			h.keyFramed = true
		}
		if !h.keyFramed {
			continue
		}
		h.muxer.Write(h.vtid, data, uint64(packet.Timestamp), uint64(packet.Timestamp))
	}
	h.cached_frames = nil
}

func (muxer *HLSMuxer) makeM3u8() string {
	buf := make([]byte, 0, 4096)
	m3u := bytes.NewBuffer(buf)
	maxDuration := 0
	for _, seg := range muxer.segments {
		if maxDuration < int(math.Ceil(float64(seg.duration))) {
			maxDuration = int(math.Ceil(float64(seg.duration)))
		}
	}

	m3u.WriteString("#EXTM3U\n")
	m3u.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", maxDuration))
	m3u.WriteString("#EXT-X-VERSION:7\n")
	m3u.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	m3u.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")
	m3u.WriteString(fmt.Sprintf("#EXT-X-MAP:URI=\"%s\"\n", muxer.initUri))

	for _, seg := range muxer.segments {
		m3u.WriteString(fmt.Sprintf("#EXTINF:%.3f,%s\n", seg.duration, "no desc"))
		m3u.WriteString(seg.uri + "\n")
	}
	m3u.WriteString("#EXT-X-ENDLIST\n")
	return m3u.String()
}

func (h *HLSMuxer) Close() {
	h.muxer.FlushFragment()
	m3u8Name := path.Join(h.dir, "test.m3u8")
	m3u8, _ := os.OpenFile(m3u8Name, os.O_CREATE|os.O_RDWR, 0666)
	defer m3u8.Close()
	m3u8.WriteString(h.makeM3u8())
	if h.file != nil {
		h.file.Close()
	}
}
