package api

import (
	"context"
	"net/http"
	"time"
)

// Timeout returns middleware that adds a timeout to the request context.
// If the handler does not complete within the duration, a 503 Service
// Unavailable response is sent.
func Timeout(d time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
