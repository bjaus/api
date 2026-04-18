package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// SelfValidator is implemented by request types that validate themselves.
type SelfValidator interface {
	Validate() error
}

// Validator validates any request.
type Validator interface {
	Validate(req any) error
}

// Resolver is implemented by request types that need to validate or transform
// themselves after binding, with access to context and the raw request.
type Resolver interface {
	Resolve(ctx context.Context, r *http.Request) []error
}

// resolveRequest calls Resolve on the request if it implements Resolver.
// It converts the returned errors into a ProblemDetail.
func resolveRequest(ctx context.Context, req any, r *http.Request) error {
	res, ok := req.(Resolver)
	if !ok {
		return nil
	}

	errs := res.Resolve(ctx, r)
	if len(errs) == 0 {
		return nil
	}

	var ve []ValidationError
	for _, err := range errs {
		var valErr *ValidationError
		if errors.As(err, &valErr) {
			ve = append(ve, *valErr)
		} else {
			ve = append(ve, ValidationError{Message: err.Error()})
		}
	}

	return &ProblemDetail{
		Type:   "about:blank",
		Title:  "Validation Failed",
		Status: http.StatusBadRequest,
		Detail: fmt.Sprintf("%d validation error(s)", len(ve)),
		Errors: ve,
	}
}
