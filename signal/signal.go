package signal

import (
	"time"

	"github.com/vipcxj/conference.go/model"
	"go.uber.org/zap"
)

type AckFunc = func([]any, error)

type Signal interface {
	SendMsg(timeout time.Duration, ack bool, evt string, args ...any) (res []any, err error)
	On(evt string, cb func(ack AckFunc, args ...any)) error
	OnCustom(cb func(evt string, msg *model.CustomMessage)) error
	OnCustomAck(cb func(msg *model.CustomAckMessage)) error
	OnClose(cb func(args ...any))
	GetContext() *SignalContext
	Sugar() *zap.SugaredLogger
	Close()
}
