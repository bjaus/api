package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjaus/api"
)

func TestTimeout(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		timeout    time.Duration
		handler    http.HandlerFunc
		wantStatus int
	}{
		"context has deadline set": {
			timeout: 5 * time.Second,
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				deadline, ok := r.Context().Deadline()
				if !ok {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				if time.Until(deadline) <= 0 {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
			}),
			wantStatus: http.StatusOK,
		},
		"normal requests complete": {
			timeout: 5 * time.Second,
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}),
			wantStatus: http.StatusNoContent,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mw := api.Timeout(tc.timeout)
			handler := mw(tc.handler)

			srv := httptest.NewServer(handler)
			t.Cleanup(srv.Close)

			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			assert.Equal(t, tc.wantStatus, resp.StatusCode)
		})
	}
}
