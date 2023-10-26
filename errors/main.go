package errors

import "net/http"

const (
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

func InvalidParam(msg string) *ConferenceError {
	return NewError(http.StatusBadRequest, msg)
}

func FatalError(msg string) *ConferenceError {
	return NewError(ERR_FATAL, msg)
}