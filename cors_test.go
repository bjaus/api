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

func TestCORS(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		cfg        []api.CORSConfig
		method     string
		wantStatus int
		wantHeader map[string]string
	}{
		"default headers on GET": {
			method:     http.MethodGet,
			wantStatus: http.StatusOK,
			wantHeader: map[string]string{
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "GET, POST, PUT, PATCH, DELETE, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, Authorization",
				"Vary":                         "Origin",
			},
		},
		"preflight OPTIONS returns 204": {
			method:     http.MethodOptions,
			wantStatus: http.StatusNoContent,
			wantHeader: map[string]string{
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "GET, POST, PUT, PATCH, DELETE, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, Authorization",
				"Vary":                         "Origin",
			},
		},
		"custom config overrides defaults": {
			cfg: []api.CORSConfig{{
				AllowOrigins:  []string{"https://example.com"},
				AllowMethods:  []string{"GET", "POST"},
				AllowHeaders:  []string{"X-Custom"},
				ExposeHeaders: []string{"X-Exposed"},
				MaxAge:        3600,
			}},
			method:     http.MethodGet,
			wantStatus: http.StatusOK,
			wantHeader: map[string]string{
				"Access-Control-Allow-Origin":   "https://example.com",
				"Access-Control-Allow-Methods":  "GET, POST",
				"Access-Control-Allow-Headers":  "X-Custom",
				"Access-Control-Expose-Headers": "X-Exposed",
				"Access-Control-Max-Age":        "3600",
				"Vary":                          "Origin",
			},
		},
		"credentials header when AllowCredentials is true": {
			cfg: []api.CORSConfig{{
				AllowOrigins:     []string{"*"},
				AllowMethods:     []string{"GET"},
				AllowHeaders:     []string{"Content-Type"},
				AllowCredentials: true,
			}},
			method:     http.MethodGet,
			wantStatus: http.StatusOK,
			wantHeader: map[string]string{
				"Access-Control-Allow-Credentials": "true",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mw := api.CORS(tc.cfg...)
			handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			srv := httptest.NewServer(handler)
			t.Cleanup(srv.Close)

			req, err := http.NewRequestWithContext(context.Background(), tc.method, srv.URL+"/", nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			assert.Equal(t, tc.wantStatus, resp.StatusCode)

			for header, want := range tc.wantHeader {
				assert.Equal(t, want, resp.Header.Get(header), "header %s", header)
			}
		})
	}
}
