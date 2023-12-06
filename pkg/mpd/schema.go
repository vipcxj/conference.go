package mpd

// MPD ...
type MPD *MPDtype

// MPDtype ...
type MPDtype struct {
	XMLNsAttr                      string                    `xml:"xmlns,attr"`
	IdAttr                         string                    `xml:"id,attr,omitempty"`
	ProfilesAttr                   string                    `xml:"profiles,attr"`
	TypeAttr                       string                    `xml:"type,attr,omitempty"`
	AvailabilityStartTimeAttr      string                    `xml:"availabilityStartTime,attr,omitempty"`
	AvailabilityEndTimeAttr        string                    `xml:"availabilityEndTime,attr,omitempty"`
	PublishTimeAttr                string                    `xml:"publishTime,attr,omitempty"`
	MediaPresentationDurationAttr  string                    `xml:"mediaPresentationDuration,attr,omitempty"`
	MinimumUpdatePeriodAttr        string                    `xml:"minimumUpdatePeriod,attr,omitempty"`
	MinBufferTimeAttr              string                    `xml:"minBufferTime,attr"`
	TimeShiftBufferDepthAttr       string                    `xml:"timeShiftBufferDepth,attr,omitempty"`
	SuggestedPresentationDelayAttr string                    `xml:"suggestedPresentationDelay,attr,omitempty"`
	MaxSegmentDurationAttr         string                    `xml:"maxSegmentDuration,attr,omitempty"`
	MaxSubsegmentDurationAttr      string                    `xml:"maxSubsegmentDuration,attr,omitempty"`
	ProgramInformation             []*ProgramInformationType `xml:"ProgramInformation"`
	BaseURL                        []*BaseURLType            `xml:"BaseURL"`
	Location                       []string                  `xml:"Location"`
	Period                         []*PeriodType             `xml:"Period"`
	Metrics                        []*MetricsType            `xml:"Metrics"`
	EssentialProperty              []*DescriptorType         `xml:"EssentialProperty"`
	SupplementalProperty           []*DescriptorType         `xml:"SupplementalProperty"`
	UTCTiming                      []*DescriptorType         `xml:"UTCTiming"`
}

// PresentationType ...
type PresentationType string

// PeriodType ...
type PeriodType struct {
	XlinkHrefAttr          string               `xml:"xlink:href,attr,omitempty"`
	XlinkActuateAttr       string               `xml:"xlink:actuate,attr,omitempty"`
	IdAttr                 string               `xml:"id,attr,omitempty"`
	StartAttr              string               `xml:"start,attr,omitempty"`
	DurationAttr           string               `xml:"duration,attr,omitempty"`
	BitstreamSwitchingAttr bool                 `xml:"bitstreamSwitching,attr,omitempty"`
	BaseURL                []*BaseURLType       `xml:"BaseURL"`
	SegmentBase            *SegmentBaseType     `xml:"SegmentBase"`
	SegmentList            *SegmentListType     `xml:"SegmentList"`
	SegmentTemplate        *SegmentTemplateType `xml:"SegmentTemplate"`
	AssetIdentifier        *DescriptorType      `xml:"AssetIdentifier"`
	EventStream            []*EventStreamType   `xml:"EventStream"`
	AdaptationSet          []*AdaptationSetType `xml:"AdaptationSet"`
	Subset                 []*SubsetType        `xml:"Subset"`
	SupplementalProperty   []*DescriptorType    `xml:"SupplementalProperty"`
}

// EventStreamType ...
type EventStreamType struct {
	XlinkHrefAttr    string       `xml:"xlink:href,attr,omitempty"`
	XlinkActuateAttr string       `xml:"xlink:actuate,attr,omitempty"`
	MessageDataAttr  string       `xml:"messageData,attr,omitempty"`
	SchemeIdUriAttr  string       `xml:"schemeIdUri,attr"`
	ValueAttr        string       `xml:"value,attr,omitempty"`
	TimescaleAttr    uint32       `xml:"timescale,attr,omitempty"`
	Event            []*EventType `xml:"Event"`
}

