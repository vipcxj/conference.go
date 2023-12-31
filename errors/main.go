package errors

import (
	_errors "errors"
	"fmt"
	"net/http"
	"runtime"
)

const (
	ERR_OK    = 0
	ERR_FATAL = 1000

	BAD_PACKET          = 10000
	INVALID_PUB_PATTERN = 11000
	SUB_NOT_EXIST       = 12000
	ROOM_NO_RIGHT       = 13000

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
	CallFrames []CallFrame `json:"callFrames" mapstructure:"callFrames"`
}

func (e *ConferenceError) GenCallStacks() *ConferenceError {
	callers := make([]uintptr, 128)
	n := runtime.Callers(1, callers)
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

func NewError(code int, msg string, args ...any) *ConferenceError {
	return &ConferenceError{
		Msg:  fmt.Sprintf(msg, args...),
		Code: code,
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
