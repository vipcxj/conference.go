package log

import (
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func Init(level string, profile config.LogProfile, file string) {
	atLevel, err := zap.ParseAtomicLevel(level)
	if err != nil {
		panic(err)
	}
	var cfg zap.Config
	switch profile {
	case config.LOG_PROFILE_DEVELOPMENT:
		cfg = zap.NewDevelopmentConfig()
	case config.LOG_PROFILE_PRODUCTION:
		cfg = zap.NewProductionConfig()
	default:
		err = errors.ThisIsImpossible().GenCallStacks(0)
		panic(err)
	}
	cfg.Level = atLevel
	if len(file) > 0 {
		cfg.OutputPaths = []string {
			file,
		}
	}
	logger, err := cfg.Build()
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

type PionLogger struct {
	logger *zap.Logger
}

func NewPionLogger(logger *zap.Logger) *PionLogger {
	return &PionLogger{logger: logger}
}

func (pl *PionLogger) Trace(msg string) {
	if pl != nil && pl.logger != nil {
		pl.logger.Debug(msg)
	}
}

func (pl *PionLogger) Tracef(format string, args ...interface{}) {
	if pl != nil && pl.logger != nil {
		pl.logger.Debug(fmt.Sprintf(format, args...))
	}
}

func (pl *PionLogger) Debug(msg string) {
	if pl != nil && pl.logger != nil {
		pl.logger.Debug(msg)
	}
}

func (pl *PionLogger) Debugf(format string, args ...interface{}) {
	if pl != nil && pl.logger != nil {
		pl.logger.Debug(fmt.Sprintf(format, args...))
	}
}

func (pl *PionLogger) Info(msg string) {
	if pl != nil && pl.logger != nil {
		pl.logger.Info(msg)
	}
}

func (pl *PionLogger) Infof(format string, args ...interface{}) {
	if pl != nil && pl.logger != nil {
		pl.logger.Info(fmt.Sprintf(format, args...))
	}
}

func (pl *PionLogger) Warn(msg string) {
	if pl != nil && pl.logger != nil {
		pl.logger.Warn(msg)
	}
}

func (pl *PionLogger) Warnf(format string, args ...interface{}) {
	if pl != nil && pl.logger != nil {
		pl.logger.Warn(fmt.Sprintf(format, args...))
	}
}

func (pl *PionLogger) Error(msg string) {
	if pl != nil && pl.logger != nil {
		pl.logger.Error(msg)
	}
}

func (pl *PionLogger) Errorf(format string, args ...interface{}) {
	if pl != nil && pl.logger != nil {
		pl.logger.Error(fmt.Sprintf(format, args...))
	}
}

type PionSugar struct {
	sugar *zap.SugaredLogger
}

func NewPionSugar(sugar *zap.SugaredLogger) *PionSugar {
	return &PionSugar{sugar: sugar}
}

func (ps *PionSugar) Trace(msg string) {
	if ps != nil && ps.sugar != nil {
		ps.sugar.Debug(msg)
	}
}

func (ps *PionSugar) Tracef(format string, args ...interface{}) {
	if ps != nil && ps.sugar != nil {
		ps.sugar.Debugf(format, args...)
	}
}

func (ps *PionSugar) Debug(msg string) {
	if ps != nil && ps.sugar != nil {
		ps.sugar.Debug(msg)
	}
}

func (ps *PionSugar) Debugf(format string, args ...interface{}) {
	if ps != nil && ps.sugar != nil {
		ps.sugar.Debugf(format, args...)
	}
}

func (ps *PionSugar) Info(msg string) {
	if ps != nil && ps.sugar != nil {
		ps.sugar.Info(msg)
	}
}

func (ps *PionSugar) Infof(format string, args ...interface{}) {
	if ps != nil && ps.sugar != nil {
		ps.sugar.Infof(format, args...)
	}
}

func (ps *PionSugar) Warn(msg string) {
	if ps != nil && ps.sugar != nil {
		ps.sugar.Warn(msg)
	}
}

func (ps *PionSugar) Warnf(format string, args ...interface{}) {
	if ps != nil && ps.sugar != nil {
		ps.sugar.Warnf(format, args...)
	}
}

func (ps *PionSugar) Error(msg string) {
	if ps != nil && ps.sugar != nil {
		ps.sugar.Error(msg)
	}
}

func (ps *PionSugar) Errorf(format string, args ...interface{}) {
	if ps != nil && ps.sugar != nil {
		ps.sugar.Errorf(format, args...)
	}
}

func zapLvl2KgoLevel(lvl zapcore.Level) kgo.LogLevel {
	if lvl == zap.DebugLevel {
		return kgo.LogLevelDebug
	} else if lvl == zap.InfoLevel {
		return kgo.LogLevelInfo
	} else if lvl == zap.WarnLevel {
		return kgo.LogLevelWarn
	} else {
		return kgo.LogLevelError
	}
}

func kgoLevel2ZapLvl(level kgo.LogLevel) zapcore.Level {
	var lvl zapcore.Level
	switch level {
	case kgo.LogLevelDebug:
		lvl = zap.DebugLevel
	case kgo.LogLevelInfo:
		lvl = zap.InfoLevel
	case kgo.LogLevelWarn:
		lvl = zap.WarnLevel
	case kgo.LogLevelError:
		lvl = zap.ErrorLevel
	}
	return lvl
}

type KgoLogger struct {
	logger *zap.Logger
}

func NewKgoLogger(logger *zap.Logger) *KgoLogger {
	return &KgoLogger{logger: logger}
}

func (kl *KgoLogger) Level() kgo.LogLevel {
	lvl := kl.logger.Level()
	return zapLvl2KgoLevel(lvl)
}

func kgoKeyVals2ZapFields(keyvals ...any) ([]zapcore.Field, error) {
	argv := len(keyvals)
	if argv%2 != 0 {
		return nil, fmt.Errorf("wrong keyvals num")
	}
	fields := make([]zapcore.Field, 0, argv/2)
	for i := 0; i < argv/2; i++ {
		key, ok := keyvals[i*2].(string)
		if !ok {
			return nil, fmt.Errorf("key must be string")
		}
		fields = append(fields, zap.Any(key, keyvals[i*2+1]))
	}
	return fields, nil
}

func (kl *KgoLogger) Log(level kgo.LogLevel, msg string, keyvals ...any) {
	lvl := kgoLevel2ZapLvl(level)
	fields, err := kgoKeyVals2ZapFields(keyvals...)
	if err != nil {
		panic(err)
	}
	kl.logger.Log(lvl, msg, fields...)
}

type KgoSugar struct {
	sugar *zap.SugaredLogger
}

func NewKgoSugar(sugar *zap.SugaredLogger) *KgoSugar {
	return &KgoSugar{sugar: sugar}
}

func (kl *KgoSugar) Level() kgo.LogLevel {
	lvl := kl.sugar.Level()
	return zapLvl2KgoLevel(lvl)
}

func (kl *KgoSugar) Log(level kgo.LogLevel, msg string, keyvals ...any) {
	lvl := kgoLevel2ZapLvl(level)
	kl.sugar.Logw(lvl, msg, keyvals...)
}