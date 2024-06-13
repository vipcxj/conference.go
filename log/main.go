package log

import (
	"os"

	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func Init(level string, profile config.LogProfile) {
	atLevel, err := zap.ParseAtomicLevel(level)
	if err != nil {
		panic(err)
	}
	var encoderCfg zapcore.EncoderConfig
	switch profile {
	case config.LOG_PROFILE_DEVELOPMENT:
		encoderCfg = zap.NewDevelopmentEncoderConfig()
	case config.LOG_PROFILE_PRODUCTION:
		encoderCfg = zap.NewProductionEncoderConfig()
	default:
		err = errors.ThisIsImpossible().GenCallStacks(0)
		panic(err)
	}
	var logger = zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.Lock(os.Stdout),
		atLevel,
	))
	zap.ReplaceGlobals(logger)
}

func Logger() *zap.Logger {
	return zap.L()
}

func Sugar() *zap.SugaredLogger {
	return zap.S()
}
