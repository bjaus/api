package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// CSRFConfig configures the CSRF middleware.
type CSRFConfig struct {
	TokenLength int            // default: 32
	CookieName  string         // default: "_csrf"
	HeaderName  string         // default: "X-CSRF-Token"
	Secure      bool           // cookie secure flag
	SameSite    http.SameSite
}

type csrfTokenKey struct{}

// CSRF returns middleware that implements double-submit cookie CSRF protection.
// Safe methods (GET, HEAD, OPTIONS) are skipped.
func CSRF(cfg ...CSRFConfig) Middleware {
	c := CSRFConfig{
		TokenLength: 32,
		CookieName:  "_csrf",
		HeaderName:  "X-CSRF-Token",
		SameSite:    http.SameSiteLaxMode,
	}
	if len(cfg) > 0 {
		if cfg[0].TokenLength > 0 {
			c.TokenLength = cfg[0].TokenLength
		}
		if cfg[0].CookieName != "" {
			c.CookieName = cfg[0].CookieName
		}
		if cfg[0].HeaderName != "" {
			c.HeaderName = cfg[0].HeaderName
		}
		c.Secure = cfg[0].Secure
		if cfg[0].SameSite != 0 {
			c.SameSite = cfg[0].SameSite
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Read existing token from cookie.
			cookie, err := r.Cookie(c.CookieName)
			token := ""
			if err == nil {
				token = cookie.Value
			}

			// Generate a new token if missing.
			if token == "" {
				token = generateCSRFToken(c.TokenLength)
				http.SetCookie(w, &http.Cookie{
					Name:     c.CookieName,
					Value:    token,
					Path:     "/",
					HttpOnly: true,
					Secure:   c.Secure,
					SameSite: c.SameSite,
				})
			}

			// Store token in context for handlers to read.
			ctx := r.Context()
			r = r.WithContext(setCSRFToken(ctx, token))

			// Safe methods — skip validation.
			if isSafeMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			// Validate token from header matches cookie.
			headerToken := r.Header.Get(c.HeaderName)
			if headerToken == "" || headerToken != token {
				http.Error(w, "CSRF token mismatch", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetCSRFToken retrieves the CSRF token from the request context.
func GetCSRFToken(r *http.Request) string {
	if v, ok := r.Context().Value(csrfTokenKey{}).(string); ok {
		return v
	}
	return ""
}

func setCSRFToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, csrfTokenKey{}, token)
}

func generateCSRFToken(length int) string {
	b := make([]byte, length)
	//nolint:errcheck,gosec // crypto/rand.Read always returns nil error
	rand.Read(b)
	return hex.EncodeToString(b)
}

func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}
