package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

type requestIDKey struct{}

// RequestIDConfig configures the RequestID middleware.
type RequestIDConfig struct {
	Header    string       // default: "X-Request-ID"
	Generator func() string // default: random hex
}

// RequestID returns middleware that assigns a unique request ID to each request.
// The ID is read from the request header (if present) or generated.
// It is stored in the context and set on the response header.
func RequestID(cfg ...RequestIDConfig) Middleware {
	c := RequestIDConfig{
		Header:    "X-Request-ID",
		Generator: defaultIDGenerator,
	}
	if len(cfg) > 0 {
		if cfg[0].Header != "" {
			c.Header = cfg[0].Header
		}
		if cfg[0].Generator != nil {
			c.Generator = cfg[0].Generator
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(c.Header)
			if id == "" {
				id = c.Generator()
			}

			ctx := context.WithValue(r.Context(), requestIDKey{}, id)
			w.Header().Set(c.Header, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetRequestID extracts the request ID from the request context.
func GetRequestID(r *http.Request) string {
	if id, ok := r.Context().Value(requestIDKey{}).(string); ok {
		return id
	}
	return ""
}

func defaultIDGenerator() string {
	b := make([]byte, 16)
	//nolint:errcheck,gosec // crypto/rand.Read always returns nil error
	rand.Read(b)
	return hex.EncodeToString(b)
}
