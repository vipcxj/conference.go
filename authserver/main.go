package authserver

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/vipcxj/conference.go/auth"
	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/middleware"
)

func Run(ch chan error) {
	if !config.Conf().AuthServerEnable {
		ch <- errors.Ok()
	}
	g := gin.Default()
	g.Use(middleware.ErrorHandler())
	if cors := config.Conf().AuthServerCors; cors != "" {
		g.Use(middleware.Cors(cors))
	}
	g.GET("/token", func(ctx *gin.Context) {
		ctx.Request.ParseForm()
		authInfo, err := auth.NewAuthInfoFromForm(ctx.Request.Form)
		if err != nil {
			panic(err)
		}
		token, err := auth.Encode(authInfo)
		if err != nil {
			panic(err)
		}
		ctx.String(http.StatusOK, token)
	})
	host := config.Conf().AuthServerHost
	port := config.Conf().AuthServerPort
	addr := fmt.Sprintf("%s:%d", host, port)
	if config.Conf().AuthServerSsl {
		certPath := config.Conf().AuthServerCertPath
		keyPath := config.Conf().AuthServerKeyPath
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
