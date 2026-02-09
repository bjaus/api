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

func TestTrailingSlash_strips_trailing_slash(t *testing.T) {
	t.Parallel()

	handler := api.TrailingSlash()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	tests := map[string]struct {
		path           string
		wantRedirect   bool
		wantLocation   string
		wantStatusCode int
	}{
		"trailing slash redirects": {
			path:           "/users/",
			wantRedirect:   true,
			wantLocation:   "/users",
			wantStatusCode: http.StatusMovedPermanently,
		},
		"no trailing slash passes through": {
			path:           "/users",
			wantRedirect:   false,
			wantStatusCode: http.StatusOK,
		},
		"root path is not stripped": {
			path:           "/",
			wantRedirect:   false,
			wantStatusCode: http.StatusOK,
		},
		"nested trailing slash redirects": {
			path:           "/api/v1/users/",
			wantRedirect:   true,
			wantLocation:   "/api/v1/users",
			wantStatusCode: http.StatusMovedPermanently,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+tc.path, nil)
			require.NoError(t, err)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			assert.Equal(t, tc.wantStatusCode, resp.StatusCode)
			if tc.wantRedirect {
				assert.Equal(t, tc.wantLocation, resp.Header.Get("Location"))
			}
		})
	}
}

func TestTrailingSlash_preserves_query_string(t *testing.T) {
	t.Parallel()

	handler := api.TrailingSlash()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/users/?page=2&limit=10", nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusMovedPermanently, resp.StatusCode)
	assert.Equal(t, "/users?page=2&limit=10", resp.Header.Get("Location"))
}

func TestNonWWWRedirect_strips_www(t *testing.T) {
	t.Parallel()

	handler := api.NonWWWRedirect()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := map[string]struct {
		host         string
		wantRedirect bool
	}{
		"www host redirects": {
			host:         "www.example.com",
			wantRedirect: true,
		},
		"non-www host passes through": {
			host:         "example.com",
			wantRedirect: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+tc.host+"/path", nil)
			require.NoError(t, err)
			req.Host = tc.host

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if tc.wantRedirect {
				assert.Equal(t, http.StatusMovedPermanently, rec.Code)
				location := rec.Header().Get("Location")
				assert.Contains(t, location, "example.com")
				assert.NotContains(t, location, "www.")
			} else {
				assert.Equal(t, http.StatusOK, rec.Code)
			}
		})
	}
}

func TestHTTPSRedirect_redirects_http_to_https(t *testing.T) {
	t.Parallel()

	handler := api.HTTPSRedirect()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := map[string]struct {
		forwardedProto string
		wantRedirect   bool
	}{
		"plain HTTP redirects to HTTPS": {
			forwardedProto: "",
			wantRedirect:   true,
		},
		"X-Forwarded-Proto https passes through": {
			forwardedProto: "https",
			wantRedirect:   false,
		},
		"X-Forwarded-Proto http redirects": {
			forwardedProto: "http",
			wantRedirect:   true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/api/v1/users?page=1", nil)
			require.NoError(t, err)
			req.Host = "example.com"
			if tc.forwardedProto != "" {
				req.Header.Set("X-Forwarded-Proto", tc.forwardedProto)
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if tc.wantRedirect {
				assert.Equal(t, http.StatusMovedPermanently, rec.Code)
				location := rec.Header().Get("Location")
				assert.Equal(t, "https://example.com/api/v1/users?page=1", location)
			} else {
				assert.Equal(t, http.StatusOK, rec.Code)
			}
		})
	}
}
