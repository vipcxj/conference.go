package errors

import "net/http"

const (
	ERR_OK = 0
	ERR_FATAL = 1000
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