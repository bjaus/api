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

// ProblemDetail is an RFC 9457 problem details response, used as the body
// shape returned by the built-in ValidationErrorBuilder.
//
//nolint:errname // RFC 9457 standard name
type ProblemDetail struct {
	Type     string            `json:"type,omitempty"`
	Title    string            `json:"title,omitempty"`
	Status   int               `json:"status"`
	Detail   string            `json:"detail,omitempty"`
	Instance string            `json:"instance,omitempty"`
	Errors   []ValidationError `json:"errors,omitempty"`
}

// Error returns the detail message (or title if detail is empty).
func (p *ProblemDetail) Error() string {
	if p.Detail != "" {
		return p.Detail
	}
	return p.Title
}

// StatusCode returns the HTTP status code.
func (p *ProblemDetail) StatusCode() int { return p.Status }

// ValidationError describes a single field validation failure.
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
