package errors

import (
	"net/http"
)

type ApiError struct {
	Message    string `json:"error"`
	StatusCode int    `json:"status"`
}

func (e *ApiError) Error() string {
	return e.Message
}

func newError(msg string, status int) *ApiError {
	return &ApiError{msg, status}
}

func JoinError(err error, wrapper *ApiError) *ApiError {
	if err != nil {
		return &ApiError{wrapper.Message + ": " + err.Error(), wrapper.StatusCode}
	}
	return nil
}

var (
	ErrBadRequestBody     = newError("bad request body", http.StatusBadRequest)
	ErrMissingField       = newError("missing required field", http.StatusBadRequest)
	ErrProxyNotFound      = newError("proxy not found", http.StatusNotFound)
	ErrProxyAlreadyExists = newError("proxy already exists", http.StatusConflict)
	ErrInvalidStream      = newError("stream was invalid, can be either upstream or downstream", http.StatusBadRequest)
	ErrInvalidToxicType   = newError("invalid toxic type", http.StatusBadRequest)
	ErrToxicAlreadyExists = newError("toxic already exists", http.StatusConflict)
	ErrToxicNotFound      = newError("toxic not found", http.StatusNotFound)
)
