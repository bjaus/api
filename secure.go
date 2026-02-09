package api

import (
	"net/http"
	"strconv"
)

// SecureConfig configures the Secure headers middleware.
type SecureConfig struct {
	ContentTypeNosniff bool   // default: true → X-Content-Type-Options: nosniff
	FrameDeny          bool   // default: true → X-Frame-Options: DENY
	HSTSMaxAge         int    // default: 0 (disabled). If >0: Strict-Transport-Security
	XSSProtection      string // default: "1; mode=block"
	ReferrerPolicy     string // default: "strict-origin-when-cross-origin"
}

// Secure returns middleware that sets security response headers.
// With no arguments, it uses sensible defaults.
func Secure(cfg ...SecureConfig) Middleware {
	c := SecureConfig{
		ContentTypeNosniff: true,
		FrameDeny:          true,
		XSSProtection:      "1; mode=block",
		ReferrerPolicy:     "strict-origin-when-cross-origin",
	}
	if len(cfg) > 0 {
		c = cfg[0]
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if c.ContentTypeNosniff {
				w.Header().Set("X-Content-Type-Options", "nosniff")
			}
			if c.FrameDeny {
				w.Header().Set("X-Frame-Options", "DENY")
			}
			if c.HSTSMaxAge > 0 {
				w.Header().Set("Strict-Transport-Security", "max-age="+strconv.Itoa(c.HSTSMaxAge))
			}
			if c.XSSProtection != "" {
				w.Header().Set("X-XSS-Protection", c.XSSProtection)
			}
			if c.ReferrerPolicy != "" {
				w.Header().Set("Referrer-Policy", c.ReferrerPolicy)
			}

			next.ServeHTTP(w, r)
		})
	}
}
