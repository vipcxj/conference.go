package segmenter

import (
	"fmt"
	"time"

	"github.com/sosodev/duration"
	"github.com/zencoder/go-dash/v3/helpers/ptrs"
	"github.com/zencoder/go-dash/v3/mpd"
	"github.com/zencoder/go-dash/v3/mpd/helpers/ptrs"
)

type Time time.Time
type Duration time.Duration


type MPD struct {
	XMLNs                      string   `xml:"xmlns,attr"`
	Profiles                   string   `xml:"profiles,attr"`
	Type                       string   `xml:"type,attr"`
	MediaPresentationDuration  *Duration   `xml:"mediaPresentationDuration,attr"`
	MinBufferTime              *Duration   `xml:"minBufferTime,attr"`
	AvailabilityStartTime      *Time   `xml:"availabilityStartTime,attr,omitempty"`
	MinimumUpdatePeriod        *Duration   `xml:"minimumUpdatePeriod,attr"`
	PublishTime                *Time   `xml:"publishTime,attr"`
	TimeShiftBufferDepth       *Duration   `xml:"timeShiftBufferDepth,attr"`
	SuggestedPresentationDelay *Duration `xml:"suggestedPresentationDelay,attr,omitempty"`
	Periods                    []Period       `xml:"Period,omitempty"`
	UTCTiming                  *DescriptorType `xml:"UTCTiming,omitempty"`
}

type Ratio struct {
	Left uint
	Right uint
}

type Fraction struct {
	Numerator uint
	Denominator uint
}

type VideoScanType string

const (
	VideoScanProgressive VideoScanType = "progressive"
	VideoScanInterlaced VideoScanType = "interlaced"
	VideoScanUnknown VideoScanType = "unknown"
)

type RepresentationBase struct {
	Profiles                  *string               `xml:"profiles,attr,omitempty"`
	Width                     *uint               `xml:"width,attr,omitempty"`
	Height                    *uint               `xml:"height,attr,omitempty"`
	Sar                       *Ratio               `xml:"sar,attr,omitempty"`
	FrameRate                 *Fraction               `xml:"frameRate,attr,omitempty"`
	AudioSamplingRate         *string               `xml:"audioSamplingRate,attr,omitempty"`
	MimeType                  *string               `xml:"mimeType,attr,omitempty"`
	SegmentProfiles           *string               `xml:"segmentProfiles,attr,omitempty"`
	Codecs                    *string               `xml:"codecs,attr,omitempty"`
	MaximumSAPPeriod          *float64               `xml:"maximumSAPPeriod,attr,omitempty"`
	StartWithSAP              *uint                `xml:"startWithSAP,attr,omitempty"` // [0, 6]
	MaxPlayoutRate            *float64               `xml:"maxPlayoutRate,attr,omitempty"`
	ScanType                  *VideoScanType               `xml:"scanType,attr,omitempty"`
}

type Period struct {
	ID              string           `xml:"id,attr,omitempty"`
	Duration        *Duration         `xml:"duration,attr,omitempty"`
	Start           *Duration        `xml:"start,attr,omitempty"`
	BaseURL         []string         `xml:"BaseURL,omitempty"`
	AdaptationSets  []AdaptationSet `xml:"AdaptationSet,omitempty"`
}

type AdaptationSet struct {
	RepresentationBase
	ID                 uint           `xml:"id,attr"`
	SegmentAlignment   *bool             `xml:"segmentAlignment,attr"`
	Lang               *string           `xml:"lang,attr"`
	Group              *uint           `xml:"group,attr"`
	PAR                *string           `xml:"par,attr"`
	MinBandwidth       *string           `xml:"minBandwidth,attr"`
	MaxBandwidth       *string           `xml:"maxBandwidth,attr"`
	MinWidth           *string           `xml:"minWidth,attr"`
	MaxWidth           *string           `xml:"maxWidth,attr"`
	MinHeight          *string           `xml:"minHeight,attr"`
	MaxHeight          *string           `xml:"maxHeight,attr"`
	ContentType        *string           `xml:"contentType,attr"`
	Roles              []*Role           `xml:"Role,omitempty"`
	SegmentBase        *SegmentBase      `xml:"SegmentBase,omitempty"`
	SegmentList        *SegmentList      `xml:"SegmentList,omitempty"`
	SegmentTemplate    *SegmentTemplate  `xml:"SegmentTemplate,omitempty"` // Live Profile Only
	Representations    []*Representation `xml:"Representation,omitempty"`
	AccessibilityElems []*Accessibility  `xml:"Accessibility,omitempty"`
	Label              *string           `xml:"label,attr"`
}

