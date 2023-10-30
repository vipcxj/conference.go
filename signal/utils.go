package signal

import (
	"reflect"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/vipcxj/conference.go/errors"
	"github.com/zishang520/socket.io/v2/socket"
)

func FatalErrorAndClose(s *socket.Socket, msg string, cause string) {
	s.Timeout(time.Second*1).EmitWithAck("error", &ErrorMessage{
		Msg:   msg,
		Fatal: true,
		Cause: cause,
	})(func(a []any, err error) {
		s.Disconnect(true)
	})
}

func CatchFatalAndClose(s *socket.Socket, cause string) {
	if err := recover(); err != nil {
		FatalErrorAndClose(s, err.(error).Error(), cause)
	}
}

func parseArgs[R any, PR *R](out PR, args ...any) (func(...any), error) {
	if len(args) == 0 {
		if out == nil {
			return nil, nil
		} else {
			return nil, errors.FatalError("too little parameter")
		}
	}
	last := args[len(args)-1]
	var ark func(...any) = nil
	if reflect.TypeOf(last).Kind() == reflect.Func {
		ark = last.(func(...any))
		args = args[0 : len(args)-1]
	}
	if len(args) == 0 {
		if out == nil {
			return nil, nil
		} else {
			return nil, errors.FatalError("too little parameter")
		}
	}
	err := mapstructure.Decode(args[0], out)
	return ark, err
}

func doArk(ark func(...any), payload any) {
	if ark != nil {
		ark(payload)
	}
}
