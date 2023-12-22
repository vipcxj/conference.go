package signal

import (
	"context"
	"errors"
	"time"

	"github.com/vipcxj/conference.go/config"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type BaseInfo struct {
	Codec        string        `json:"codec" bson:"codec"`
	Path         string        `json:"path" bson:"path"`
	FileSize     string        `json:"fileSize" bson:"fileSize"`
	Start        time.Time     `json:"start" bson:"start"`
	Completed    bool          `json:"completed" bson:"completed"`
	Duration     time.Duration `json:"duration" bson:"duration"`
	AvgBindwidth int           `json:"avgBindwidth" bson:"avgBindwidth"`
	MaxBindwidth int           `json:"maxBindwidth" bson:"maxBindwidth"`
}

type VideoInfo struct {
	BaseInfo  `bson:",inline"`
	Width     int     `json:"width" bson:"width"`
	Height    int     `json:"height" bson:"height"`
	FrameRate float32 `json:"frameRate" bson:"frameRate"`
}

type AudioInfo struct {
	BaseInfo `bson:",inline"`
	Default  bool `json:"default" bson:"default"`
}

type Record struct {
	ID        primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Start     time.Time          `json:"start" bson:"start"`
	End       time.Time          `json:"end,omitempty" bson:"end,omitempty"`
	Key       string             `json:"key" bson:"key"`
	Path      string             `json:"path" bson:"path"`
	Completed bool               `json:"completed" bson:"completed"`
	Duration  time.Duration      `json:"duration" bson:"duration"`
	Video     []VideoInfo        `json:"video" bson:"video"`
	Audio     []AudioInfo        `json:"audio" bson:"audio"`
}

var (
	ErrRecordNotEnabled              = errors.New("record is not enabled")
	ErrRecordDBIndexNotEnabled       = errors.New("record.dbIndex is not enabled")
	ErrRecordDBIndexMongoUrlRequired = errors.New("record.dbIndex.mongoUrl is required")
	ErrRecordDBIndexDatabaseRequired = errors.New("record.dbIndex.database is required")
)

func createMongoAuth(opt *options.ClientOptions) {
	if opt.Auth == nil {
		opt.Auth = &options.Credential{}
	}
}

func prepareDB(ctx context.Context) (*mongo.Collection, error) {
	if !config.Conf().Record.Enable {
		return nil, ErrRecordNotEnabled
	}
	conf := config.Conf().Record.DBIndex
	if !conf.Enable {
		return nil, ErrRecordDBIndexNotEnabled
	}
	url := conf.MongoUrl
	if url == "" {
		return nil, ErrRecordDBIndexMongoUrlRequired
	}

	if conf.Database == "" {
		return nil, ErrRecordDBIndexDatabaseRequired
	}
	col := conf.Collection
	if col == "" {
		col = "Record"
	}

	opt := options.Client().ApplyURI(url)
	if conf.Auth.User != "" {
		createMongoAuth(opt)
		opt.Auth.Username = conf.Auth.User
	}
	if conf.Auth.Pass != "" {
		createMongoAuth(opt)
		opt.Auth.Password = conf.Auth.Pass
	}
	client, err := mongo.Connect(ctx, opt)
	if err != nil {
		return nil, err
	}

	db := client.Database(conf.Database)
	collection := db.Collection(col)
	_, err = collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.M{
			"key": 1,
		},
	})
	if err != nil {
		return nil, err
	}
	_, err = collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.M{
			"start": -1,
		},
	})
	if err != nil {
		return nil, err
	}
	return collection, nil
}

func NewRecord(record *Record) (*Record, error) {
	ctx := context.Background()
	col, err := prepareDB(ctx)
	if err != nil {
		return nil, err
	}
	res, err := col.InsertOne(ctx, record)
	if err != nil {
		return nil, err
	}
	record.ID = res.InsertedID.(primitive.ObjectID)
	return record, nil
}

func UpdateRecord(record *Record) (bool, error) {
	ctx := context.Background()
	col, err := prepareDB(ctx)
	if err != nil {
		return false, err
	}
	res, err := col.UpdateByID(ctx, record.ID, record)
	if err != nil {
		return false, err
	}
	return res.MatchedCount != 0, nil
}
