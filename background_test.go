package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjaus/api"
)

func TestBackground_runs_after_response(t *testing.T) {
	t.Parallel()

	var (
		mu       sync.Mutex
		invoked  bool
		gotValue string
	)
	done := make(chan struct{})

	r := api.New()
	api.Get(r, "/notify", func(ctx context.Context, _ *api.Void) (*api.Void, error) {
		api.Background(ctx, func(_ context.Context) {
			mu.Lock()
			invoked = true
			gotValue = "ran"
			mu.Unlock()
			close(done)
		})
		return &api.Void{}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/notify", nil)
	require.NoError(t, err)
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("background task did not run")
	}

	mu.Lock()
	defer mu.Unlock()
	assert.True(t, invoked)
	assert.Equal(t, "ran", gotValue)
}

func TestBackground_outside_handler_is_noop(t *testing.T) {
	t.Parallel()

	// No queue installed — should not panic.
	api.Background(context.Background(), func(_ context.Context) {
		t.Fatal("background fn should not run when no queue is installed")
	})
	// Give any rogue goroutine a chance to fire.
	time.Sleep(50 * time.Millisecond)
}

func TestBackground_panic_recovered(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/boom", func(ctx context.Context, _ *api.Void) (*api.Void, error) {
		api.Background(ctx, func(_ context.Context) {
			panic("recover me")
		})
		return &api.Void{}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/boom", nil)
	require.NoError(t, err)
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()
	// Server should still respond 204 — the background panic is contained.
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}
