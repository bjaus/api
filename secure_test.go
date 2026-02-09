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

func TestSecure(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		cfg        []api.SecureConfig
		wantHeader map[string]string
		noHeader   []string
	}{
		"default security headers are set": {
			wantHeader: map[string]string{
				"X-Content-Type-Options": "nosniff",
				"X-Frame-Options":        "DENY",
				"X-XSS-Protection":       "1; mode=block",
				"Referrer-Policy":        "strict-origin-when-cross-origin",
			},
			noHeader: []string{"Strict-Transport-Security"},
		},
		"HSTS header when configured": {
			cfg: []api.SecureConfig{{
				ContentTypeNosniff: true,
				FrameDeny:          true,
				HSTSMaxAge:         31536000,
				XSSProtection:      "1; mode=block",
				ReferrerPolicy:     "no-referrer",
			}},
			wantHeader: map[string]string{
				"Strict-Transport-Security": "max-age=31536000",
				"X-Content-Type-Options":    "nosniff",
				"X-Frame-Options":           "DENY",
				"X-XSS-Protection":          "1; mode=block",
				"Referrer-Policy":           "no-referrer",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mw := api.Secure(tc.cfg...)
			handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

			for header, want := range tc.wantHeader {
				assert.Equal(t, want, resp.Header.Get(header), "header %s", header)
			}

			for _, header := range tc.noHeader {
				assert.Empty(t, resp.Header.Get(header), "header %s should not be set", header)
			}
		})
	}
}
