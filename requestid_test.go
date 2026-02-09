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

func TestRequestID(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		cfg       []api.RequestIDConfig
		reqHeader map[string]string
		checkID   func(t *testing.T, resp *http.Response)
	}{
		"generates X-Request-ID when none provided": {
			checkID: func(t *testing.T, resp *http.Response) {
				t.Helper()
				id := resp.Header.Get("X-Request-ID")
				assert.NotEmpty(t, id)
				assert.Len(t, id, 32) // 16 bytes hex-encoded
			},
		},
		"preserves existing X-Request-ID": {
			reqHeader: map[string]string{
				"X-Request-ID": "my-custom-id-123",
			},
			checkID: func(t *testing.T, resp *http.Response) {
				t.Helper()
				assert.Equal(t, "my-custom-id-123", resp.Header.Get("X-Request-ID"))
			},
		},
		"custom header name": {
			cfg: []api.RequestIDConfig{{
				Header: "X-Trace-ID",
			}},
			checkID: func(t *testing.T, resp *http.Response) {
				t.Helper()
				id := resp.Header.Get("X-Trace-ID")
				assert.NotEmpty(t, id)
				assert.Len(t, id, 32)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mw := api.RequestID(tc.cfg...)
			handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			srv := httptest.NewServer(handler)
			t.Cleanup(srv.Close)

			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
			require.NoError(t, err)

			for k, v := range tc.reqHeader {
				req.Header.Set(k, v)
			}

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			tc.checkID(t, resp)
		})
	}
}

func TestGetRequestID(t *testing.T) {
	t.Parallel()

	var captured string

	mw := api.RequestID()
	handler := mw(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = api.GetRequestID(r)
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
	require.NoError(t, err)

	req.Header.Set("X-Request-ID", "ctx-test-id")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, "ctx-test-id", captured)
}

func TestRequestID_custom_generator(t *testing.T) {
	t.Parallel()

	mw := api.RequestID(api.RequestIDConfig{
		Generator: func() string { return "fixed-id-42" },
	})
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

	assert.Equal(t, "fixed-id-42", resp.Header.Get("X-Request-ID"))
}
