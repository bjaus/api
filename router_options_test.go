package api_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjaus/api"
)

func TestWithErrorHandler(t *testing.T) {
	t.Parallel()

	r := api.New(api.WithErrorHandler(func(w http.ResponseWriter, _ *http.Request, err error) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(api.ErrorStatus(err))
		//nolint:errcheck,gosec
		w.Write([]byte("custom: " + err.Error()))
	}))

	api.Get(r, "/fail", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return nil, api.Error(http.StatusTeapot, "I'm a teapot")
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/fail", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusTeapot, resp.StatusCode)
	assert.Equal(t, "text/plain", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "custom: I'm a teapot", string(body))
}

type mockEncoder struct{}

func (e *mockEncoder) ContentType() string             { return "application/xml" }
func (e *mockEncoder) Encode(_ io.Writer, _ any) error { return nil }

type mockDecoder struct{}

func (d *mockDecoder) ContentType() string             { return "application/xml" }
func (d *mockDecoder) Decode(_ io.Reader, _ any) error { return nil }

func TestWithEncoder(t *testing.T) {
	t.Parallel()

	r := api.New(api.WithEncoder(&mockEncoder{}))
	assert.NotNil(t, r)
}

func TestWithDecoder(t *testing.T) {
	t.Parallel()

	r := api.New(api.WithDecoder(&mockDecoder{}))
	assert.NotNil(t, r)
}

type mockTracer struct {
	called bool
}

func (m *mockTracer) StartSpan(ctx context.Context, _ string, _ map[string]string) (context.Context, func()) {
	m.called = true
	return ctx, func() {}
}

func TestWithTracer(t *testing.T) {
	t.Parallel()

	tracer := &mockTracer{}
	r := api.New(api.WithTracer(tracer))
	assert.NotNil(t, r)
}

func TestListenAndServe_cancelled_context(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/ping", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	err := r.ListenAndServe(ctx, "127.0.0.1:0")
	// The server should shut down due to the cancelled context.
	// Either it returns nil (graceful shutdown) or context.Canceled.
	if err != nil {
		assert.ErrorIs(t, err, context.Canceled)
	}
}

func TestListenAndServe_port_in_use(t *testing.T) {
	t.Parallel()

	// Bind a port first so ListenAndServe fails immediately via errCh path.
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, ln.Close()) })

	addr := ln.Addr().String()

	r := api.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = r.ListenAndServe(ctx, addr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bind")
}

type mockValidator struct{}

func (m *mockValidator) Validate(_ any) error { return nil }

func TestWithValidator(t *testing.T) {
	t.Parallel()

	r := api.New(api.WithValidator(&mockValidator{}))
	assert.NotNil(t, r)
}

func TestWithServers(t *testing.T) {
	t.Parallel()

	r := api.New(
		api.WithTitle("Server Test"),
		api.WithServers(
			api.Server{URL: "https://api.example.com", Description: "Production"},
			api.Server{URL: "https://staging.example.com", Description: "Staging"},
		),
	)

	spec := r.Spec()
	require.Len(t, spec.Servers, 2)
	assert.Equal(t, "https://api.example.com", spec.Servers[0].URL)
	assert.Equal(t, "Production", spec.Servers[0].Description)
}

func TestWithGlobalSecurity(t *testing.T) {
	t.Parallel()

	r := api.New(
		api.WithTitle("Global Security"),
		api.WithSecurityScheme("bearerAuth", api.SecurityScheme{
			Type:   "http",
			Scheme: "bearer",
		}),
		api.WithGlobalSecurity("bearerAuth"),
	)

	spec := r.Spec()
	require.Len(t, spec.Security, 1)
	assert.Contains(t, spec.Security[0], "bearerAuth")
}

func TestWithTagDescriptions(t *testing.T) {
	t.Parallel()

	r := api.New(
		api.WithTitle("Tag Desc"),
		api.WithTagDescriptions(map[string]string{
			"users":  "User operations",
			"orders": "Order operations",
		}),
	)

	spec := r.Spec()
	require.Len(t, spec.Tags, 2)
	// Tags are sorted by name.
	assert.Equal(t, "orders", spec.Tags[0].Name)
	assert.Equal(t, "Order operations", spec.Tags[0].Description)
	assert.Equal(t, "users", spec.Tags[1].Name)
	assert.Equal(t, "User operations", spec.Tags[1].Description)
}

func TestWithWebhook(t *testing.T) {
	t.Parallel()

	r := api.New(
		api.WithTitle("Webhook Test"),
		api.WithWebhook("orderCreated", api.PathItem{
			"post": api.Operation{
				Summary: "Order created webhook",
			},
		}),
	)

	spec := r.Spec()
	require.NotNil(t, spec.Webhooks)
	require.Contains(t, spec.Webhooks, "orderCreated")
}
