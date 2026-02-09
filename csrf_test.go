package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjaus/api"
)

func TestCSRF_safe_methods_pass_without_token(t *testing.T) {
	t.Parallel()

	methods := map[string]struct{}{
		http.MethodGet:     {},
		http.MethodHead:    {},
		http.MethodOptions: {},
	}

	for method := range methods {
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			handler := api.CSRF()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			srv := httptest.NewServer(handler)
			t.Cleanup(srv.Close)

			req, err := http.NewRequestWithContext(context.Background(), method, srv.URL+"/", nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
}

func TestCSRF_unsafe_methods_require_matching_token(t *testing.T) {
	t.Parallel()

	handler := api.CSRF()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	tests := map[string]struct {
		method     string
		withCookie bool
		withHeader bool
		matchToken bool
		wantStatus int
	}{
		"POST without token returns 403": {
			method:     http.MethodPost,
			withCookie: false,
			withHeader: false,
			wantStatus: http.StatusForbidden,
		},
		"POST with cookie but no header returns 403": {
			method:     http.MethodPost,
			withCookie: true,
			withHeader: false,
			matchToken: false,
			wantStatus: http.StatusForbidden,
		},
		"POST with matching cookie and header returns 200": {
			method:     http.MethodPost,
			withCookie: true,
			withHeader: true,
			matchToken: true,
			wantStatus: http.StatusOK,
		},
		"POST with mismatched tokens returns 403": {
			method:     http.MethodPost,
			withCookie: true,
			withHeader: true,
			matchToken: false,
			wantStatus: http.StatusForbidden,
		},
		"PUT without token returns 403": {
			method:     http.MethodPut,
			withCookie: false,
			withHeader: false,
			wantStatus: http.StatusForbidden,
		},
		"DELETE without token returns 403": {
			method:     http.MethodDelete,
			withCookie: false,
			withHeader: false,
			wantStatus: http.StatusForbidden,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(context.Background(), tc.method, srv.URL+"/", nil)
			require.NoError(t, err)

			token := "test-csrf-token-value" //nolint:gosec // test value, not a credential
			if tc.withCookie {
				req.AddCookie(&http.Cookie{
					Name:  "_csrf",
					Value: token,
				})
			}
			if tc.withHeader {
				if tc.matchToken {
					req.Header.Set("X-CSRF-Token", token)
				} else {
					req.Header.Set("X-CSRF-Token", "wrong-token")
				}
			}

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			assert.Equal(t, tc.wantStatus, resp.StatusCode)
		})
	}
}

func TestCSRF_sets_cookie_on_first_request(t *testing.T) {
	t.Parallel()

	handler := api.CSRF()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var found bool
	for _, c := range resp.Cookies() {
		if c.Name == "_csrf" {
			found = true
			assert.NotEmpty(t, c.Value)
			assert.True(t, c.HttpOnly)
			break
		}
	}
	assert.True(t, found, "CSRF cookie should be set on first GET request")
}

func TestGetCSRFToken_returns_token_from_context(t *testing.T) {
	t.Parallel()

	var captured string

	handler := api.CSRF()(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = api.GetCSRFToken(r)
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.NotEmpty(t, captured)
}

func TestGetCSRFToken_returns_empty_without_middleware(t *testing.T) {
	t.Parallel()

	r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	require.NoError(t, err)

	token := api.GetCSRFToken(r)
	assert.Empty(t, token)
}

func TestCSRF_custom_config(t *testing.T) {
	t.Parallel()

	cfg := api.CSRFConfig{
		CookieName: "my_csrf",
		HeaderName: "X-My-Token",
	}

	var captured string

	handler := api.CSRF(cfg)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = api.GetCSRFToken(r)
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	// GET request to get the cookie.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.NotEmpty(t, captured)

	var cookieFound bool
	for _, c := range resp.Cookies() {
		if c.Name == "my_csrf" {
			cookieFound = true
			assert.Equal(t, captured, c.Value)
			break
		}
	}
	assert.True(t, cookieFound, "custom cookie name should be used")

	// POST with the custom header and cookie.
	postReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/", nil)
	require.NoError(t, err)
	postReq.AddCookie(&http.Cookie{Name: "my_csrf", Value: captured})
	postReq.Header.Set("X-My-Token", captured)

	postResp, err := http.DefaultClient.Do(postReq)
	require.NoError(t, err)
	defer func() { require.NoError(t, postResp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, postResp.StatusCode)
}

func TestCSRF_custom_samesite_and_secure(t *testing.T) {
	t.Parallel()

	cfg := api.CSRFConfig{
		TokenLength: 16,
		Secure:      true,
		SameSite:    http.SameSiteStrictMode,
	}

	handler := api.CSRF(cfg)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var csrfCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "_csrf" {
			csrfCookie = c
			break
		}
	}
	require.NotNil(t, csrfCookie, "CSRF cookie should be set")
	assert.NotEmpty(t, csrfCookie.Value)
}
