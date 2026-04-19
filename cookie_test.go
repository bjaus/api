package api_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/bjaus/api"
)

func TestCookie_IsZero(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		c    api.Cookie
		want bool
	}{
		"zero value":      {api.Cookie{}, true},
		"with value":      {api.Cookie{Value: "x"}, false},
		"with path only":  {api.Cookie{Path: "/"}, false},
		"with httponly":   {api.Cookie{HttpOnly: true}, false},
		"with expires":    {api.Cookie{Expires: time.Unix(1, 0)}, false},
		"with maxage":     {api.Cookie{MaxAge: 60}, false},
		"with samesite":   {api.Cookie{SameSite: http.SameSiteLaxMode}, false},
		"with partitioned": {api.Cookie{Partitioned: true}, false},
		"with quoted":     {api.Cookie{Quoted: true}, false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.c.IsZero())
		})
	}
}

func TestCookie_ToHTTPCookie(t *testing.T) {
	t.Parallel()

	exp := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	c := api.Cookie{
		Value:       "v",
		Path:        "/p",
		Domain:      "example.com",
		Expires:     exp,
		MaxAge:      3600,
		Secure:      true,
		HttpOnly:    true,
		SameSite:    http.SameSiteLaxMode,
		Partitioned: true,
		Quoted:      true,
	}

	out := c.ToHTTPCookie("session")

	assert.Equal(t, "session", out.Name)
	assert.Equal(t, "v", out.Value)
	assert.Equal(t, "/p", out.Path)
	assert.Equal(t, "example.com", out.Domain)
	assert.Equal(t, exp, out.Expires)
	assert.Equal(t, 3600, out.MaxAge)
	assert.True(t, out.Secure)
	assert.True(t, out.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, out.SameSite)
	assert.True(t, out.Partitioned)
	assert.True(t, out.Quoted)
}

func TestCookieFromHTTP(t *testing.T) {
	t.Parallel()

	t.Run("nil input returns zero value", func(t *testing.T) {
		t.Parallel()
		c := api.CookieFromHTTP(nil)
		assert.True(t, c.IsZero())
	})

	t.Run("copies all emission-relevant fields", func(t *testing.T) {
		t.Parallel()
		exp := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
		in := &http.Cookie{
			Name:        "session",
			Value:       "v",
			Path:        "/p",
			Domain:      "example.com",
			Expires:     exp,
			MaxAge:      3600,
			Secure:      true,
			HttpOnly:    true,
			SameSite:    http.SameSiteStrictMode,
			Partitioned: true,
			Quoted:      true,
		}

		out := api.CookieFromHTTP(in)

		assert.Equal(t, "v", out.Value)
		assert.Equal(t, "/p", out.Path)
		assert.Equal(t, "example.com", out.Domain)
		assert.Equal(t, exp, out.Expires)
		assert.Equal(t, 3600, out.MaxAge)
		assert.True(t, out.Secure)
		assert.True(t, out.HttpOnly)
		assert.Equal(t, http.SameSiteStrictMode, out.SameSite)
		assert.True(t, out.Partitioned)
		assert.True(t, out.Quoted)
	})
}

func TestCookie_Roundtrip(t *testing.T) {
	t.Parallel()

	original := api.Cookie{
		Value:    "v",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	stdlib := original.ToHTTPCookie("s")
	back := api.CookieFromHTTP(stdlib)
	assert.Equal(t, original, back, "roundtrip should be lossless for emission fields")
}
