package signalserver

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/log"
	"github.com/vipcxj/conference.go/middleware"
	"github.com/vipcxj/conference.go/signal"
	"github.com/zishang520/socket.io/v2/socket"
)

func Run(ch chan error) {
	signal.InitRouter()
	if !config.Conf().SignalEnable {
		ch <- errors.Ok()
	}
	g := gin.Default()
	g.Use(middleware.ErrorHandler())
	if cors := config.Conf().SignalCors; cors != "" {
		g.Use(middleware.Cors(cors))
	}
	io := socket.NewServer(nil, nil)
	io.Use(middleware.SocketIOAuthHandler())
	io.On("connection", func(clients ...any) {
		socket := clients[0].(*socket.Socket)
		err := signal.InitSignal(socket)
		if err != nil {
			signal.FatalErrorAndClose(socket, signal.ErrToMsg(err), "init signal")
		}
		log.Sugar().Infof(`socket %s connected`, socket.Id())

		// upon disconnection
		socket.On("disconnect", func(reason ...any) {
			log.Sugar().Infof(`socket %s disconnected due to %s`, socket.Id(), reason[0])
		})
	})
	handler := io.ServeHandler(nil)
	g.GET("/socket.io/", gin.WrapH(handler))
	g.POST("/socket.io/", gin.WrapH(handler))

	host := config.Conf().SignalHost
	port := config.Conf().SignalPort
	addr := fmt.Sprintf("%s:%d", host, port)

	if config.Conf().SignalSsl {
		certPath := config.Conf().SignalCertPath
		keyPath := config.Conf().SignalKeyPath
		if certPath == "" || keyPath == "" {
			ch <- errors.FatalError("to enable ssl for auth server, the authServerCertPath and authServerKeyPath must be provided")
			return
		}
		err := g.RunTLS(addr, certPath, keyPath)
		if err != nil {
			ch <- err
		}
	} else {
		err := g.Run(addr)
		if err != nil {
			ch <- err
		}
	}
}