// EventType ...
type EventType struct {
	PresentationTimeAttr uint64 `xml:"presentationTime,attr,omitempty"`
	DurationAttr         uint64 `xml:"duration,attr,omitempty"`
	IdAttr               uint32 `xml:"id,attr,omitempty"`
	MessageDataAttr      string `xml:"messageData,attr,omitempty"`
}

// AdaptationSetType ...
type AdaptationSetType struct {
	XlinkHrefAttr               string                  `xml:"xlink:href,attr,omitempty"`
	XlinkActuateAttr            string                  `xml:"xlink:actuate,attr,omitempty"`
	IdAttr                      uint32                  `xml:"id,attr,omitempty"`
	GroupAttr                   uint32                  `xml:"group,attr,omitempty"`
	LangAttr                    string                  `xml:"lang,attr,omitempty"`
	ContentTypeAttr             string                  `xml:"contentType,attr,omitempty"`
	ParAttr                     string                  `xml:"par,attr,omitempty"`
	MinBandwidthAttr            uint32                  `xml:"minBandwidth,attr,omitempty"`
	MaxBandwidthAttr            uint32                  `xml:"maxBandwidth,attr,omitempty"`
	MinWidthAttr                uint32                  `xml:"minWidth,attr,omitempty"`
	MaxWidthAttr                uint32                  `xml:"maxWidth,attr,omitempty"`
	MinHeightAttr               uint32                  `xml:"minHeight,attr,omitempty"`
	MaxHeightAttr               uint32                  `xml:"maxHeight,attr,omitempty"`
	MinFrameRateAttr            string                  `xml:"minFrameRate,attr,omitempty"`
	MaxFrameRateAttr            string                  `xml:"maxFrameRate,attr,omitempty"`
	SegmentAlignmentAttr        *ConditionalUintType    `xml:"segmentAlignment,attr,omitempty"`
	SubsegmentAlignmentAttr     *ConditionalUintType    `xml:"subsegmentAlignment,attr,omitempty"`
	SubsegmentStartsWithSAPAttr uint32                  `xml:"subsegmentStartsWithSAP,attr,omitempty"`
	BitstreamSwitchingAttr      bool                    `xml:"bitstreamSwitching,attr,omitempty"`
	Accessibility               []*DescriptorType       `xml:"Accessibility"`
	Role                        []*DescriptorType       `xml:"Role"`
	Rating                      []*DescriptorType       `xml:"Rating"`
	Viewpoint                   []*DescriptorType       `xml:"Viewpoint"`
	ContentComponent            []*ContentComponentType `xml:"ContentComponent"`
	BaseURL                     []*BaseURLType          `xml:"BaseURL"`
	SegmentBase                 *SegmentBaseType        `xml:"SegmentBase"`
	SegmentList                 *SegmentListType        `xml:"SegmentList"`
	SegmentTemplate             *SegmentTemplateType    `xml:"SegmentTemplate"`
	Representation              []*RepresentationType   `xml:"Representation"`
	*RepresentationBaseType
}

// RatioType ...
type RatioType string

// FrameRateType ...
type FrameRateType string

// ConditionalUintType ...
type ConditionalUintType struct {
	Boolean     bool
	UnsignedInt uint32
}

// ContentComponentType ...
type ContentComponentType struct {
	IdAttr          uint32            `xml:"id,attr,omitempty"`
	LangAttr        string            `xml:"lang,attr,omitempty"`
	ContentTypeAttr string            `xml:"contentType,attr,omitempty"`
	ParAttr         string            `xml:"par,attr,omitempty"`
	Accessibility   []*DescriptorType `xml:"Accessibility"`
	Role            []*DescriptorType `xml:"Role"`
	Rating          []*DescriptorType `xml:"Rating"`
	Viewpoint       []*DescriptorType `xml:"Viewpoint"`
}

