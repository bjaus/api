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

func TestETag_sets_etag_header(t *testing.T) {
	t.Parallel()

	handler := api.ETag()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck,gosec
		w.Write([]byte(`{"ok":true}`))
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	etag := resp.Header.Get("ETag")
	assert.NotEmpty(t, etag)
	assert.True(t, etag[0] == '"', "ETag should start with a quote")
}

func TestETag_if_none_match_returns_304(t *testing.T) {
	t.Parallel()

	body := []byte(`{"ok":true}`)
	handler := api.ETag()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck,gosec
		w.Write(body)
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	// First request to get the ETag.
	req1, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
	require.NoError(t, err)

	resp1, err := http.DefaultClient.Do(req1)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp1.Body.Close()) }()

	etag := resp1.Header.Get("ETag")
	require.NotEmpty(t, etag)

	// Second request with If-None-Match.
	req2, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
	require.NoError(t, err)
	req2.Header.Set("If-None-Match", etag)

	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp2.Body.Close()) }()

	assert.Equal(t, http.StatusNotModified, resp2.StatusCode)
}

func TestETag_weak_etag(t *testing.T) {
	t.Parallel()

	handler := api.ETag(api.ETagConfig{Weak: true})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck,gosec
		w.Write([]byte(`{"data":"test"}`))
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	etag := resp.Header.Get("ETag")
	assert.NotEmpty(t, etag)
	assert.Contains(t, etag, "W/", "weak ETag should start with W/")
}

func TestETag_post_request_bypasses_etag(t *testing.T) {
	t.Parallel()

	handler := api.ETag()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		//nolint:errcheck,gosec
		w.Write([]byte(`{"id":"1"}`))
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Empty(t, resp.Header.Get("ETag"), "POST should not have ETag")
}

func TestETag_weak_if_none_match_returns_304(t *testing.T) {
	t.Parallel()

	body := []byte(`{"ok":true}`)
	handler := api.ETag(api.ETagConfig{Weak: true})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck,gosec
		w.Write(body)
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	// First request to get the weak ETag.
	req1, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
	require.NoError(t, err)

	resp1, err := http.DefaultClient.Do(req1)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp1.Body.Close()) }()

	etag := resp1.Header.Get("ETag")
	require.NotEmpty(t, etag)
	require.Contains(t, etag, "W/")

	// Second request with If-None-Match using the weak ETag.
	req2, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
	require.NoError(t, err)
	req2.Header.Set("If-None-Match", etag)

	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp2.Body.Close()) }()

	assert.Equal(t, http.StatusNotModified, resp2.StatusCode)
}

func TestETag_non_2xx_response_passes_through(t *testing.T) {
	t.Parallel()

	handler := api.ETag()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		//nolint:errcheck,gosec
		w.Write([]byte(`{"error":"not found"}`))
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Empty(t, resp.Header.Get("ETag"), "non-2xx should not have ETag")
}

func TestETag_if_match_matching(t *testing.T) {
	t.Parallel()

	body := []byte(`{"ok":true}`)
	handler := api.ETag()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck,gosec
		w.Write(body)
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	// First request to get the ETag.
	req1, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
	require.NoError(t, err)

	resp1, err := http.DefaultClient.Do(req1)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp1.Body.Close()) }()

	etag := resp1.Header.Get("ETag")
	require.NotEmpty(t, etag)

	// Second request with If-Match header (matching).
	req2, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
	require.NoError(t, err)
	req2.Header.Set("If-Match", etag)

	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp2.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp2.StatusCode)
}

func TestETag_if_match_non_matching(t *testing.T) {
	t.Parallel()

	body := []byte(`{"ok":true}`)
	handler := api.ETag()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck,gosec
		w.Write(body)
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
	require.NoError(t, err)
	req.Header.Set("If-Match", `"wrong-etag"`)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusPreconditionFailed, resp.StatusCode)
}

func TestETag_if_match_star(t *testing.T) {
	t.Parallel()

	body := []byte(`{"ok":true}`)
	handler := api.ETag()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck,gosec
		w.Write(body)
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
	require.NoError(t, err)
	req.Header.Set("If-Match", "*")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestETag_head_request(t *testing.T) {
	t.Parallel()

	handler := api.ETag()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck,gosec
		w.Write([]byte(`{"ok":true}`))
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodHead, srv.URL+"/", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get("ETag"), "HEAD should still compute ETag")
}
