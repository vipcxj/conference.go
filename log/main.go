package log

import (
	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/errors"
	"go.uber.org/zap"
)



func Init() {
	var logger = MustCreate(nil)
	zap.ReplaceGlobals(logger)
}

func MustCreate(logger *zap.Logger, fields ...zap.Field) *zap.Logger {
	if logger == nil {
		var err error
		switch config.Conf().LogProfile() {
		case config.LOG_PROFILE_DEVELOPMENT:
			logger, err = zap.NewDevelopment()
		case config.LOG_PROFILE_PRODUCTION:
			logger, err = zap.NewProduction()
		default:
			err = errors.ThisIsImpossible().GenCallStacks()
		}
		if err != nil {
			panic(err)
		}
	}
	return logger.With(fields...)
}

func Logger() *zap.Logger {
	return zap.L()
}

func Sugar() *zap.SugaredLogger {
	return zap.S()
}