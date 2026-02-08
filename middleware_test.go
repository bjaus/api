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

func TestRecovery(t *testing.T) {
	t.Parallel()

	r := api.New()
	r.Use(api.Recovery())

	api.Get(r, "/panic", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		panic("boom")
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/panic", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestMiddleware_ordering(t *testing.T) {
	t.Parallel()

	type Resp struct {
		Value string `json:"value"`
	}

	r := api.New()

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-First", "1")
			next.ServeHTTP(w, req)
		})
	})

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Second", "2")
			next.ServeHTTP(w, req)
		})
	})

	api.Get(r, "/test", func(_ context.Context, _ *api.Void) (*Resp, error) {
		return &Resp{Value: "ok"}, nil
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/test", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, "1", resp.Header.Get("X-First"))
	assert.Equal(t, "2", resp.Header.Get("X-Second"))
}
