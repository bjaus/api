package api_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjaus/api"
)

func TestLogger(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		handlerStatus int
		wantSubstr    []string
	}{
		"request is logged": {
			handlerStatus: http.StatusOK,
			wantSubstr: []string{
				"request",
				"method",
				"GET",
				"path",
				"/test-log",
				"status",
			},
		},
		"status code is captured": {
			handlerStatus: http.StatusCreated,
			wantSubstr: []string{
				"status",
				"201",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, nil))

			mw := api.Logger(logger)
			handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.handlerStatus)
			}))

			srv := httptest.NewServer(handler)
			t.Cleanup(srv.Close)

			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/test-log", nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			logOutput := buf.String()
			for _, s := range tc.wantSubstr {
				assert.Contains(t, logOutput, s, "log output should contain %q", s)
			}
		})
	}
}

func TestLogger_captures_body_size(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	bodyContent := "hello world response"
	mw := api.Logger(logger)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(bodyContent)) //nolint:errcheck
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/size-test", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	logOutput := buf.String()
	assert.Contains(t, logOutput, "size")
	assert.Contains(t, logOutput, "20")
}

func TestLogger_unwrap_response_controller(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	mw := api.Logger(logger)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Exercise the Unwrap path via http.ResponseController.
		// Flush implicitly writes headers so no separate WriteHeader call.
		rc := http.NewResponseController(w)
		_ = rc.Flush() //nolint:errcheck
	}))

	rec := httptest.NewRecorder()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/unwrap-test", nil)
	require.NoError(t, err)

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, buf.String(), "request")
}

func TestLogger_with_request_id(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Chain: RequestID -> Logger -> handler
	handler := api.RequestID()(api.Logger(logger)(inner))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/rid-test", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	logOutput := buf.String()
	assert.Contains(t, logOutput, "request_id")
}
