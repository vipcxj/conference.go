package signal

import "sync"

var GLOBAL Global

type Global struct {
	sig_map map[string]*SignalContext
	sig_mux sync.Mutex
}

func (g *Global) RegisterSignalContext(ctx *SignalContext) {
	g.sig_mux.Lock()
	defer g.sig_mux.Unlock()
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