package api

import (
	"net/http"
	"time"
)

// Cookie is the framework's output-cookie type. The cookie's name is carried
// by the struct tag (`cookie:"name"`) on the response field, not by the type
// itself. A zero-value Cookie means "do not emit." Set Value plus any desired
// attributes to emit one Set-Cookie header.
type Cookie struct {
	Value       string
	Path        string
	Domain      string
	Expires     time.Time
	MaxAge      int
	Secure      bool
	HttpOnly    bool //nolint:staticcheck // ST1003: matches net/http.Cookie.HttpOnly for interop
	SameSite    http.SameSite
	Partitioned bool
	Quoted      bool
}

// IsZero reports whether c holds the zero value, indicating no cookie should
// be emitted. A cookie with an empty Value and no attributes set is treated
// as absent.
func (c Cookie) IsZero() bool {
	return c == Cookie{}
}

// ToHTTPCookie produces an *http.Cookie suitable for http.SetCookie or
// attaching to a client request. The name argument becomes the cookie's
// Name field (Cookie does not carry its own name).
func (c Cookie) ToHTTPCookie(name string) *http.Cookie {
	return &http.Cookie{
		Name:        name,
		Value:       c.Value,
		Path:        c.Path,
		Domain:      c.Domain,
		Expires:     c.Expires,
		MaxAge:      c.MaxAge,
		Secure:      c.Secure,
		HttpOnly:    c.HttpOnly,
		SameSite:    c.SameSite,
		Partitioned: c.Partitioned,
		Quoted:      c.Quoted,
	}
}

// CookieFromHTTP wraps a stdlib *http.Cookie as a Cookie, discarding the
// Name (carried by the tag) and the parser-only fields (Raw, RawExpires,
// Unparsed). Returns a zero-value Cookie if c is nil.
func CookieFromHTTP(c *http.Cookie) Cookie {
	if c == nil {
		return Cookie{}
	}
	return Cookie{
		Value:       c.Value,
		Path:        c.Path,
		Domain:      c.Domain,
		Expires:     c.Expires,
		MaxAge:      c.MaxAge,
		Secure:      c.Secure,
		HttpOnly:    c.HttpOnly,
		SameSite:    c.SameSite,
		Partitioned: c.Partitioned,
		Quoted:      c.Quoted,
	}
}
