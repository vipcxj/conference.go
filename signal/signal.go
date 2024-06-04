package signal

import (
	"time"

	"go.uber.org/zap"
)

type AckFunc = func([]any, error)

type Signal interface {
	SendMsg(timeout time.Duration, ack bool, evt string, args ...any) (res []any, err error)
	On(evt string, cb func(ack AckFunc, args ...any)) error
	GetContext() *SignalContext
	Sugar() *zap.SugaredLogger
	Close()
}
