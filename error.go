package api

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
)

// ErrorInfo is the narrow read-only view of an API error exposed to body
// mappers supplied via WithErrorBody. It carries the semantic payload
// that belongs in a response body — code, message, details, plus the
// request's instance URI — and nothing else. Response-level concerns
// (headers, cookies, status) are handled by the framework outside the
// body mapper.
type ErrorInfo interface {
	// Code returns the error's semantic classification.
	Code() Code

	// Message returns the error's human-readable message.
	Message() string

	// Details returns the attached typed details.
	Details() []any

	// Instance returns a URI reference identifying the specific
	// occurrence of the problem — typically the request's URI. Empty
	// when the error is examined outside a request pipeline.
	Instance() string
}

// Err is the framework's concrete error type. Construct via api.Error;
// consumers interact with it through the error interface and ErrorInfo.
// The struct is exported so wrapping, errors.As, and reflection work
// cleanly, but fields are unexported — all mutation goes through
// ErrorOption.
//
//nolint:errname // Err is deliberately short; the XxxError suffix doesn't fit the framework's idiom.
type Err struct {
	code            Code
	message         string
	details         []any
	headers         http.Header
	cookies         map[string]Cookie
	body            bodyMapper
	cause           error
	documentedCodes []Code // populated by WithErrors when used at scope level
}

// Error is the constructor for framework errors. The code is required;
// everything else — message, headers, cookies, details, body shape — is
// supplied via ErrorOption. Inline options take precedence over any
// scope-level options configured via WithError.
func Error(code Code, opts ...ErrorOption) error {
	e := &Err{code: code}
	for _, o := range opts {
		o.applyErr(e)
	}
	return e
}

// Error implements the error interface. Returns the message if set, or
// the code's canonical status text otherwise.
func (e *Err) Error() string {
	if e.message != "" {
		return e.message
	}
	return http.StatusText(e.code.HTTPStatus())
}

// Code returns the error's semantic code.
func (e *Err) Code() Code { return e.code }

// Message returns the error's message.
func (e *Err) Message() string { return e.message }

// Details returns the attached typed details.
func (e *Err) Details() []any { return e.details }

// Instance returns the empty string. When an *Err is being rendered by
// the framework, it is wrapped in a request-aware view that overrides
// Instance with the request's URI.
func (e *Err) Instance() string { return "" }

// Unwrap exposes a wrapped cause for errors.Is / errors.As chains.
func (e *Err) Unwrap() error { return e.cause }

// StatusCode returns the resolved HTTP status. It comes from the Code
// only — errors cannot override status via options, since every Code
// maps canonically to a status >= 400.
func (e *Err) StatusCode() int { return e.code.HTTPStatus() }

// ErrorOption configures an *Err. The same options work when passed
// inline to api.Error and when collected at router/group/route scope
// via api.WithError.
type ErrorOption interface {
	applyErr(*Err)
}

type errOptFunc func(*Err)

func (f errOptFunc) applyErr(e *Err) { f(e) }

// WithMessage sets the error's message.
func WithMessage(msg string) ErrorOption {
	return errOptFunc(func(e *Err) { e.message = msg })
}

// WithMessagef sets the error's message using Printf-style formatting.
func WithMessagef(format string, args ...any) ErrorOption {
	return errOptFunc(func(e *Err) { e.message = fmt.Sprintf(format, args...) })
}

// WithHeader adds a response header to the error. When applied across
// scopes, later declarations replace earlier ones with the same name.
func WithHeader(name, value string) ErrorOption {
	return errOptFunc(func(e *Err) {
		if e.headers == nil {
			e.headers = make(http.Header)
		}
		e.headers.Set(name, value)
	})
}

// WithCookie sets a response cookie on the error. Later declarations
// with the same name replace earlier ones.
func WithCookie(name string, c Cookie) ErrorOption {
	return errOptFunc(func(e *Err) {
		if e.cookies == nil {
			e.cookies = make(map[string]Cookie)
		}
		e.cookies[name] = c
	})
}

// WithDetail appends a typed detail to the error. Details always
// accumulate across scopes — never replaced.
func WithDetail(d any) ErrorOption {
	return errOptFunc(func(e *Err) {
		e.details = append(e.details, d)
	})
}

// WithCause attaches an underlying error so errors.Is / errors.As can
// traverse the chain. Handy when wrapping a third-party error.
func WithCause(cause error) ErrorOption {
	return errOptFunc(func(e *Err) { e.cause = cause })
}

// WithErrors declares which Codes a route may return. Used for OpenAPI
// documentation only; has no runtime effect. Declarations accumulate
// across scopes.
func WithErrors(codes ...Code) ErrorOption {
	return errOptFunc(func(e *Err) {
		e.documentedCodes = append(e.documentedCodes, codes...)
	})
}

// WithErrorBody installs a body mapper: a function that produces the
// response body's shape from the request context and ErrorInfo. The
// function's return type drives emission:
//
//   - *SomeStruct → encode via negotiated codec (JSON / XML / ...).
//   - *string     → write as text/plain.
//   - nil return  → emit no body for this specific error.
//
// Later declarations replace earlier ones.
func WithErrorBody[T any](fn func(ctx context.Context, e ErrorInfo) *T) ErrorOption {
	mapper := &typedBodyMapper[T]{fn: fn}
	return errOptFunc(func(e *Err) { e.body = mapper })
}

// WithoutErrorBody explicitly opts out of emitting an error body. Errors
// on routes configured this way produce status plus headers only. Use
// when the framework default (ErrorBodyProblemDetails) isn't wanted.
func WithoutErrorBody() ErrorOption {
	return errOptFunc(func(e *Err) { e.body = noBodyMapper{} })
}

// bodyMapper is the internal, type-erased view of a WithErrorBody
// mapping. The framework's emission pipeline dispatches on the concrete
// implementation.
type bodyMapper interface {
	// produce calls the consumer's mapper and returns the body value
	// (a reflect.Value of pointer kind) and a skip flag (true if no
	// body should be emitted — either because the mapper returned nil
	// or because WithoutErrorBody was configured).
	produce(ctx context.Context, info ErrorInfo) (rv reflect.Value, skip bool)

	// elemType is the T in *T, used at registration for schema generation.
	// Returns nil when no body should be emitted.
	elemType() reflect.Type
}

type typedBodyMapper[T any] struct {
	fn func(ctx context.Context, e ErrorInfo) *T
}

func (m *typedBodyMapper[T]) produce(ctx context.Context, info ErrorInfo) (reflect.Value, bool) {
	v := m.fn(ctx, info)
	if v == nil {
		return reflect.Value{}, true
	}
	return reflect.ValueOf(v), false
}

func (m *typedBodyMapper[T]) elemType() reflect.Type {
	var zero T
	return reflect.TypeOf(zero)
}

// noBodyMapper represents WithoutErrorBody: always skip body emission.
type noBodyMapper struct{}

func (noBodyMapper) produce(context.Context, ErrorInfo) (reflect.Value, bool) {
	return reflect.Value{}, true
}

func (noBodyMapper) elemType() reflect.Type { return nil }

// Compile-time assertions that Err satisfies the interfaces it promises.
//
//nolint:errcheck // these aren't error returns; the pattern is Go's standard compile-time check idiom.
var (
	_ error     = (*Err)(nil)
	_ ErrorInfo = (*Err)(nil)
)
