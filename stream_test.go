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
