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

func Run(ch chan error) {
	signal.InitRouter()
	if !config.Conf().Signal.Enable {
		ch <- errors.Ok()
		return
	}

	host := config.Conf().Signal.HostOrIp
	port := config.Conf().Signal.Port
	addr := fmt.Sprintf("%s:%d", host, port)
	var err error

	if config.Conf().Signal.Gin.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	var g *graceful.Graceful
	if config.Conf().Signal.Tls.Enable {
		certPath := config.Conf().Signal.Tls.Cert
		keyPath := config.Conf().Signal.Tls.Key
		if certPath == "" || keyPath == "" {
			ch <- errors.FatalError("to enable ssl for auth server, the authServerCertPath and authServerKeyPath must be provided")
			return
		}
		g, err = graceful.New(gin.New(), graceful.WithTLS(addr, certPath, keyPath))
	} else {
		g, err = graceful.New(gin.New(), graceful.WithAddr(addr))
	}
	if err != nil {
		ch <- err
		return
	}
	defer g.Close()

	if !config.Conf().Signal.Gin.NoRequestLog {
		g.Use(gin.Logger())
	}

	if config.Conf().PromEnable() {
		g.GET("/metrics", gin.WrapH(promhttp.HandlerFor(config.Prom().Registry(), promhttp.HandlerOpts{
			Registry: config.Prom().Registry(),
		})))
	}

	pprof.Register(g.Engine)
	g.Use(middleware.ErrorHandler())
	if cors := config.Conf().Signal.Cors; cors != "" {
		g.Use(middleware.Cors(cors))
	}

	messager, err := signal.NewMessager()
	if err != nil {
		ch <- err
		return
	}

	io := socket.NewServer(nil, nil)
	io.Use(middleware.SocketIOAuthHandler(messager))
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
			signal.FatalErrorAndClose(socket, signal.ErrToMsg(err), "init signal")
		} else {
			suger.Info("socket connected")
		}
	})
	handler := io.ServeHandler(nil)
	g.GET("/socket.io/", gin.WrapH(handler))
	g.POST("/socket.io/", gin.WrapH(handler))
	g.GET(fmt.Sprintf("%v/:id", signal.CLOSE_CALLBACK_PREFIX), func(gctx *gin.Context) {
		id := gctx.Param("id")
		signal.GLOBAL.CloseSignalContext(id, true)
	})

	ctx, stop := ossignal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go messager.Run(ctx)

	if config.Conf().Signal.Healthy.Enable {
		healthConf := healthconfig.DefaultConfig()
		healthConf.FailureNotification.Chan = make(chan error, 1)
		defer close(healthConf.FailureNotification.Chan)
		healthConf.FailureNotification.Threshold = uint32(config.Conf().Signal.Healthy.FailureThreshold)
		healthConf.HealthPath = config.Conf().Signal.Healthy.Path

		signalsCheck := healthchecks.NewContextCheck(ctx, "signals")
		healthcheck.New(g.Engine, healthConf, []healthchecks.Check{signalsCheck})
	}

	err = g.RunWithContext(ctx)
	if err != nil && err != context.Canceled {
		ch <- err
	}
}
