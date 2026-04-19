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

	api.Get(v1, "/health", func(_ context.Context, _ *api.Void) (*api.Resp[Resp], error) {
		return &api.Resp[Resp]{Body: Resp{Version: "v1"}}, nil
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

	api.Get(authed, "/dashboard", func(_ context.Context, _ *api.Void) (*api.Resp[Resp], error) {
		return &api.Resp[Resp]{Body: Resp{OK: true}}, nil
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

func TestGroup_nested_prefix(t *testing.T) {
	t.Parallel()

	type Resp struct {
		OK bool `json:"ok"`
	}

	r := api.New()
	api.Get(
		r.Group("/api").Group("/identity").Group("/admin"),
		"/users",
		func(_ context.Context, _ *api.Void) (*api.Resp[Resp], error) {
			return &api.Resp[Resp]{Body: Resp{OK: true}}, nil
		},
	)

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/api/identity/admin/users", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestGroup_nested_middleware_order(t *testing.T) {
	t.Parallel()

	r := api.New()

	record := func(label string) api.Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Add("X-Order", label)
				next.ServeHTTP(w, req)
			})
		}
	}

	outer := r.Group("/a", api.WithGroupMiddleware(record("outer")))
	middle := outer.Group("/b", api.WithGroupMiddleware(record("middle")))
	inner := middle.Group("/c", api.WithGroupMiddleware(record("inner")))

	api.Get(inner, "/ping", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/a/b/c/ping", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, []string{"outer", "middle", "inner"}, resp.Header.Values("X-Order"))
}

func TestGroup_nested_middleware_reset(t *testing.T) {
	t.Parallel()

	r := api.New()

	record := func(label string) api.Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Add("X-Order", label)
				next.ServeHTTP(w, req)
			})
		}
	}

	parent := r.Group("/a", api.WithGroupMiddleware(record("parent")))
	isolated := parent.Group("/b",
		api.WithGroupMiddlewareReset(),
		api.WithGroupMiddleware(record("child")),
	)

	api.Get(isolated, "/ping", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/a/b/ping", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, []string{"child"}, resp.Header.Values("X-Order"))
}

func TestGroup_nested_tags_union(t *testing.T) {
	t.Parallel()

	r := api.New(api.WithTitle("Test"))
	outer := r.Group("/api", api.WithGroupTags("api"))
	inner := outer.Group("/identity", api.WithGroupTags("identity"))

	api.Get(inner, "/users", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	}, api.WithTags("users"))

	spec := r.Spec()
	ops, ok := spec.Paths["/api/identity/users"]
	require.True(t, ok)
	assert.Equal(t, []string{"api", "identity", "users"}, ops["get"].Tags)
}

func TestGroup_nested_security_inherits(t *testing.T) {
	t.Parallel()

	r := api.New(
		api.WithTitle("Nested Security"),
		api.WithSecurityScheme("bearerAuth", api.SecurityScheme{
			Type:   "http",
			Scheme: "bearer",
		}),
	)

	outer := r.Group("/api", api.WithGroupSecurity("bearerAuth"))
	inner := outer.Group("/identity")

	api.Get(inner, "/me", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	})

	spec := r.Spec()
	op := spec.Paths["/api/identity/me"]["get"]
	require.NotNil(t, op.Security)
	require.Len(t, *op.Security, 1)
	assert.Contains(t, (*op.Security)[0], "bearerAuth")
}

func TestGroup_nested_security_skips_empty_ancestor(t *testing.T) {
	t.Parallel()

	r := api.New(
		api.WithTitle("Skip Empty Ancestor"),
		api.WithSecurityScheme("bearerAuth", api.SecurityScheme{
			Type:   "http",
			Scheme: "bearer",
		}),
	)

	outer := r.Group("/api", api.WithGroupSecurity("bearerAuth"))
	middle := outer.Group("/v1")
	inner := middle.Group("/users")

	api.Get(inner, "/me", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	})

	spec := r.Spec()
	op := spec.Paths["/api/v1/users/me"]["get"]
	require.NotNil(t, op.Security, "security should be inherited from grandparent")
	require.Len(t, *op.Security, 1)
	assert.Contains(t, (*op.Security)[0], "bearerAuth")
}

func TestGroup_nested_route_no_security(t *testing.T) {
	t.Parallel()

	r := api.New(
		api.WithTitle("Nested NoSecurity"),
		api.WithSecurityScheme("bearerAuth", api.SecurityScheme{
			Type:   "http",
			Scheme: "bearer",
		}),
	)

	outer := r.Group("/api", api.WithGroupSecurity("bearerAuth"))
	inner := outer.Group("/identity")

	api.Get(inner, "/health", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	}, api.WithNoSecurity())

	spec := r.Spec()
	op := spec.Paths["/api/identity/health"]["get"]
	require.NotNil(t, op.Security)
	assert.Empty(t, *op.Security, "WithNoSecurity should bypass all ancestor security")
}

func TestGroup_nested_security_child_replaces(t *testing.T) {
	t.Parallel()

	r := api.New(
		api.WithTitle("Nested Security Replace"),
		api.WithSecurityScheme("bearerAuth", api.SecurityScheme{
			Type:   "http",
			Scheme: "bearer",
		}),
		api.WithSecurityScheme("apiKey", api.SecurityScheme{
			Type: "apiKey",
			Name: "X-API-Key",
			In:   "header",
		}),
	)

	outer := r.Group("/api", api.WithGroupSecurity("bearerAuth"))
	inner := outer.Group("/admin", api.WithGroupSecurity("apiKey"))

	api.Get(inner, "/users", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	})

	spec := r.Spec()
	op := spec.Paths["/api/admin/users"]["get"]
	require.NotNil(t, op.Security)
	require.Len(t, *op.Security, 1)
	assert.Contains(t, (*op.Security)[0], "apiKey")
	assert.NotContains(t, (*op.Security)[0], "bearerAuth")
}
