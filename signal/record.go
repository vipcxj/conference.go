package signal

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/pkg/segmenter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type BaseInfo struct {
	Codec        string        `json:"codec" bson:"codec"`
	InitPath     string        `json:"initPath" bson:"initPath"`
	IndexPath    string        `json:"indexPath" bson:"indexPath"`
	SegmentPath  string        `json:"segmentPath" bson:"segmentPath"`
	Start        time.Time     `json:"start" bson:"start"`
	Completed    bool          `json:"completed" bson:"completed"`
	Duration     time.Duration `json:"duration" bson:"duration"`
	AvgBandwidth int           `json:"avgBandwidth" bson:"avgBandwidth"`
	MaxBandwidth int           `json:"maxBandwidth" bson:"maxBandwidth"`
}

type VideoInfo struct {
	BaseInfo  `bson:",inline"`
	Width     int     `json:"width" bson:"width"`
	Height    int     `json:"height" bson:"height"`
	FrameRate float64 `json:"frameRate" bson:"frameRate"`
}

type AudioInfo struct {
	BaseInfo `bson:",inline"`
	Default  bool `json:"default" bson:"default"`
}

type Record struct {
	ID             primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Start          time.Time          `json:"start" bson:"start"`
	End            time.Time          `json:"end,omitempty" bson:"end,omitempty"`
	Key            string             `json:"key" bson:"key"`
	Path           string             `json:"path" bson:"path"`
	Completed      bool               `json:"completed" bson:"completed"`
	Videos         []*VideoInfo       `json:"videos" bson:"videos"`
	VideosDuration time.Duration      `json:"videosDuration" bson:"videosDuration"`
	Audios         []*AudioInfo       `json:"audios" bson:"audios"`
	AudiosDuration time.Duration      `json:"audiosDuration" bson:"audiosDuration"`
}

var (
	ErrRecordNotEnabled              = errors.New("record is not enabled")
	ErrRecordDBIndexNotEnabled       = errors.New("record.dbIndex is not enabled")
	ErrRecordDBIndexMongoUrlRequired = errors.New("record.dbIndex.mongoUrl is required")
	ErrRecordDBIndexDatabaseRequired = errors.New("record.dbIndex.database is required")
	ErrRecordThisIsImpossible        = errors.New("this is impossible")
)

func createMongoAuth(opt *options.ClientOptions) {
	if opt.Auth == nil {
		opt.Auth = &options.Credential{}
	}
}

type Recorder struct {
	conf  *config.ConferenceConfigure
	id    primitive.ObjectID
	mongo *Mongo
	key   string
	err   error
	mux   sync.Mutex
}

func NewRecorder(conf *config.ConferenceConfigure, key string, mongo *Mongo) *Recorder {
	return &Recorder{
		conf: conf,
		key:  key,
		mongo: mongo,
	}
}

func fillMediaInfo(info *BaseInfo, trackCtx *segmenter.TrackContext) {
	info.Codec = trackCtx.Codec
	info.InitPath = trackCtx.InitPath
	info.IndexPath = trackCtx.IndexPath
	info.SegmentPath = trackCtx.SegmentPath
	info.Start = trackCtx.Start
	info.Duration = trackCtx.End.Sub(trackCtx.Start)
	info.Completed = trackCtx.Last
	info.AvgBandwidth = trackCtx.AvgBandwidth
	info.MaxBandwidth = trackCtx.MaxBandwidth
}

func MediaInfoFromTrackCtx(trackCtx *segmenter.TrackContext) interface{} {
	if trackCtx.Audio {
		info := &AudioInfo{}
		fillMediaInfo(&info.BaseInfo, trackCtx)
		return info
	} else {
		info := &VideoInfo{
			Width:     trackCtx.Width,
			Height:    trackCtx.Height,
			FrameRate: trackCtx.FrameRate,
		}
		fillMediaInfo(&info.BaseInfo, trackCtx)
		return info
	}
}

func RecordFromSegCtx(segCtx *segmenter.SegmentContext, key string) *Record {
	record := &Record{
		ID:        primitive.NilObjectID,
		Start:     segCtx.Start.NTP,
		End:       segCtx.End.NTP,
		Key:       key,
		Path:      segCtx.Path,
		Completed: segCtx.Last,
	}
	if segCtx.Last {
		record.End = segCtx.End.NTP
	}
	for _, track := range segCtx.Tracks {
		mediaInfo := MediaInfoFromTrackCtx(track)
		switch info := mediaInfo.(type) {
		case *VideoInfo:
			record.Videos = append(record.Videos, info)
			record.VideosDuration += info.Duration
		case *AudioInfo:
			record.Audios = append(record.Audios, info)
			record.AudiosDuration += info.Duration
		}
	}
	return record
}

func (r *Recorder) Record(segCtx *segmenter.SegmentContext) (bool, error) {
	r.mux.Lock()
	defer r.mux.Unlock()
	if r.err != nil {
		return false, r.err
	}
	if segCtx.First {
		record := RecordFromSegCtx(segCtx, r.key)
		res, err := r.mongo.RecordCollection().InsertOne(context.Background(), record)
		if err != nil {
			r.err = err
			return false, err
		}
		r.id = res.InsertedID.(primitive.ObjectID)
		return true, nil
	} else {
		if r.id == primitive.NilObjectID {
			panic(ErrRecordThisIsImpossible)
		}
		record := RecordFromSegCtx(segCtx, r.key)
		res, err := r.mongo.RecordCollection().ReplaceOne(context.Background(), bson.M{"_id": r.id}, record)
		if err != nil {
			r.err = err
			return false, err
		}
		return res.MatchedCount != 0, nil
	}
}
