package signal

import (
	"sync"
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
}

var GLOBAL Global

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
