package main

import (
	"errors"
	"fmt"

	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/middleware"

	"github.com/gin-gonic/gin"
	"github.com/zishang520/socket.io/v2/socket"
)

func main() {
	config.Init()
	g := gin.Default()
	g.Use(middleware.ErrorHandler())
	io := socket.NewServer(nil, nil)
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
			panic(errors.New("to enable ssl for auth server, the authServerCertPath and authServerKeyPath must be provided"))
		}
		err := g.RunTLS(addr, certPath, keyPath)
		if err != nil {
			panic(err)
		}
	} else {
		err := g.Run(addr)
		if err != nil {
			panic(err)
		}
	}
}