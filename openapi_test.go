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

func TestSpec_basic(t *testing.T) {
	t.Parallel()

	type ListReq struct {
		Page int `query:"page" doc:"Page number"`
	}
	type Item struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	type ListResp struct {
		Items []Item `json:"items"`
	}
	type CreateReq struct {
		Body struct {
			Name string `json:"name" required:"true" doc:"Item name"`
		}
	}

	r := api.New(api.WithTitle("Items API"), api.WithVersion("2.0.0"))

	api.Get(r, "/items", func(_ context.Context, req *ListReq) (*ListResp, error) {
		return &ListResp{}, nil
	}, api.WithSummary("List items"), api.WithTags("items"))

	api.Post(r, "/items", func(_ context.Context, req *CreateReq) (*Item, error) {
		return &Item{ID: "1", Name: req.Body.Name}, nil
	}, api.WithStatus(http.StatusCreated), api.WithTags("items"))

	api.Delete(r, "/items/{id}", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	}, api.WithTags("items"))

	spec := r.Spec()

	assert.Equal(t, "3.1.0", spec.OpenAPI)
	assert.Equal(t, "Items API", spec.Info.Title)
	assert.Equal(t, "2.0.0", spec.Info.Version)

	// GET /items
	getItems, ok := spec.Paths["/items"]["get"]
	require.True(t, ok)
	assert.Equal(t, "List items", getItems.Summary)
	assert.Contains(t, getItems.Tags, "items")
	require.Len(t, getItems.Parameters, 1)
	assert.Equal(t, "page", getItems.Parameters[0].Name)
	assert.Equal(t, "query", getItems.Parameters[0].In)

	// POST /items
	postItems, ok := spec.Paths["/items"]["post"]
	require.True(t, ok)
	require.NotNil(t, postItems.RequestBody)
	assert.True(t, postItems.RequestBody.Required)
	_, hasResp := postItems.Responses["201"]
	assert.True(t, hasResp)

	// DELETE /items/{id}
	deleteItems, ok := spec.Paths["/items/{id}"]["delete"]
	require.True(t, ok)
	_, hasNoContent := deleteItems.Responses["204"]
	assert.True(t, hasNoContent)
}

func TestSpec_deprecated_route(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/old", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	}, api.WithDeprecated())

	spec := r.Spec()
	op := spec.Paths["/old"]["get"]
	assert.True(t, op.Deprecated)
}

func TestServeSpec(t *testing.T) {
	t.Parallel()

	r := api.New(api.WithTitle("Spec Test"), api.WithVersion("0.1.0"))
	api.Get(r, "/health", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	})
	r.ServeSpec("/openapi.json")

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/openapi.json", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var spec api.OpenAPISpec
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&spec))
	assert.Equal(t, "3.1.0", spec.OpenAPI)
	assert.Equal(t, "Spec Test", spec.Info.Title)
	assert.Contains(t, spec.Paths, "/health")
}

func TestSpec_body_only_request(t *testing.T) {
	t.Parallel()

	type CreateUser struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	type User struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	r := api.New()
	api.Post(r, "/users", func(_ context.Context, req *CreateUser) (*User, error) {
		return &User{ID: "1", Name: req.Name, Email: req.Email}, nil
	})

	spec := r.Spec()
	postOp := spec.Paths["/users"]["post"]
	require.NotNil(t, postOp.RequestBody)

	schema := postOp.RequestBody.Content["application/json"].Schema
	assert.Equal(t, "object", schema.Type)
	_, hasName := schema.Properties["name"]
	assert.True(t, hasName)
	_, hasEmail := schema.Properties["email"]
	assert.True(t, hasEmail)
}