// RepresentationType ...
type RepresentationType struct {
	IdAttr                     string                   `xml:"id,attr"`
	BandwidthAttr              uint32                   `xml:"bandwidth,attr"`
	QualityRankingAttr         uint32                   `xml:"qualityRanking,attr,omitempty"`
	DependencyIdAttr           *StringVectorType        `xml:"dependencyId,attr,omitempty"`
	MediaStreamStructureIdAttr *StringVectorType        `xml:"mediaStreamStructureId,attr,omitempty"`
	BaseURL                    []*BaseURLType           `xml:"BaseURL"`
	SubRepresentation          []*SubRepresentationType `xml:"SubRepresentation"`
	SegmentBase                *SegmentBaseType         `xml:"SegmentBase"`
	SegmentList                *SegmentListType         `xml:"SegmentList"`
	SegmentTemplate            *SegmentTemplateType     `xml:"SegmentTemplate"`
	*RepresentationBaseType
}

// StringNoWhitespaceType ...
type StringNoWhitespaceType string

// SubRepresentationType ...
type SubRepresentationType struct {
	LevelAttr            uint32            `xml:"level,attr,omitempty"`
	DependencyLevelAttr  *UIntVectorType   `xml:"dependencyLevel,attr,omitempty"`
	BandwidthAttr        uint32            `xml:"bandwidth,attr,omitempty"`
	ContentComponentAttr *StringVectorType `xml:"contentComponent,attr,omitempty"`
	*RepresentationBaseType
}

// RepresentationBaseType ...
type RepresentationBaseType struct {
	ProfilesAttr              string             `xml:"profiles,attr,omitempty"`
	WidthAttr                 uint32             `xml:"width,attr,omitempty"`
	HeightAttr                uint32             `xml:"height,attr,omitempty"`
	SarAttr                   string             `xml:"sar,attr,omitempty"`
	FrameRateAttr             string             `xml:"frameRate,attr,omitempty"`
	AudioSamplingRateAttr     string             `xml:"audioSamplingRate,attr,omitempty"`
	MimeTypeAttr              string             `xml:"mimeType,attr,omitempty"`
	SegmentProfilesAttr       string             `xml:"segmentProfiles,attr,omitempty"`
	CodecsAttr                string             `xml:"codecs,attr,omitempty"`
	MaximumSAPPeriodAttr      float64            `xml:"maximumSAPPeriod,attr,omitempty"`
	StartWithSAPAttr          uint32             `xml:"startWithSAP,attr,omitempty"`
	MaxPlayoutRateAttr        float64            `xml:"maxPlayoutRate,attr,omitempty"`
	CodingDependencyAttr      bool               `xml:"codingDependency,attr,omitempty"`
	ScanTypeAttr              string             `xml:"scanType,attr,omitempty"`
	FramePacking              []*DescriptorType  `xml:"FramePacking"`
	AudioChannelConfiguration []*DescriptorType  `xml:"AudioChannelConfiguration"`
	ContentProtection         []*DescriptorType  `xml:"ContentProtection"`
	EssentialProperty         []*DescriptorType  `xml:"EssentialProperty"`
	SupplementalProperty      []*DescriptorType  `xml:"SupplementalProperty"`
	InbandEventStream         []*EventStreamType `xml:"InbandEventStream"`
	Switching                 []*SwitchingType   `xml:"Switching"`
}

// SAPType ...
type SAPType uint32

// VideoScanType ...
type VideoScanType string

// SubsetType ...
type SubsetType struct {
	ContainsAttr *UIntVectorType `xml:"contains,attr"`
	IdAttr       string          `xml:"id,attr,omitempty"`
}

// SwitchingType ...
type SwitchingType struct {
	IntervalAttr uint32 `xml:"interval,attr"`
	TypeAttr     string `xml:"type,attr,omitempty"`
}

// SwitchingTypeType ...
type SwitchingTypeType string

// SegmentBaseType ...
type SegmentBaseType struct {
	TimescaleAttr                uint32   `xml:"timescale,attr,omitempty"`
	PresentationTimeOffsetAttr   uint64   `xml:"presentationTimeOffset,attr,omitempty"`
	IndexRangeAttr               string   `xml:"indexRange,attr,omitempty"`
	IndexRangeExactAttr          bool     `xml:"indexRangeExact,attr,omitempty"`
	AvailabilityTimeOffsetAttr   float64  `xml:"availabilityTimeOffset,attr,omitempty"`
	AvailabilityTimeCompleteAttr bool     `xml:"availabilityTimeComplete,attr,omitempty"`
	Initialization               *URLType `xml:"Initialization"`
	RepresentationIndex          *URLType `xml:"RepresentationIndex"`
}

