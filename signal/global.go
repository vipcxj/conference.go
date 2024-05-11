package signal

import (
	"context"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/log"
	"github.com/vipcxj/conference.go/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

type Node struct {
	Epoch uint32
}

type Room struct {
	Servers []string
}

type Mongo struct {
	client    *mongo.Client
	recordCol *mongo.Collection
}

func NewMongo(global *Global) (*Mongo, error) {
	cfg := &global.Conf().Record.DBIndex
	if global.Conf().Record.Enable && cfg.Enable {
		url := cfg.MongoUrl
		col := cfg.Collection
		if col == "" {
			col = "record"
		}
		opt := options.Client().ApplyURI(url)
		if cfg.Auth.User != "" {
			createMongoAuth(opt)
			opt.Auth.Username = cfg.Auth.User
		}
		if cfg.Auth.Pass != "" {
			createMongoAuth(opt)
			opt.Auth.Password = cfg.Auth.Pass
		}
		client, err := mongo.Connect(context.Background(), opt)
		if err != nil {
			return nil, errors.FatalError("unable to create mongo, %v", err)
		}
		recordDb := client.Database(cfg.Database)
		recordCol := recordDb.Collection(col)
		_, err = recordCol.Indexes().CreateOne(context.Background(), mongo.IndexModel{
			Keys: bson.M{
				"key": 1,
			},
		})
		if err != nil {
			return nil, err
		}
		_, err = recordCol.Indexes().CreateOne(context.Background(), mongo.IndexModel{
			Keys: bson.M{
				"start": -1,
			},
		})
		if err != nil {
			return nil, err
		}
		return &Mongo{
			client:    client,
			recordCol: recordCol,
		}, nil
	} else {
		return nil, nil
	}
}

func (m *Mongo) RecordCollection() *mongo.Collection {
	return m.recordCol
}

type Global struct {
	sig_map         map[string]*SignalContext
	sig_map_by_user map[string][]*SignalContext
	sig_mux         sync.RWMutex
	conf            *config.ConferenceConfigure
	ctx             context.Context
	cancel          context.CancelCauseFunc
	promReg         *prometheus.Registry
	router          *Router
	messager        *Messager
	metrics         *Metrics
	mongo           *Mongo
	logger          *zap.Logger
	sugar           *zap.SugaredLogger
}

func NewGlobal(ctx context.Context, conf *config.ConferenceConfigure) (*Global, error) {
	reg := prometheus.NewRegistry()
	promConf := conf.GetProm()
	if promConf.Enable {
		if promConf.GoCollectors {
			reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
			reg.MustRegister(collectors.NewGoCollector())
		}
	}
	ctx, cancel := context.WithCancelCause(ctx)
	logger := log.Logger().With(zap.String("tag", "global"))
	global := &Global{
		promReg: reg,
		conf:    conf,
		ctx:     ctx,
		cancel:  cancel,
		logger:  logger,
		sugar:   log.Sugar(),
	}
	router, err := NewRouter(global)
	if err != nil {
		return nil, errors.FatalError("unable to create global object, %v", err)
	}
	global.router = router
	messager, err := NewMessager(global)
	if err != nil {
		return nil, errors.FatalError("unable to create global object, %v", err)
	}
	global.messager = messager
	var metrics *Metrics
	signalConf := &conf.Signal
	if signalConf.Prometheus.Enable {
		metrics = NewMetrics(reg, conf)
	}
	global.metrics = metrics
	mongo, err := NewMongo(global)
	if err != nil {
		return nil, errors.FatalError("unable to create global object, %v", err)
	}
	global.mongo = mongo
	return global, nil
}

func (g *Global) Logger() *zap.Logger {
	return g.logger
}

func (g *Global) Sugar() *zap.SugaredLogger {
	return g.sugar
}

func (g *Global) RegisterSignalContext(ctx *SignalContext) {
	g.sig_mux.Lock()
	defer g.sig_mux.Unlock()
	if g.sig_map_by_user == nil {
		g.sig_map_by_user = make(map[string][]*SignalContext)
	}
	ctxs, found := g.sig_map_by_user[ctx.AuthInfo.UID]
	if !found {
		g.sig_map_by_user[ctx.AuthInfo.UID] = []*SignalContext{ctx}
	} else {
		g.sig_map_by_user[ctx.AuthInfo.UID] = append(ctxs, ctx)
	}
	if g.sig_map == nil {
		g.sig_map = make(map[string]*SignalContext)
	}
	g.sig_map[ctx.Id] = ctx
}

func (g *Global) CloseSignalContext(id string, disableCloseCallback bool) {
	g.sig_mux.Lock()
	defer g.sig_mux.Unlock()
	ctx, ok := g.sig_map[id]
	if ok {
		delete(g.sig_map, id)
		ctxs, found := g.sig_map_by_user[ctx.AuthInfo.UID]
		if found {
			ctxs = utils.SliceRemoveIfIgnoreOrder(ctxs, func(v *SignalContext) bool { return v.Id == id })
			if len(ctxs) == 0 {
				delete(g.sig_map_by_user, ctx.AuthInfo.UID)
			} else {
				g.sig_map_by_user[ctx.AuthInfo.UID] = ctxs
			}
		}
		ctx.SelfClose(disableCloseCallback)
	}
}

func (g *Global) FindSignalContextById(id string) (res *SignalContext) {
	g.sig_mux.RLock()
	defer g.sig_mux.RUnlock()
	ctx, found := g.sig_map[id]
	if found {
		return ctx
	} else {
		return nil
	}
}

func (g *Global) FindSignalContextByUser(uid string) (res []*SignalContext) {
	g.sig_mux.RLock()
	defer g.sig_mux.RUnlock()
	ctxs, found := g.sig_map_by_user[uid]
	if found {
		res = append(res, ctxs...)
	}
	return
}

func (g *Global) Conf() *config.ConferenceConfigure {
	if g == nil {
		return nil
	}
	return g.conf
}

func (g *Global) Router() *Router {
	if g == nil {
		return nil
	}
	return g.router
}

func (g *Global) GetPromReg() *prometheus.Registry {
	if g == nil {
		return nil
	}
	return g.promReg
}

func (g *Global) GetMessager() *Messager {
	if g == nil {
		return nil
	}
	return g.messager
}

func (g *Global) GetMetrics() *Metrics {
	if g == nil {
		return nil
	}
	return g.metrics
}

func (g *Global) Mongo() *Mongo {
	if g == nil {
		return nil
	}
	return g.mongo
}
