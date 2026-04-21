package api

import (
	"context"
	"net/http"
)

// ProblemDetails is the RFC 9457 Problem Details for HTTP APIs shape.
// It is the framework's default error response body; consumers can
// override this via WithErrorBody. All fields are exported so that
// custom body mappers can tweak individual values without reconstructing
// the whole struct.
//
// The first five fields (Type through Instance) are defined by RFC 9457.
// Code and Errors are framework-provided extensions, permitted by the
// RFC's extensibility rules.
type ProblemDetails struct {
	// Type is a URI reference identifying the problem type. Defaults to
	// "about:blank", per the RFC, which implies the problem has no
	// additional semantics beyond those of the HTTP status code.
	Type string `json:"type,omitempty"`

	// Title is a short, human-readable summary of the problem type. It
	// defaults to the standard HTTP status text (e.g., "Not Found").
	Title string `json:"title,omitempty"`

	// Status is the HTTP status code, derived from the error's Code.
	Status int `json:"status"`

	// Detail is a human-readable explanation specific to this occurrence,
	// taken from the error's message.
	Detail string `json:"detail,omitempty"`

	// Instance is a URI reference identifying the specific occurrence of
	// the problem — by default the request's URI.
	Instance string `json:"instance,omitempty"`

	// Code is the framework's semantic classification of the error.
	// (RFC 9457 extension.)
	Code Code `json:"code,omitempty"`

	// Errors carries the error's attached details (validation failures,
	// retry hints, etc.). (RFC 9457 extension.)
	Errors []any `json:"errors,omitempty"`
}

// ContentType returns the RFC 9457 media type for this body shape.
// Implementing the contentTyped interface causes the framework to set
// Content-Type: application/problem+json instead of codec-negotiating.
func (*ProblemDetails) ContentType() string { return "application/problem+json" }

// NewProblemDetails constructs a ProblemDetails populated from the
// error information, using RFC 9457 defaults for every field. Consumers
// writing custom body mappers can call this to get the defaulted struct
// and then overwrite individual fields.
func NewProblemDetails(e ErrorInfo) *ProblemDetails {
	status := e.Code().HTTPStatus()
	return &ProblemDetails{
		Type:     "about:blank",
		Title:    http.StatusText(status),
		Status:   status,
		Detail:   e.Message(),
		Instance: e.Instance(),
		Code:     e.Code(),
		Errors:   e.Details(),
	}
}

// ErrorBodyProblemDetails is the framework's default body mapper. It
// returns an RFC 9457 ProblemDetails for every error. Pass it to
// WithErrorBody to make the default explicit or to restore it at an
// inner scope.
func ErrorBodyProblemDetails(_ context.Context, e ErrorInfo) *ProblemDetails {
	return NewProblemDetails(e)
}

// ErrorBodyText is a body mapper that emits the error's message as a
// text/plain response body.
func ErrorBodyText(_ context.Context, e ErrorInfo) *string {
	return new(e.Message())
}
