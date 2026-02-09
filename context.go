package api

import (
	"context"
	"net/http"
)

type contextKey[T any] struct{}

// SetValue stores a typed value in the request context. For use in middleware.
func SetValue[T any](r *http.Request, val T) *http.Request {
	ctx := context.WithValue(r.Context(), contextKey[T]{}, val)
	return r.WithContext(ctx)
}

// GetValue retrieves a typed value from the request context. For use in handlers.
func GetValue[T any](ctx context.Context) (T, bool) {
	val, ok := ctx.Value(contextKey[T]{}).(T)
	return val, ok
}
