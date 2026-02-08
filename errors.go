package api

import (
	"errors"
	"fmt"
	"net/http"
)

// Sentinel errors for request binding.
var (
	ErrBindPath   = errors.New("bind path")
	ErrBindQuery  = errors.New("bind query")
	ErrBindHeader = errors.New("bind header")
	ErrBindCookie = errors.New("bind cookie")
	ErrBindBody   = errors.New("bind body")
)

// StatusCoder is implemented by errors or responses that carry an HTTP status code.
type StatusCoder interface {
	StatusCode() int
}

// HTTPError is an error with an HTTP status code.
type HTTPError struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

// Error returns the error message.
func (e *HTTPError) Error() string { return e.Message }

// StatusCode returns the HTTP status code.
func (e *HTTPError) StatusCode() int { return e.Status }

// Error returns an error with the given HTTP status code and message.
func Error(status int, message string) error {
	return &HTTPError{Status: status, Message: message}
}

// Errorf returns a formatted error with the given HTTP status code.
func Errorf(status int, format string, args ...any) error {
	return &HTTPError{Status: status, Message: fmt.Sprintf(format, args...)}
}

// ErrorStatus extracts the HTTP status code from an error. Returns
// http.StatusInternalServerError if the error does not implement StatusCoder.
func ErrorStatus(err error) int {
	var sc StatusCoder
	if errors.As(err, &sc) {
		return sc.StatusCode()
	}
	return http.StatusInternalServerError
}