// MultipleSegmentBaseType ...
type MultipleSegmentBaseType struct {
	DurationAttr       uint32               `xml:"duration,attr,omitempty"`
	StartNumberAttr    uint32               `xml:"startNumber,attr,omitempty"`
	SegmentTimeline    *SegmentTimelineType `xml:"SegmentTimeline"`
	BitstreamSwitching *URLType             `xml:"BitstreamSwitching"`
	*SegmentBaseType
}

// URLType ...
type URLType struct {
	SourceURLAttr string `xml:"sourceURL,attr,omitempty"`
	RangeAttr     string `xml:"range,attr,omitempty"`
}

// SegmentListType ...
type SegmentListType struct {
	XlinkHrefAttr    string            `xml:"xlink:href,attr,omitempty"`
	XlinkActuateAttr string            `xml:"xlink:actuate,attr,omitempty"`
	SegmentURL       []*SegmentURLType `xml:"SegmentURL"`
	*MultipleSegmentBaseType
}

// SegmentURLType ...
type SegmentURLType struct {
	MediaAttr      string `xml:"media,attr,omitempty"`
	MediaRangeAttr string `xml:"mediaRange,attr,omitempty"`
	IndexAttr      string `xml:"index,attr,omitempty"`
	IndexRangeAttr string `xml:"indexRange,attr,omitempty"`
}

// SegmentTemplateType ...
type SegmentTemplateType struct {
	MediaAttr              string `xml:"media,attr,omitempty"`
	IndexAttr              string `xml:"index,attr,omitempty"`
	InitializationAttr     string `xml:"initialization,attr,omitempty"`
	BitstreamSwitchingAttr string `xml:"bitstreamSwitching,attr,omitempty"`
	*MultipleSegmentBaseType
}

// S ...
type S struct {
	TAttr uint64 `xml:"t,attr,omitempty"`
	NAttr uint64 `xml:"n,attr,omitempty"`
	DAttr uint64 `xml:"d,attr"`
	RAttr int    `xml:"r,attr,omitempty"`
}

// SegmentTimelineType ...
type SegmentTimelineType struct {
	S []*S `xml:"S"`
}

// StringVectorType ...
type StringVectorType []string

// UIntVectorType ...
type UIntVectorType []uint32

// BaseURLType ...
type BaseURLType struct {
	ServiceLocationAttr          string  `xml:"serviceLocation,attr,omitempty"`
	ByteRangeAttr                string  `xml:"byteRange,attr,omitempty"`
	AvailabilityTimeOffsetAttr   float64 `xml:"availabilityTimeOffset,attr,omitempty"`
	AvailabilityTimeCompleteAttr bool    `xml:"availabilityTimeComplete,attr,omitempty"`
	Value                        string  `xml:",chardata"`
}

// ProgramInformationType ...
type ProgramInformationType struct {
	LangAttr               string `xml:"lang,attr,omitempty"`
	MoreInformationURLAttr string `xml:"moreInformationURL,attr,omitempty"`
	Title                  string `xml:"Title"`
	Source                 string `xml:"Source"`
	Copyright              string `xml:"Copyright"`
}

// DescriptorType ...
type DescriptorType struct {
	SchemeIdUriAttr string `xml:"schemeIdUri,attr"`
	ValueAttr       string `xml:"value,attr,omitempty"`
	IdAttr          string `xml:"id,attr,omitempty"`
}

// MetricsType ...
type MetricsType struct {
	MetricsAttr string            `xml:"metrics,attr"`
	Reporting   []*DescriptorType `xml:"Reporting"`
	Range       []*RangeType      `xml:"Range"`
}

// RangeType ...
type RangeType struct {
	StarttimeAttr string `xml:"starttime,attr,omitempty"`
	DurationAttr  string `xml:"duration,attr,omitempty"`
}
