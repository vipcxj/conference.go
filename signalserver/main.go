package signalserver

import (
	"context"
	"fmt"
	ossignal "os/signal"
	"syscall"

	"github.com/gin-contrib/graceful"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	healthcheck "github.com/tavsec/gin-healthcheck"
	healthchecks "github.com/tavsec/gin-healthcheck/checks"
	healthconfig "github.com/tavsec/gin-healthcheck/config"
	"go.uber.org/zap"

	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/log"
	"github.com/vipcxj/conference.go/middleware"
	"github.com/vipcxj/conference.go/signal"
	"github.com/zishang520/socket.io/v2/socket"
)

func Run(conf *config.ConferenceConfigure, ch chan error) {
	if !conf.Signal.Enable {
		ch <- errors.Ok()
		return
	}
	var err error

	if conf.Signal.Gin.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	var g *graceful.Graceful
	if conf.Signal.Tls.Enable {
		certPath := conf.Signal.Tls.Cert
		keyPath := conf.Signal.Tls.Key
		if certPath == "" || keyPath == "" {
			ch <- errors.FatalError("to enable ssl for auth server, the authServerCertPath and authServerKeyPath must be provided")
			return
		}
		g, err = graceful.New(gin.New(), graceful.WithTLS(conf.SignalListenAddress(), certPath, keyPath))
	} else {
		g, err = graceful.New(gin.New(), graceful.WithAddr(conf.SignalListenAddress()))
	}
	if err != nil {
		ch <- err
		return
	}
	defer g.Close()

	if !conf.Signal.Gin.NoRequestLog {
		g.Use(gin.Logger())
	}

	global, err := signal.NewGlobal(conf)
	if err != nil {
		ch <- err
		return
	}

	if conf.PromEnable() {
		g.GET("/metrics", gin.WrapH(promhttp.HandlerFor(global.GetPromReg(), promhttp.HandlerOpts{
			Registry: global.GetPromReg(),
		})))
	}

	pprof.Register(g.Engine)
	g.Use(middleware.ErrorHandler())
	if cors := conf.Signal.Cors; cors != "" {
		g.Use(middleware.Cors(cors))
	}

	io := socket.NewServer(nil, nil)
	io.Use(middleware.SocketIOAuthHandler(global))
	io.On("connection", func(clients ...any) {
		fmt.Printf("on connection\n")
		socket := clients[0].(*socket.Socket)
		ctx, err := signal.InitSignal(socket)
		var suger *zap.SugaredLogger
		if ctx != nil {
			suger = ctx.Sugar()
		} else {
			suger = log.Sugar()
		}
		if err != nil {
			suger.Errorf("socket connect failed, %v", err)
			signal.FatalErrorAndClose(socket, signal.ErrToString(err), "init signal")
		} else {
			suger.Info("socket connected")
		}
	})
	handler := io.ServeHandler(nil)
	g.GET("/socket.io/", gin.WrapH(handler))
	g.POST("/socket.io/", gin.WrapH(handler))
	g.GET(fmt.Sprintf("%v/:id", signal.CLOSE_CALLBACK_PREFIX), func(gctx *gin.Context) {
		id := gctx.Param("id")
		global.CloseSignalContext(id, true)
	})

	ctx, stop := ossignal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go global.GetMessager().Run(ctx)

	if conf.Signal.Healthy.Enable {
		healthConf := healthconfig.DefaultConfig()
		healthConf.FailureNotification.Chan = make(chan error, 1)
		defer close(healthConf.FailureNotification.Chan)
		healthConf.FailureNotification.Threshold = uint32(conf.Signal.Healthy.FailureThreshold)
		healthConf.HealthPath = conf.Signal.Healthy.Path

		signalsCheck := healthchecks.NewContextCheck(ctx, "signals")
		healthcheck.New(g.Engine, healthConf, []healthchecks.Check{signalsCheck})
	}

	err = g.RunWithContext(ctx)
	if err != nil && err != context.Canceled {
		ch <- err
	}
}
