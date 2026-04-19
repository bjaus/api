package api

import (
	"context"
	"fmt"
	"net/http"
)

// Validator is optionally implemented by request types to validate themselves
// after binding. Implementations that don't need the context can ignore it.
//
// Return api.ValidationErrors to have the framework route the failures through
// the configured ValidationErrorBuilder. Any other error type is forwarded to
// the router's ErrorHandler untouched.
type Validator interface {
	Validate(ctx context.Context) error
}

// ValidatorFunc is the router-level validation plugin. It runs for every
// request after binding and per-type validation. Typical use is to wrap a
// reflection-based validator library:
//
//	r := api.New(api.WithValidator(func(req any) error {
//	    return playground.New().Struct(req)
//	}))
//
// Return api.ValidationErrors to get the configured builder; any other error
// is forwarded as-is.
type ValidatorFunc func(req any) error

// ValidationErrors is a collection of field-level validation failures.
// Any layer in the validation pipeline that returns this type (or wraps it
// via fmt.Errorf %w) is routed through the router's ValidationErrorBuilder.
type ValidationErrors []ValidationError

// Error implements the error interface.
func (v ValidationErrors) Error() string {
	return fmt.Sprintf("%d validation error(s)", len(v))
}

// ValidationErrorBuilder shapes a slice of ValidationError into the response
// error returned to the router's ErrorHandler. Supply one via
// WithValidationErrorBuilder to unify validation errors with a domain error
// taxonomy.
type ValidationErrorBuilder interface {
	Build(violations ValidationErrors) error
}

// ValidationMode controls when the constraint-tag checks run relative to the
// request type's Validator and the router-level ValidatorFunc.
type ValidationMode int

const (
	// ValidateConstraintsLast runs the request's Validator, then the router's
	// ValidatorFunc, then constraint-tag enforcement. The consumer's validator
	// produces user-facing messages; tags act as a spec-fidelity backstop.
	// This is the default.
	ValidateConstraintsLast ValidationMode = iota

	// ValidateConstraintsFirst runs constraint-tag enforcement first, failing
	// fast on spec violations before any user validator runs.
	ValidateConstraintsFirst

	// ValidateConstraintsOff disables runtime enforcement of constraint tags.
	// Tags still feed the OpenAPI spec; runtime validation is entirely owned
	// by the consumer's Validator and ValidatorFunc.
	ValidateConstraintsOff
)

// defaultValidationErrorBuilder builds a *ProblemDetail from a
// ValidationErrors slice. Used when no custom builder is configured.
type defaultValidationErrorBuilder struct{}

func (defaultValidationErrorBuilder) Build(violations ValidationErrors) error {
	return &ProblemDetail{
		Type:   "about:blank",
		Title:  "Validation Failed",
		Status: http.StatusBadRequest,
		Detail: fmt.Sprintf("%d validation error(s)", len(violations)),
		Errors: []ValidationError(violations),
	}
}
