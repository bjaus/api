package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjaus/api"
)

func TestGroup_prefix(t *testing.T) {
	t.Parallel()

	type Resp struct {
		Version string `json:"version"`
	}

	r := api.New()
	v1 := r.Group("/v1")

	api.Get(v1, "/health", func(_ context.Context, _ *api.Void) (*Resp, error) {
		return &Resp{Version: "v1"}, nil
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/v1/health", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body Resp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "v1", body.Version)
}

func TestGroup_middleware(t *testing.T) {
	t.Parallel()

	type Resp struct {
		OK bool `json:"ok"`
	}

	r := api.New()

	authed := r.Group("/admin", api.WithGroupMiddleware(
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("X-Group-MW", "yes")
				next.ServeHTTP(w, req)
			})
		},
	))

	api.Get(authed, "/dashboard", func(_ context.Context, _ *api.Void) (*Resp, error) {
		return &Resp{OK: true}, nil
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/admin/dashboard", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "yes", resp.Header.Get("X-Group-MW"))
}

func TestGroup_tags_in_spec(t *testing.T) {
	t.Parallel()

	r := api.New(api.WithTitle("Test"))
	v1 := r.Group("/v1", api.WithGroupTags("v1"))

	api.Get(v1, "/items", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	})

	spec := r.Spec()
	ops, ok := spec.Paths["/v1/items"]
	require.True(t, ok, "path /v1/items should exist")
	assert.Contains(t, ops["get"].Tags, "v1")
}
