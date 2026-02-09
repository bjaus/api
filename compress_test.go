package api_test

import (
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjaus/api"
)

func TestCompress_no_accept_encoding(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("hello world ", 200) // >1024 bytes
	handler := api.Compress()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body)) //nolint:errcheck
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/test", nil)
	require.NoError(t, err)
	// Explicitly do NOT set Accept-Encoding
	req.Header.Del("Accept-Encoding")

	transport := &http.Transport{
		DisableCompression: true,
	}
	client := &http.Client{Transport: transport}

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Empty(t, resp.Header.Get("Content-Encoding"))
	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, body, string(got))
}

func TestCompress_gzip_large_json(t *testing.T) {
	t.Parallel()

	body := strings.Repeat(`{"key":"value"},`, 200) // >1024 bytes
	handler := api.Compress()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body)) //nolint:errcheck
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/test", nil)
	require.NoError(t, err)
	req.Header.Set("Accept-Encoding", "gzip")

	transport := &http.Transport{
		DisableCompression: true,
	}
	client := &http.Client{Transport: transport}

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, "gzip", resp.Header.Get("Content-Encoding"))
	assert.Equal(t, "Accept-Encoding", resp.Header.Get("Vary"))

	gz, err := gzip.NewReader(resp.Body)
	require.NoError(t, err)
	defer func() { require.NoError(t, gz.Close()) }()

	got, err := io.ReadAll(gz)
	require.NoError(t, err)
	assert.Equal(t, body, string(got))
}

func TestCompress_small_response_not_compressed(t *testing.T) {
	t.Parallel()

	body := `{"ok":true}` // <1024 bytes
	handler := api.Compress()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body)) //nolint:errcheck
	}))

	rec := httptest.NewRecorder()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
	require.NoError(t, err)
	req.Header.Set("Accept-Encoding", "gzip")

	handler.ServeHTTP(rec, req)

	// Should NOT be gzip since body < MinSize
	assert.NotEqual(t, "gzip", rec.Header().Get("Content-Encoding"))
	assert.Contains(t, rec.Body.String(), body)
}

func TestCompress_sse_not_compressed(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("data: hello\n\n", 200) // >1024 bytes
	handler := api.Compress()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(body)) //nolint:errcheck
	}))

	rec := httptest.NewRecorder()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
	require.NoError(t, err)
	req.Header.Set("Accept-Encoding", "gzip")

	handler.ServeHTTP(rec, req)

	// SSE should NOT be compressed even though "text/" matches the type list
	assert.NotEqual(t, "gzip", rec.Header().Get("Content-Encoding"))
	assert.Contains(t, rec.Body.String(), body)
}

func TestCompress_custom_config(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("x", 100) // 100 bytes, we set MinSize to 50
	handler := api.Compress(api.CompressConfig{
		Level:   9,
		MinSize: 50,
		Types:   []string{"text/plain"},
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(body)) //nolint:errcheck
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/test", nil)
	require.NoError(t, err)
	req.Header.Set("Accept-Encoding", "gzip")

	transport := &http.Transport{
		DisableCompression: true,
	}
	client := &http.Client{Transport: transport}

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, "gzip", resp.Header.Get("Content-Encoding"))

	gz, err := gzip.NewReader(resp.Body)
	require.NoError(t, err)
	defer func() { require.NoError(t, gz.Close()) }()

	got, err := io.ReadAll(gz)
	require.NoError(t, err)
	assert.Equal(t, body, string(got))
}

func TestCompress_already_encoded_not_double_compressed(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("compressed data ", 200) // >1024 bytes
	handler := api.Compress()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "br")
		_, _ = w.Write([]byte(body)) //nolint:errcheck
	}))

	rec := httptest.NewRecorder()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
	require.NoError(t, err)
	req.Header.Set("Accept-Encoding", "gzip")

	handler.ServeHTTP(rec, req)

	// Should NOT have gzip Content-Encoding since the response was already encoded
	assert.NotEqual(t, "gzip", rec.Header().Get("Content-Encoding"))
	assert.Contains(t, rec.Body.String(), body)
}

func TestCompress_unwrap(t *testing.T) {
	t.Parallel()

	handler := api.Compress()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Test that http.ResponseController can unwrap through the gzipResponseWriter.
		// The Unwrap method is called internally by http.ResponseController.
		rc := http.NewResponseController(w)
		// Flush exercises the Unwrap path — if gzipResponseWriter doesn't implement Unwrap,
		// ResponseController cannot reach the underlying ResponseWriter's Flush.
		_ = rc.Flush() //nolint:errcheck
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`)) //nolint:errcheck
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/test", nil)
	require.NoError(t, err)
	req.Header.Set("Accept-Encoding", "gzip")

	transport := &http.Transport{
		DisableCompression: true,
	}
	client := &http.Client{Transport: transport}

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestCompress_non_matching_content_type(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("binary data here ", 200) // >1024 bytes
	handler := api.Compress()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte(body)) //nolint:errcheck
	}))

	rec := httptest.NewRecorder()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
	require.NoError(t, err)
	req.Header.Set("Accept-Encoding", "gzip")

	handler.ServeHTTP(rec, req)

	// image/png is NOT in the default types list, so gzipActive should be false
	// and Content-Encoding should not be "gzip" in the final response header
	assert.NotEqual(t, "gzip", rec.Header().Get("Content-Encoding"))
	assert.Contains(t, rec.Body.String(), body)
}

func TestCompress_text_content_type_compressed(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("hello html content ", 200) // >1024 bytes
	handler := api.Compress()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(body)) //nolint:errcheck
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/test", nil)
	require.NoError(t, err)
	req.Header.Set("Accept-Encoding", "gzip")

	transport := &http.Transport{
		DisableCompression: true,
	}
	client := &http.Client{Transport: transport}

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, "gzip", resp.Header.Get("Content-Encoding"))

	gz, err := gzip.NewReader(resp.Body)
	require.NoError(t, err)
	defer func() { require.NoError(t, gz.Close()) }()

	got, err := io.ReadAll(gz)
	require.NoError(t, err)
	assert.Equal(t, body, string(got))
}

func TestCompress_multiple_writes(t *testing.T) {
	t.Parallel()

	chunk := strings.Repeat("a", 600)
	handler := api.Compress()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// First write is large enough (>=1024)
		_, _ = w.Write([]byte(chunk + chunk)) //nolint:errcheck
		// Second write should also go through gzip
		_, _ = w.Write([]byte(chunk)) //nolint:errcheck
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/test", nil)
	require.NoError(t, err)
	req.Header.Set("Accept-Encoding", "gzip")

	transport := &http.Transport{
		DisableCompression: true,
	}
	client := &http.Client{Transport: transport}

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, "gzip", resp.Header.Get("Content-Encoding"))

	gz, err := gzip.NewReader(resp.Body)
	require.NoError(t, err)
	defer func() { require.NoError(t, gz.Close()) }()

	got, err := io.ReadAll(gz)
	require.NoError(t, err)
	assert.Equal(t, chunk+chunk+chunk, string(got))
}
