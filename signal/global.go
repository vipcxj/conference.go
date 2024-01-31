package signal

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/errors"
)

type Node struct {
	Epoch uint32
}

type Room struct {
	Servers []string
}

type Global struct {
	sig_map  map[string]*SignalContext
	sig_mux  sync.Mutex
	conf     *config.ConferenceConfigure
	promReg  *prometheus.Registry
	router   *Router
	messager *Messager
	metrics  *Metrics
}

func NewGlobal(conf *config.ConferenceConfigure) (*Global, error) {
	reg := prometheus.NewRegistry()
	promConf := conf.GetProm()
	if promConf.Enable {
		if promConf.GoCollectors {
			reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
			reg.MustRegister(collectors.NewGoCollector())
		}
	}
	global := &Global{
		promReg: reg,
		conf:    conf,
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
	return global, nil
}

func (g *Global) RegisterSignalContext(ctx *SignalContext) {
	g.sig_mux.Lock()
	defer g.sig_mux.Unlock()
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
		ctx.Close(disableCloseCallback)
	}
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
