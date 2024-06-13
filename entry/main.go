package entry

import (
	"context"

	"github.com/vipcxj/conference.go/authserver"
	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/log"
	"github.com/vipcxj/conference.go/signalserver"
	"github.com/vipcxj/conference.go/utils"
)

func Run(ctx context.Context) {
	ctx, spawn, start := utils.WithAutoCancel(ctx)
	auth_ctx, auth_done := spawn()
	conf := config.Conf()
	go authserver.Run(conf, auth_ctx, auth_done)
	cfgs := conf.Split()
	for _, cfg := range cfgs {
		signal_ctx, signal_done := spawn()
		go signalserver.Run(cfg, signal_ctx, signal_done)
	}
	start()
	<- ctx.Done()
	if context.Cause(ctx) != context.Canceled {
		log.Sugar().Fatalf("the server stopped, %v", context.Cause(ctx))
	}
}