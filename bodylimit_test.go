package api_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjaus/api"
)

func TestBodyLimit(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		maxBytes   int64
		bodySize   int
		wantStatus int
	}{
		"request within limit succeeds": {
			maxBytes:   1024,
			bodySize:   512,
			wantStatus: http.StatusOK,
		},
		"request exceeding limit fails": {
			maxBytes:   64,
			bodySize:   128,
			wantStatus: http.StatusRequestEntityTooLarge,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mw := api.BodyLimit(tc.maxBytes)
			handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))

			srv := httptest.NewServer(handler)
			t.Cleanup(srv.Close)

			body := bytes.Repeat([]byte("x"), tc.bodySize)
			req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/", bytes.NewReader(body))
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			assert.Equal(t, tc.wantStatus, resp.StatusCode)
		})
	}
}
