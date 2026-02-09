package api_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjaus/api"
)

func TestWithGroupSecurity_applied(t *testing.T) {
	t.Parallel()

	r := api.New(
		api.WithTitle("Group Security"),
		api.WithSecurityScheme("bearerAuth", api.SecurityScheme{
			Type:   "http",
			Scheme: "bearer",
		}),
	)

	g := r.Group("/api", api.WithGroupSecurity("bearerAuth"))

	api.Get(g, "/items", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	})

	spec := r.Spec()
	path, ok := spec.Paths["/api/items"]
	require.True(t, ok, "path /api/items should exist")

	op := path["get"]
	require.NotNil(t, op.Security, "security should be set from group")
	require.Len(t, *op.Security, 1)
	assert.Contains(t, (*op.Security)[0], "bearerAuth")
}

func TestWithGroupSecurity_not_overridden_by_route(t *testing.T) {
	t.Parallel()

	r := api.New(
		api.WithTitle("Explicit Route Security"),
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

	g := r.Group("/api", api.WithGroupSecurity("bearerAuth"))

	// This route has explicit security, so group security should NOT apply.
	api.Get(g, "/special", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	}, api.WithSecurity("apiKey"))

	spec := r.Spec()
	path, ok := spec.Paths["/api/special"]
	require.True(t, ok)

	op := path["get"]
	require.NotNil(t, op.Security)
	require.Len(t, *op.Security, 1)
	// Should be apiKey, not bearerAuth.
	assert.Contains(t, (*op.Security)[0], "apiKey")
	assert.NotContains(t, (*op.Security)[0], "bearerAuth")
}

func TestWithGroupSecurity_not_applied_with_NoSecurity(t *testing.T) {
	t.Parallel()

	r := api.New(
		api.WithTitle("NoSecurity Route"),
		api.WithSecurityScheme("bearerAuth", api.SecurityScheme{
			Type:   "http",
			Scheme: "bearer",
		}),
	)

	g := r.Group("/api", api.WithGroupSecurity("bearerAuth"))

	api.Get(g, "/public", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	}, api.WithNoSecurity())

	spec := r.Spec()
	path, ok := spec.Paths["/api/public"]
	require.True(t, ok)

	op := path["get"]
	require.NotNil(t, op.Security, "security should be set (empty array for no security)")
	assert.Empty(t, *op.Security, "security should be an empty array for no-security routes")
}
