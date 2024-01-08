package signalserver

import (
	"context"
	"fmt"
	ossignal "os/signal"
	"syscall"

	"github.com/gin-contrib/graceful"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	healthcheck "github.com/tavsec/gin-healthcheck"
	healthchecks "github.com/tavsec/gin-healthcheck/checks"
	healthconfig "github.com/tavsec/gin-healthcheck/config"

	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/middleware"
	"github.com/vipcxj/conference.go/signal"
	"github.com/zishang520/socket.io/v2/socket"
)

func Run(ch chan error) {
	signal.InitRouter()
	if !config.Conf().SignalEnable {
		ch <- errors.Ok()
		return
	}

	host := config.Conf().SignalHost
	port := config.Conf().SignalPort
	addr := fmt.Sprintf("%s:%d", host, port)
	var err error
	var g *graceful.Graceful
	if config.Conf().SignalSsl {
		certPath := config.Conf().SignalCertPath
		keyPath := config.Conf().SignalKeyPath
		if certPath == "" || keyPath == "" {
			ch <- errors.FatalError("to enable ssl for auth server, the authServerCertPath and authServerKeyPath must be provided")
			return
		}
		g, err = graceful.New(gin.Default(), graceful.WithTLS(addr, certPath, keyPath))
	} else {
		g, err = graceful.New(gin.Default())
	}
	if err != nil {
		ch <- err
		return
	}
	defer g.Close()

	pprof.Register(g.Engine)
	g.Use(middleware.ErrorHandler())
	if cors := config.Conf().SignalCors; cors != "" {
		g.Use(middleware.Cors(cors))
	}
	io := socket.NewServer(nil, nil)
	io.Use(middleware.SocketIOAuthHandler())
	io.On("connection", func(clients ...any) {
		fmt.Printf("on connection\n")
		socket := clients[0].(*socket.Socket)
		ctx, err := signal.InitSignal(socket)
		if err != nil {
			ctx.Sugar().Errorf("socket connect failed, %v", err)
			signal.FatalErrorAndClose(socket, signal.ErrToMsg(err), "init signal")
		} else {
			ctx.Sugar().Info("socket connected")
		}
	})
	handler := io.ServeHandler(nil)
	g.GET("/socket.io/", gin.WrapH(handler))
	g.POST("/socket.io/", gin.WrapH(handler))
	g.GET(fmt.Sprintf("%v/:id", signal.CLOSE_CALLBACK_PREFIX), func(ctx *gin.Context) {
		id := ctx.Param("id")
		signal.GLOBAL.CloseSignalContext(id, true)
	})

	ctx, stop := ossignal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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
