package api

import "net/http"

// BodyLimit returns middleware that limits the maximum request body size.
// If the body exceeds maxBytes, a 413 Payload Too Large response is sent.
func BodyLimit(maxBytes int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
