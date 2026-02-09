package api_test

import (
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

func TestStream_response(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/download", func(_ context.Context, _ *api.Void) (*api.Stream, error) {
		return &api.Stream{
			ContentType: "text/plain",
			Status:      http.StatusOK,
			Body:        strings.NewReader("file contents here"),
		}, nil
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/download", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/plain", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "file contents here", string(body))
}

func TestSSEStream_response(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/events", func(_ context.Context, _ *api.Void) (*api.SSEStream, error) {
		ch := make(chan api.SSEEvent, 3)
		ch <- api.SSEEvent{Event: "message", Data: "hello", ID: "1"}
		ch <- api.SSEEvent{Event: "message", Data: map[string]string{"key": "value"}, ID: "2"}
		ch <- api.SSEEvent{Data: "plain"}
		close(ch)

		return &api.SSEStream{Events: ch}, nil
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/events", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	content := string(body)
	assert.Contains(t, content, "id: 1")
	assert.Contains(t, content, "event: message")
	assert.Contains(t, content, "data: hello")
	assert.Contains(t, content, `data: {"key":"value"}`)
	assert.Contains(t, content, "data: plain")
}

func TestStream_status_zero_defaults_to_200(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/stream-default", func(_ context.Context, _ *api.Void) (*api.Stream, error) {
		return &api.Stream{
			ContentType: "text/plain",
			Status:      0,
			Body:        strings.NewReader("default status"),
		}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/stream-default", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "default status", string(body))
}

func TestStream_nil_body(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/stream-nil", func(_ context.Context, _ *api.Void) (*api.Stream, error) {
		return &api.Stream{
			ContentType: "text/plain",
			Status:      http.StatusNoContent,
			Body:        nil,
		}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/stream-nil", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestStream_custom_status(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/stream-202", func(_ context.Context, _ *api.Void) (*api.Stream, error) {
		return &api.Stream{
			ContentType: "application/octet-stream",
			Status:      http.StatusAccepted,
			Body:        strings.NewReader("accepted"),
		}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/stream-202", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
}

func TestSSEEvent_with_byte_data(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/sse-bytes", func(_ context.Context, _ *api.Void) (*api.SSEStream, error) {
		ch := make(chan api.SSEEvent, 1)
		ch <- api.SSEEvent{Data: []byte("raw bytes")}
		close(ch)
		return &api.SSEStream{Events: ch}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/sse-bytes", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "data: raw bytes")
}

func TestSSEEvent_with_unmarshalable_data(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/sse-bad", func(_ context.Context, _ *api.Void) (*api.SSEStream, error) {
		ch := make(chan api.SSEEvent, 1)
		// channels cannot be marshaled to JSON.
		ch <- api.SSEEvent{Data: make(chan int)}
		close(ch)
		return &api.SSEStream{Events: ch}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/sse-bad", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	// The error message from json.Marshal should be used as the data.
	assert.Contains(t, string(body), "data: ")
	assert.Contains(t, string(body), "unsupported type")
}

func TestSSEEvent_with_id_field(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/sse-id", func(_ context.Context, _ *api.Void) (*api.SSEStream, error) {
		ch := make(chan api.SSEEvent, 1)
		ch <- api.SSEEvent{ID: "evt-42", Data: "hello"}
		close(ch)
		return &api.SSEStream{Events: ch}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/sse-id", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	content := string(body)
	assert.Contains(t, content, "id: evt-42")
	assert.Contains(t, content, "data: hello")
}

type noFlushWriter struct {
	header     http.Header
	statusCode int
	body       []byte
}

func (w *noFlushWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *noFlushWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return len(b), nil
}

func (w *noFlushWriter) WriteHeader(code int) {
	w.statusCode = code
}

func TestSSEStream_non_flusher_writer(t *testing.T) {
	t.Parallel()

	ch := make(chan api.SSEEvent, 1)
	ch <- api.SSEEvent{Data: "test"}
	close(ch)

	r := api.New()
	api.Get(r, "/sse-noflusher", func(_ context.Context, _ *api.Void) (*api.SSEStream, error) {
		return &api.SSEStream{Events: ch}, nil
	})

	// Use a non-flusher writer to trigger the 500 error path in writeSSEStream.
	w := &noFlushWriter{}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/sse-noflusher", nil)
	require.NoError(t, err)

	r.ServeHTTP(w, req)

	// writeSSEStream first writes headers (200), then detects no flusher and calls http.Error
	// which tries to write 500 but since headers are already sent, the status might vary.
	// The important thing is the handler completes without panic.
	bodyStr := string(w.body)
	assert.Contains(t, bodyStr, "streaming not supported")
}