func NewMPD() *MPD {
	return &MPD{
		XMLNs: "urn:mpeg:dash:schema:mpd:2011",
		Profiles: "urn:mpeg:dash:profile:isoff-live:2011",
		Type: "dynamic",
	}
}

func (m *MPD) Start() {
	m.AvailabilityStartTime = time.Now()
}


type MyMPD struct {
	baseURL string
	file string
	mpd *mpd.MPD
	rep *mpd.Representation
	minBufferTime time.Duration
	initSegment *Segment
	segments []Segment
}

func NewMPD1(
	baseURL string,
	file string,
	startTime time.Time,
	minBufferTime time.Duration,
	minimumUpdatePeriod time.Duration,
	timeScale uint32,
	dur uint32,
) *MyMPD {
	m := mpd.NewDynamicMPD(
		mpd.DASH_PROFILE_LIVE,
		startTime.Format(time.RFC3339),
		duration.Format(minBufferTime),
		mpd.AttrMinimumUpdatePeriod(duration.Format(minimumUpdatePeriod)),
	)
	m.BaseURL = []string{baseURL}
	p := m.GetCurrentPeriod()
	p.ID = "p01"
	a, _ := p.AddNewAdaptationSetVideoWithID("ac01", mpd.DASH_MIME_TYPE_VIDEO_MP4, "progressive", true, 1)
	r, _ := a.AddNewRepresentationVideo(0, "", "r01", "6", 0, 0)
	r.SegmentList = &mpd.SegmentList{
		MultipleSegmentBase: mpd.MultipleSegmentBase{
			SegmentBase: mpd.SegmentBase{
				Timescale: ptrs.Uint32ptr(timeScale),
			},
			Duration: ptrs.Uint32ptr(dur),
		},
	}
	return &MyMPD{
		baseURL: baseURL,
		file: file,
		mpd: m,
		rep: r,
		minBufferTime: minBufferTime,
	}
}

func (m *MyMPD) UpdateBandwidth() {
	bandwidth := int64(0)
	buffer_time := int64(m.minBufferTime)
	lenSeg := len(m.segments)
	for i := 0; i < lenSeg; i++ {
		accu_size := int64(0)
		accu_duration := int64(0)
		buffer_size := (buffer_time * bandwidth) / 8
		for j := i; j < lenSeg; j++ {
			seg := m.segments[j]
			accu_size += int64(seg.Size)
			accu_duration += int64(seg.Duration)
			max_avail := buffer_size + accu_duration * bandwidth / 8
			if accu_size > max_avail && accu_duration != 0 {
				bandwidth = 8 * (accu_size - buffer_size) / accu_duration
				break
			}
		}
	}
	m.rep.Bandwidth = ptrs.Int64ptr(bandwidth)
}

func (m *MyMPD) SetInit(seg Segment) {
	m.initSegment = &seg
	m.rep.SegmentList.Initialization = &mpd.
}

func (m *MMyMPD AddSegment(seg Segment) {
	m.segments = append(m.segments, seg)
	m.UpdateBandwidth()
	m.rep.SegmentList.SegmentURLs = append(m.rep.SegmentList.SegmentURLs, &mpd.SegmentURL{
		Media: &m.file,
		MediaRange: ptrs.Strptr(fmt.Sprintf("%d-%d", seg.Start, seg.Size + seg.Size)),
	})
}