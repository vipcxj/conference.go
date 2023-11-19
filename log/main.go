package log

import (
	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/errors"
	"go.uber.org/zap"
)

func Init() {
	atLevel, err := zap.ParseAtomicLevel(config.Conf().Log.Level)
	if err != nil {
		panic(err)
	}
	var logger = MustCreate(nil)
	zap.ReplaceGlobals(logger)
}

func MustCreate(logger *zap.Logger, options ...zap.Option) *zap.Logger {
	if logger == nil {
		var err error
		switch config.Conf().LogProfile() {
		case config.LOG_PROFILE_DEVELOPMENT:
			logger, err = zap.NewDevelopment(options...)
		case config.LOG_PROFILE_PRODUCTION:
			logger, err = zap.NewProduction(options...)
		default:
			err = errors.ThisIsImpossible().GenCallStacks()
		}
		if err != nil {
			panic(err)
		}
		return logger
	}
	return logger.WithOptions(options...)
}

func Logger() *zap.Logger {
	return zap.L()
}

func Sugar() *zap.SugaredLogger {
	return zap.S()
}
