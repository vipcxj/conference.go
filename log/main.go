package log

import (
	"github.com/vipcxj/conference.go/config"
	"go.uber.org/zap"
)



func Init() {
	var err error
	var logger *zap.Logger
	switch config.Conf().LogProfile() {
	case config.LOG_PROFILE_DEVELOPMENT:
		logger, err = zap.NewDevelopment()
	case config.LOG_PROFILE_PRODUCTION:
		logger, err = zap.NewProduction()
	}
	if err != nil {
		panic(err)
	}
	zap.ReplaceGlobals(logger)
}

func Logger() *zap.Logger {
	return zap.L()
}

func Sugar() *zap.SugaredLogger {
	return zap.S()
}