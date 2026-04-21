package api

import (
	"context"
	"fmt"
)

// Validator is optionally implemented by request types to validate themselves
// after binding. Implementations that don't need the context can ignore it.
//
// Return api.ValidationErrors to have the framework render the failures
// through the configured error body mapper. Any other error type is
// forwarded to the router's error pipeline untouched.
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
// Return api.ValidationErrors to flow through the unified error pipeline;
// any other error is forwarded as-is.
type ValidatorFunc func(req any) error

// ValidationErrors is a collection of field-level validation failures.
// Returning this type (or wrapping it via fmt.Errorf %w) causes the
// framework to convert it to a 422 Unprocessable Content error with each
// violation attached as a detail.
type ValidationErrors []ValidationError

// Error implements the error interface.
func (v ValidationErrors) Error() string {
	return fmt.Sprintf("%d validation error(s)", len(v))
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
