package signal

import (
	"fmt"
	"reflect"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/utils"
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

func ErrToMsg(err any) string {
	var msg string
	switch e := err.(type) {
	case error:
		msg = e.Error()
	default:
		msg = fmt.Sprint(e)
	}
	return fmt.Sprintf("%s\n%s\n", msg, string(debug.Stack()))
}

func CatchFatalAndClose(s *socket.Socket, cause string) {
	if err := recover(); err != nil {
		FatalErrorAndClose(s, ErrToMsg(err), cause)
	}
}

func parseArgs[R any, PR *R](out PR, args ...any) (func([]any, error), error) {
	if len(args) == 0 {
		if out == nil {
			return nil, nil
		} else {
			return nil, errors.FatalError("too little parameter")
		}
	}
	last := args[len(args)-1]
	var ark func([]any, error) = nil
	if reflect.TypeOf(last).Kind() == reflect.Func {
		ark = last.(func([]any, error))
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

func doArk(ark func([]any, error), payload any) {
	if ark != nil {
		ark([]any{payload}, nil)
	}
}

func NewUUID(id string, base string) string {
	newBytes := uuid.MustParse(id)
	baseBytes := uuid.MustParse(base)
	utils.XOrBytes(newBytes[:], baseBytes[:], newBytes[:])
	return newBytes.String()
}