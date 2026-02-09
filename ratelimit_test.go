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

func TestRateLimit(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		rate       float64
		burst      int
		numReqs    int
		wantOK     int
		wantLimited int
	}{
		"requests within rate succeed": {
			rate:        100,
			burst:       10,
			numReqs:     5,
			wantOK:      5,
			wantLimited: 0,
		},
		"requests exceeding rate get 429": {
			rate:        1,
			burst:       1,
			numReqs:     5,
			wantOK:      1,
			wantLimited: 4,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mw := api.RateLimit(api.RateLimitConfig{
				Rate:  tc.rate,
				Burst: tc.burst,
			})
			handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			srv := httptest.NewServer(handler)
			t.Cleanup(srv.Close)

			okCount := 0
			limitedCount := 0

			for range tc.numReqs {
				req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
				require.NoError(t, err)

				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)

				switch resp.StatusCode {
				case http.StatusOK:
					okCount++
				case http.StatusTooManyRequests:
					limitedCount++
					assert.NotEmpty(t, resp.Header.Get("Retry-After"), "Retry-After header should be set")
				}

				require.NoError(t, resp.Body.Close())
			}

			assert.Equal(t, tc.wantOK, okCount, "expected OK responses")
			assert.Equal(t, tc.wantLimited, limitedCount, "expected rate-limited responses")
		})
	}
}

func TestRateLimit_custom_key_func(t *testing.T) {
	t.Parallel()

	mw := api.RateLimit(api.RateLimitConfig{
		Rate:  1,
		Burst: 1,
		KeyFunc: func(r *http.Request) string {
			return r.Header.Get("X-User-ID")
		},
	})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	// User A makes 2 requests - second should be limited.
	reqA1, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
	require.NoError(t, err)
	reqA1.Header.Set("X-User-ID", "user-a")
	respA1, err := http.DefaultClient.Do(reqA1)
	require.NoError(t, err)
	require.NoError(t, respA1.Body.Close())
	assert.Equal(t, http.StatusOK, respA1.StatusCode)

	reqA2, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
	require.NoError(t, err)
	reqA2.Header.Set("X-User-ID", "user-a")
	respA2, err := http.DefaultClient.Do(reqA2)
	require.NoError(t, err)
	require.NoError(t, respA2.Body.Close())
	assert.Equal(t, http.StatusTooManyRequests, respA2.StatusCode)

	// User B should still get through because different key.
	reqB, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
	require.NoError(t, err)
	reqB.Header.Set("X-User-ID", "user-b")
	respB, err := http.DefaultClient.Do(reqB)
	require.NoError(t, err)
	require.NoError(t, respB.Body.Close())
	assert.Equal(t, http.StatusOK, respB.StatusCode)
}

func TestRateLimit_default_keyfunc_splithost_error(t *testing.T) {
	t.Parallel()

	mw := api.RateLimit(api.RateLimitConfig{
		Rate:  100,
		Burst: 10,
	})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Call handler directly with a RemoteAddr that has no port (causes SplitHostPort error).
	rec := httptest.NewRecorder()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	require.NoError(t, err)
	req.RemoteAddr = "10.0.0.1" // no port

	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRateLimit_cleanup_expired_limiters(t *testing.T) {
	t.Parallel()

	mw := api.RateLimit(api.RateLimitConfig{
		Rate:            100,
		Burst:           100,
		CleanupInterval: time.Millisecond,
		MaxIdle:         time.Millisecond,
	})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	// First request creates a limiter entry.
	req1, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
	require.NoError(t, err)
	resp1, err := http.DefaultClient.Do(req1)
	require.NoError(t, err)
	require.NoError(t, resp1.Body.Close())
	assert.Equal(t, http.StatusOK, resp1.StatusCode)

	// Wait for entries to become idle.
	time.Sleep(5 * time.Millisecond)

	// Second request triggers cleanup of the expired entry and creates a new one.
	req2, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
	require.NoError(t, err)
	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	require.NoError(t, resp2.Body.Close())
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
}

func TestRateLimit_custom_on_limit(t *testing.T) {
	t.Parallel()

	mw := api.RateLimit(api.RateLimitConfig{
		Rate:  1,
		Burst: 1,
		OnLimit: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			//nolint:errcheck,gosec
			w.Write([]byte(`{"error":"custom limit"}`))
		},
	})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	// First request should pass.
	req1, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
	require.NoError(t, err)
	resp1, err := http.DefaultClient.Do(req1)
	require.NoError(t, err)
	require.NoError(t, resp1.Body.Close())
	assert.Equal(t, http.StatusOK, resp1.StatusCode)

	// Second request should trigger the custom OnLimit handler.
	req2, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
	require.NoError(t, err)
	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp2.Body.Close()) }()

	assert.Equal(t, http.StatusServiceUnavailable, resp2.StatusCode)
	assert.Equal(t, "application/json", resp2.Header.Get("Content-Type"))
}
