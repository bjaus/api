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

func TestSetValueGetValue_roundTrip(t *testing.T) {
	t.Parallel()

	type userID string

	r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
	require.NoError(t, err)

	r = api.SetValue[userID](r, "user-123")

	val, ok := api.GetValue[userID](r.Context())
	assert.True(t, ok)
	assert.Equal(t, userID("user-123"), val)
}

func TestGetValue_missing_returns_false(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	val, ok := api.GetValue[string](ctx)
	assert.False(t, ok)
	assert.Equal(t, "", val)
}

func TestGetValue_missing_int_returns_zero(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	val, ok := api.GetValue[int](ctx)
	assert.False(t, ok)
	assert.Equal(t, 0, val)
}

func TestSetValueGetValue_different_types_no_collision(t *testing.T) {
	t.Parallel()

	r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
	require.NoError(t, err)

	r = api.SetValue[string](r, "hello")
	r = api.SetValue[int](r, 42)
	r = api.SetValue[bool](r, true)

	strVal, ok := api.GetValue[string](r.Context())
	assert.True(t, ok)
	assert.Equal(t, "hello", strVal)

	intVal, ok := api.GetValue[int](r.Context())
	assert.True(t, ok)
	assert.Equal(t, 42, intVal)

	boolVal, ok := api.GetValue[bool](r.Context())
	assert.True(t, ok)
	assert.True(t, boolVal)
}

func TestSetValueGetValue_custom_types_no_collision(t *testing.T) {
	t.Parallel()

	type tenantID string
	type requestID string

	r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/test", nil)
	require.NoError(t, err)

	r = api.SetValue[tenantID](r, "tenant-1")
	r = api.SetValue[requestID](r, "req-abc")

	tenant, ok := api.GetValue[tenantID](r.Context())
	assert.True(t, ok)
	assert.Equal(t, tenantID("tenant-1"), tenant)

	reqID, ok := api.GetValue[requestID](r.Context())
	assert.True(t, ok)
	assert.Equal(t, requestID("req-abc"), reqID)
}

func TestSetValueGetValue_in_middleware(t *testing.T) {
	t.Parallel()

	type userCtx struct {
		Name string
		Role string
	}

	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = api.SetValue[userCtx](r, userCtx{Name: "Alice", Role: "admin"})
			next.ServeHTTP(w, r)
		})
	}

	var captured userCtx
	var found bool
	handler := mw(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured, found = api.GetValue[userCtx](r.Context())
	}))

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.True(t, found)
	assert.Equal(t, "Alice", captured.Name)
	assert.Equal(t, "admin", captured.Role)
}
