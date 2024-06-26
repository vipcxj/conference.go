package errors

import (
	_errors "errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"
)

const (
	ERR_OK    = 0
	ERR_FATAL = 1000

	CLIENT_ERR = 5000

	BAD_PACKET          = 10000
	INVALID_PUB_PATTERN = 11000
	SUB_NOT_EXIST       = 12000
	ROOM_NO_RIGHT       = 13000

	INVALID_CONFIG  = 14000
	INVALID_MESSAGE = 15000

	MSG_TIMEOUT = 16000

	SIGNAL_CONTEXT_CLOSED = 20000

	INVALID_STATE = 10000000
)

type CallFrame struct {
	Filename string `json:"filename" mapstructure:"filename"`
	Line     int    `json:"line" mapstructure:"line"`
	FuncName string `json:"funcname" mapstructure:"funcname"`
}

type ConferenceError struct {
	Code       int         `json:"code" mapstructure:"code"`
	Msg        string      `json:"msg" mapstructure:"msg"`
	Data       interface{} `json:"data" mapstructure:"data"`
	CallFrames []CallFrame `json:"callFrames,omitempty" mapstructure:"callFrames"`
	Cause      string      `json:"cause" mapstructure:"cause"`
	cause      error
}

func (e *ConferenceError) GenCallStacks(ignores int) *ConferenceError {
	callers := make([]uintptr, 128)
	n := runtime.Callers(ignores+1, callers)
	frames := runtime.CallersFrames(callers[:n])
	callFrames := make([]CallFrame, 0, n)
	for {
		frame, more := frames.Next()
		if !more {
			break
		}
		callFrames = append(callFrames, CallFrame{
			Filename: frame.File,
			Line:     frame.Line,
			FuncName: frame.Function,
		})
	}
	e.CallFrames = callFrames
	return e
}

func (e *ConferenceError) Error() string {
	return e.Msg
}

func (e *ConferenceError) Unwrap() error {
	return e.cause
}

func (e *ConferenceError) ToString() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%d] %v", e.Code, e.Msg))
	if len(e.CallFrames) > 0 {
		sb.WriteString("\n")
		for _, f := range e.CallFrames {
			sb.WriteString("  ")
			sb.WriteString("At ")
			sb.WriteString(f.Filename)
			sb.WriteString(" - ")
			sb.WriteString(f.FuncName)
			sb.WriteString(fmt.Sprintf("[%d]", f.Line))
		}
	}
	return sb.String()
}

func NewError(code int, msg string, args ...any) *ConferenceError {
	err := fmt.Errorf(msg, args...)
	cause := _errors.Unwrap(err)
	cause_msg := ""
	if cause != nil {
		cause_msg = cause.Error()
	}
	res := &ConferenceError{
		Msg:   err.Error(),
		Code:  code,
		cause: _errors.Unwrap(err),
		Cause: cause_msg,
	}
	cfgo_err, ok := cause.(*ConferenceError)
	if ok {
		res.Data = cfgo_err.Data
		res.CallFrames = cfgo_err.CallFrames
	}
	return res
}

func NewStackError(ignores int, code int, msg string, args ...any) *ConferenceError {
	return NewError(code, msg, args...).GenCallStacks(ignores + 1)
}

func MaybeWrapError(cause error) *ConferenceError {
	cfgo_err, ok := cause.(*ConferenceError)
	if ok {
		return cfgo_err
	} else {
		return NewError(ERR_FATAL, "%w", cause)
	}
}

func IsOk(err error) bool {
	if myErr, ok := err.(*ConferenceError); ok {
		return myErr.Code == ERR_OK
	}
	return false
}

func Ok() *ConferenceError {
	return NewError(ERR_OK, "")
}

func Unauthorized(msg string, args ...any) *ConferenceError {
	return NewError(http.StatusUnauthorized, msg, args...)
}

func InvalidParam(msg string, args ...any) *ConferenceError {
	return NewError(http.StatusBadRequest, msg, args...)
}

func FatalError(msg string, args ...any) *ConferenceError {
	return NewError(ERR_FATAL, msg, args...)
}

func BadPacket(msg string, args ...any) *ConferenceError {
	return NewError(BAD_PACKET, msg, args...)
}

func ClientError(arg any) *ConferenceError {
	return NewError(CLIENT_ERR, fmt.Sprintf("%v", arg))
}

func InvalidPubPattern(msg string, args ...any) *ConferenceError {
	return NewError(INVALID_PUB_PATTERN, msg, args...)
}

func RoomNoRight(room string) *ConferenceError {
	return NewError(ROOM_NO_RIGHT, "no right for room %s", room)
}

func SubNotExist(subId string) *ConferenceError {
	return NewError(SUB_NOT_EXIST, "the subscription %v does not exists", subId)
}

func InvalidState(msg string, args ...any) *ConferenceError {
	return NewError(INVALID_STATE, msg, args...)
}

func InvalidConfig(msg string, args ...any) *ConferenceError {
	return NewError(INVALID_CONFIG, msg, args...)
}

func InvalidMessage(msg string, args ...any) *ConferenceError {
	return NewError(INVALID_MESSAGE, msg, args...)
}

func MsgTimeout(msg string, args ...any) *ConferenceError {
	return NewError(MSG_TIMEOUT, msg, args...)
}

func SignalContextClosed() *ConferenceError {
	return NewError(SIGNAL_CONTEXT_CLOSED, "the signal context is closed")
}

func ThisIsImpossible() *ConferenceError {
	return NewError(INVALID_STATE, "this is impossible, if happened, must be a bug")
}

func Join(errs ...error) error {
	return _errors.Join(errs...)
}

func Is(err error, target error) bool {
	return _errors.Is(err, target)
}

func Ignore(errs ...error) {
	if r := recover(); r != nil {
		switch e := r.(type) {
		case error:
			for _, err := range errs {
				if Is(e, err) {
					return
				}
			}
		}
		panic(r)
	}
}

func Recover(doctors map[error]func(error) bool) {
	if r := recover(); r != nil {
		switch e := r.(type) {
		case error:
			for err, docter := range doctors {
				if Is(e, err) {
					if docter(e) {
						return
					}
				}
			}
		}
		panic(r)
	}
}
