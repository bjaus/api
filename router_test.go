package api_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjaus/api"
)

func TestRouter_ServeHTTP_basic(t *testing.T) {
	t.Parallel()

	type Resp struct {
		Message string `json:"message"`
	}

	r := api.New()
	api.Get(r, "/health", func(_ context.Context, _ *api.Void) (*Resp, error) {
		return &Resp{Message: "ok"}, nil
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/health", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"message":"ok"`)
}

func TestRouter_Use_middleware(t *testing.T) {
	t.Parallel()

	type Resp struct {
		Value string `json:"value"`
	}

	r := api.New()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Custom", "applied")
			next.ServeHTTP(w, req)
		})
	})

	api.Get(r, "/test", func(_ context.Context, _ *api.Void) (*Resp, error) {
		return &Resp{Value: "hello"}, nil
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/test", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, "applied", resp.Header.Get("X-Custom"))
}

func TestRouter_options(t *testing.T) {
	t.Parallel()

	r := api.New(
		api.WithTitle("Test API"),
		api.WithVersion("1.0.0"),
	)

	spec := r.Spec()
	assert.Equal(t, "Test API", spec.Info.Title)
	assert.Equal(t, "1.0.0", spec.Info.Version)
}

func TestRouter_error_response(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/fail", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return nil, api.Error(http.StatusUnprocessableEntity, "bad data")
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/fail", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Equal(t, "application/problem+json", resp.Header.Get("Content-Type"))

	var body api.ProblemDetail
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, http.StatusUnprocessableEntity, body.Status)
	assert.Equal(t, "bad data", body.Detail)
}
