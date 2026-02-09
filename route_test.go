package api_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjaus/api"
)

func TestRouteOptions_applied_via_registration(t *testing.T) {
	t.Parallel()

	r := api.New(api.WithTitle("Options Test"))

	type Resp struct {
		ID string `json:"id"`
	}

	api.Get(r, "/items/{id}", func(_ context.Context, _ *api.Void) (*Resp, error) {
		return &Resp{ID: "1"}, nil
	},
		api.WithStatus(http.StatusCreated),
		api.WithSummary("Get an item"),
		api.WithDescription("Fetches a single item by ID"),
		api.WithTags("items", "read"),
		api.WithDeprecated(),
		api.WithErrors(http.StatusNotFound, http.StatusConflict),
		api.WithOperationID("getItemById"),
		api.WithExtension("x-custom", "value"),
	)

	spec := r.Spec()
	path, ok := spec.Paths["/items/{id}"]
	require.True(t, ok)

	op := path["get"]
	assert.Equal(t, "Get an item", op.Summary)
	assert.Equal(t, "Fetches a single item by ID", op.Description)
	assert.Contains(t, op.Tags, "items")
	assert.Contains(t, op.Tags, "read")
	assert.True(t, op.Deprecated)
	assert.Equal(t, "getItemById", op.OperationID)
	assert.Contains(t, op.Extensions, "x-custom")
	assert.Equal(t, "value", op.Extensions["x-custom"])
	// Verify errors are reflected in the spec.
	assert.Contains(t, op.Responses, "404")
	assert.Contains(t, op.Responses, "409")
}

func TestWithLink(t *testing.T) {
	t.Parallel()

	r := api.New(api.WithTitle("Link Test"))

	type Resp struct {
		ID string `json:"id"`
	}

	api.Get(r, "/users/{id}", func(_ context.Context, _ *api.Void) (*Resp, error) {
		return &Resp{ID: "123"}, nil
	}, api.WithLink("GetUser", api.Link{
		OperationID: "getUserById",
		Description: "Fetch the created user",
		Parameters:  map[string]any{"id": "$response.body#/id"},
	}))

	spec := r.Spec()
	path, ok := spec.Paths["/users/{id}"]
	require.True(t, ok, "path /users/{id} should exist")

	op, ok := path["get"]
	require.True(t, ok, "GET operation should exist")

	require.Contains(t, op.Responses, "200")
	resp200 := op.Responses["200"]
	require.NotNil(t, resp200.Links)
	require.Contains(t, resp200.Links, "GetUser")
	assert.Equal(t, "getUserById", resp200.Links["GetUser"].OperationID)
	assert.Equal(t, "Fetch the created user", resp200.Links["GetUser"].Description)
}

func TestWithCallback(t *testing.T) {
	t.Parallel()

	r := api.New(api.WithTitle("Callback Test"))

	type Resp struct {
		ID string `json:"id"`
	}

	api.Post(r, "/subscribe", func(_ context.Context, _ *api.Void) (*Resp, error) {
		return &Resp{ID: "sub-1"}, nil
	}, api.WithCallback("onEvent", map[string]api.PathItem{
		"{$request.body#/callbackUrl}": {
			"post": api.Operation{
				Summary: "Event callback",
			},
		},
	}))

	spec := r.Spec()
	path, ok := spec.Paths["/subscribe"]
	require.True(t, ok, "path /subscribe should exist")

	op, ok := path["post"]
	require.True(t, ok, "POST operation should exist")

	require.NotNil(t, op.Callbacks)
	require.Contains(t, op.Callbacks, "onEvent")
	cbPaths := op.Callbacks["onEvent"]
	require.Contains(t, cbPaths, "{$request.body#/callbackUrl}")
}

func TestWithDescription_in_spec(t *testing.T) {
	t.Parallel()

	r := api.New(api.WithTitle("Desc Test"))

	api.Get(r, "/health", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	}, api.WithDescription("Health check endpoint"))

	spec := r.Spec()
	path, ok := spec.Paths["/health"]
	require.True(t, ok)

	op := path["get"]
	assert.Equal(t, "Health check endpoint", op.Description)
}

func TestWithDescription_absent(t *testing.T) {
	t.Parallel()

	r := api.New(api.WithTitle("No Desc"))

	api.Get(r, "/ping", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	})

	spec := r.Spec()
	path, ok := spec.Paths["/ping"]
	require.True(t, ok)

	op := path["get"]
	assert.Empty(t, op.Description)
}
