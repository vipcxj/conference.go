package errors

import (
	_errors "errors"
	"net/http"
)

const (
	ERR_OK = 0
	ERR_FATAL = 1000

	BAD_PACKET = 10000
)

type ConferenceError struct {
	Code int
	Msg  string
	Data interface{}
}

func (e *ConferenceError) Error() string {
	return e.Msg
}

func NewError(code int, msg string) *ConferenceError {
	return &ConferenceError{
		Msg:  msg,
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

func Unauthorized(msg string) *ConferenceError {
	return NewError(http.StatusUnauthorized, msg)
}

func InvalidParam(msg string) *ConferenceError {
	return NewError(http.StatusBadRequest, msg)
}

func FatalError(msg string) *ConferenceError {
	return NewError(ERR_FATAL, msg)
}

func BadPacket(msg string) *ConferenceError {
	return NewError(BAD_PACKET, msg)
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
			for err, docter := range(doctors) {
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