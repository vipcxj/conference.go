package authserver

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	healthcheck "github.com/tavsec/gin-healthcheck"
	healthchecks "github.com/tavsec/gin-healthcheck/checks"
	healthconfig "github.com/tavsec/gin-healthcheck/config"
	"github.com/vipcxj/conference.go/auth"
	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/middleware"
)

func Run(conf *config.ConferenceConfigure, ctx context.Context, done context.CancelCauseFunc) {
	if !conf.AuthServer.Enable {
		done(nil)
		return
	}
	g := gin.Default()
	g.Use(middleware.ErrorHandler())
	if cors := conf.AuthServer.Cors; cors != "" {
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
	if conf.AuthServer.Healthy.Enable {
		healthConf := healthconfig.DefaultConfig()
		healthConf.FailureNotification.Chan = make(chan error, 1)
		defer close(healthConf.FailureNotification.Chan)
		healthConf.FailureNotification.Threshold = uint32(conf.Signal.Healthy.FailureThreshold)
		healthConf.HealthPath = conf.Signal.Healthy.Path

		authCheck := healthchecks.NewContextCheck(ctx, "auth")
		healthcheck.New(g, healthConf, []healthchecks.Check{authCheck})
	}
	host := conf.AuthServer.Host
	port := conf.AuthServer.Port
	addr := fmt.Sprintf("%s:%d", host, port)
	if conf.AuthServer.Ssl {
		certPath := conf.AuthServer.CertPath
		keyPath := conf.AuthServer.KeyPath
		if certPath == "" || keyPath == "" {
			done(errors.FatalError("to enable ssl for auth server, the authServerCertPath and authServerKeyPath must be provided"))
			return
		}
		err := g.RunTLS(addr, certPath, keyPath)
		done(err)
	} else {
		err := g.Run(addr)
		done(err)
	}
}
