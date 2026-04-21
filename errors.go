package api

import (
	"errors"
	"net/http"
)

// Sentinel errors for request binding.
var (
	ErrBindPath   = errors.New("bind path")
	ErrBindQuery  = errors.New("bind query")
	ErrBindHeader = errors.New("bind header")
	ErrBindCookie = errors.New("bind cookie")
	ErrBindBody   = errors.New("bind body")
	ErrBindForm   = errors.New("bind form")
)

// StatusCoder is implemented by errors that carry an HTTP status code.
// The framework's own *Err implements it via Code.HTTPStatus().
type StatusCoder interface {
	StatusCode() int
}

// ValidationError describes a single field validation failure. Framework
// validators attach one per failed field; a slice is returned as
// ValidationErrors for the framework to route through the error pipeline.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   any    `json:"value,omitempty"`
}

// Error returns the validation error message.
func (e *ValidationError) Error() string { return e.Message }

// ErrorStatus extracts the HTTP status code from an error. Returns
// http.StatusInternalServerError if the error does not implement
// StatusCoder.
func ErrorStatus(err error) int {
	var sc StatusCoder
	if errors.As(err, &sc) {
		return sc.StatusCode()
	}
	return http.StatusInternalServerError
}
