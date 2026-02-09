package api

import (
	"net/http"
	"strconv"
	"strings"
)

// CORSConfig configures the CORS middleware.
type CORSConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           int // seconds
}

// CORS returns middleware that handles Cross-Origin Resource Sharing.
// If no config is provided, permissive defaults are used.
func CORS(cfg ...CORSConfig) Middleware {
	c := CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Content-Type", "Authorization"},
	}
	if len(cfg) > 0 {
		c = cfg[0]
	}

	origins := strings.Join(c.AllowOrigins, ", ")
	methods := strings.Join(c.AllowMethods, ", ")
	headers := strings.Join(c.AllowHeaders, ", ")
	expose := strings.Join(c.ExposeHeaders, ", ")
	maxAge := ""
	if c.MaxAge > 0 {
		maxAge = strconv.Itoa(c.MaxAge)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", origins)
			w.Header().Set("Access-Control-Allow-Methods", methods)
			w.Header().Set("Access-Control-Allow-Headers", headers)

			if expose != "" {
				w.Header().Set("Access-Control-Expose-Headers", expose)
			}
			if c.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			if maxAge != "" {
				w.Header().Set("Access-Control-Max-Age", maxAge)
			}

			w.Header().Set("Vary", "Origin")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
