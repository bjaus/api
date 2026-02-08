package api

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Middleware is the standard middleware signature compatible with the entire
// Go middleware ecosystem.
type Middleware func(next http.Handler) http.Handler

// Recovery returns middleware that recovers from panics and responds with 500.
func Recovery() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					slog.Error("panic recovered",
						"panic", rec,
						"stack", string(debug.Stack()),
						"method", r.Method,
						"path", r.URL.Path,
					)
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
