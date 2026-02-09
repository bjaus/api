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

	// Components should contain named types.
	require.NotNil(t, spec.Components)
	require.Contains(t, spec.Components.Schemas, "ListResp")
	require.Contains(t, spec.Components.Schemas, "Item")

	listRespSchema := spec.Components.Schemas["ListResp"]
	assert.Equal(t, "object", listRespSchema.Type)
	assert.Contains(t, listRespSchema.Properties, "items")

	itemSchema := spec.Components.Schemas["Item"]
	assert.Equal(t, "object", itemSchema.Type)
	assert.Contains(t, itemSchema.Properties, "id")
	assert.Contains(t, itemSchema.Properties, "name")

	// GET /items — response uses $ref.
	getItems, ok := spec.Paths["/items"]["get"]
	require.True(t, ok)
	assert.Equal(t, "List items", getItems.Summary)
	assert.Contains(t, getItems.Tags, "items")
	require.Len(t, getItems.Parameters, 1)
	assert.Equal(t, "page", getItems.Parameters[0].Name)
	assert.Equal(t, "query", getItems.Parameters[0].In)

	respSchema := getItems.Responses["200"].Content["application/json"].Schema
	assert.Equal(t, "#/components/schemas/ListResp", respSchema.Ref)

	// POST /items — request body (anonymous Body) stays inline, response uses $ref.
	postItems, ok := spec.Paths["/items"]["post"]
	require.True(t, ok)
	require.NotNil(t, postItems.RequestBody)
	assert.True(t, postItems.RequestBody.Required)

	bodySchema := postItems.RequestBody.Content["application/json"].Schema
	assert.Equal(t, "object", bodySchema.Type)
	assert.Contains(t, bodySchema.Properties, "name")

	_, hasResp := postItems.Responses["201"]
	assert.True(t, hasResp)
	postRespSchema := postItems.Responses["201"].Content["application/json"].Schema
	assert.Equal(t, "#/components/schemas/Item", postRespSchema.Ref)

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

	// Named request type → $ref.
	reqSchema := postOp.RequestBody.Content["application/json"].Schema
	assert.Equal(t, "#/components/schemas/CreateUser", reqSchema.Ref)

	// Full schema in components.
	require.NotNil(t, spec.Components)
	require.Contains(t, spec.Components.Schemas, "CreateUser")
	fullSchema := spec.Components.Schemas["CreateUser"]
	assert.Equal(t, "object", fullSchema.Type)
	assert.Contains(t, fullSchema.Properties, "name")
	assert.Contains(t, fullSchema.Properties, "email")

	// Named response type → $ref.
	respSchema := postOp.Responses["200"].Content["application/json"].Schema
	assert.Equal(t, "#/components/schemas/User", respSchema.Ref)

	require.Contains(t, spec.Components.Schemas, "User")
	userSchema := spec.Components.Schemas["User"]
	assert.Equal(t, "object", userSchema.Type)
	assert.Contains(t, userSchema.Properties, "id")
	assert.Contains(t, userSchema.Properties, "name")
	assert.Contains(t, userSchema.Properties, "email")
}

func TestSpec_components_schemas(t *testing.T) {
	t.Parallel()

	type Resp struct {
		Value string `json:"value"`
	}

	r := api.New()
	api.Get(r, "/a", func(_ context.Context, _ *api.Void) (*Resp, error) {
		return &Resp{}, nil
	})
	api.Get(r, "/b", func(_ context.Context, _ *api.Void) (*Resp, error) {
		return &Resp{}, nil
	})

	spec := r.Spec()
	require.NotNil(t, spec.Components)

	// Same named type used twice → single entry in components.
	require.Contains(t, spec.Components.Schemas, "Resp")
	assert.Equal(t, "object", spec.Components.Schemas["Resp"].Type)

	// Both operations reference the same $ref.
	aSchema := spec.Paths["/a"]["get"].Responses["200"].Content["application/json"].Schema
	bSchema := spec.Paths["/b"]["get"].Responses["200"].Content["application/json"].Schema
	assert.Equal(t, "#/components/schemas/Resp", aSchema.Ref)
	assert.Equal(t, "#/components/schemas/Resp", bSchema.Ref)
}

func TestSpec_error_responses_default(t *testing.T) {
	t.Parallel()

	type Resp struct {
		OK bool `json:"ok"`
	}

	r := api.New()
	api.Get(r, "/ping", func(_ context.Context, _ *api.Void) (*Resp, error) {
		return &Resp{OK: true}, nil
	})

	spec := r.Spec()
	op := spec.Paths["/ping"]["get"]

	// All routes get 400 and 500.
	assert.Contains(t, op.Responses, "400")
	assert.Contains(t, op.Responses, "500")

	// No path param → no 404.
	assert.NotContains(t, op.Responses, "404")

	// Error responses reference ProblemDetail schema.
	assert.Equal(t, "#/components/schemas/ProblemDetail", op.Responses["400"].Content["application/json"].Schema.Ref)
	assert.Equal(t, "#/components/schemas/ProblemDetail", op.Responses["500"].Content["application/json"].Schema.Ref)
}

func TestSpec_error_responses_path_param(t *testing.T) {
	t.Parallel()

	type Req struct {
		ID string `path:"id"`
	}
	type Resp struct {
		ID string `json:"id"`
	}

	r := api.New()
	api.Get(r, "/things/{id}", func(_ context.Context, _ *Req) (*Resp, error) {
		return &Resp{}, nil
	})

	spec := r.Spec()
	op := spec.Paths["/things/{id}"]["get"]

	assert.Contains(t, op.Responses, "400")
	assert.Contains(t, op.Responses, "404")
	assert.Contains(t, op.Responses, "500")
}

func TestSpec_WithErrors(t *testing.T) {
	t.Parallel()

	type Resp struct {
		OK bool `json:"ok"`
	}

	r := api.New()
	api.Post(r, "/items", func(_ context.Context, _ *api.Void) (*Resp, error) {
		return &Resp{}, nil
	}, api.WithErrors(http.StatusConflict, http.StatusUnprocessableEntity))

	spec := r.Spec()
	op := spec.Paths["/items"]["post"]

	assert.Contains(t, op.Responses, "409")
	assert.Contains(t, op.Responses, "422")
	assert.Equal(t, "Conflict", op.Responses["409"].Description)
	assert.Equal(t, "Unprocessable Entity", op.Responses["422"].Description)
}

func TestSpec_error_responses_dedup(t *testing.T) {
	t.Parallel()

	type Resp struct {
		OK bool `json:"ok"`
	}

	r := api.New()
	api.Get(r, "/items/{id}", func(_ context.Context, _ *api.Void) (*Resp, error) {
		return &Resp{}, nil
	}, api.WithErrors(http.StatusBadRequest, http.StatusNotFound))

	spec := r.Spec()
	op := spec.Paths["/items/{id}"]["get"]

	// Count that 400 and 404 appear exactly once (map keys are unique by definition).
	assert.Contains(t, op.Responses, "400")
	assert.Contains(t, op.Responses, "404")
	assert.Contains(t, op.Responses, "500")
}

func TestSpec_ProblemDetail_schema(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/health", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	})

	spec := r.Spec()
	require.NotNil(t, spec.Components)
	require.Contains(t, spec.Components.Schemas, "ProblemDetail")

	errSchema := spec.Components.Schemas["ProblemDetail"]
	assert.Equal(t, "object", errSchema.Type)
	assert.Contains(t, errSchema.Properties, "status")
	assert.Contains(t, errSchema.Properties, "title")
	assert.Contains(t, errSchema.Properties, "detail")
	assert.Contains(t, errSchema.Properties, "errors")
	assert.Equal(t, []string{"status"}, errSchema.Required)
}

func TestSpec_WithServers(t *testing.T) {
	t.Parallel()

	r := api.New(
		api.WithTitle("Server Test"),
		api.WithVersion("1.0.0"),
		api.WithServers(
			api.Server{URL: "https://api.example.com", Description: "Production"},
			api.Server{URL: "https://staging.example.com", Description: "Staging"},
		),
	)

	api.Get(r, "/health", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	})

	spec := r.Spec()

	require.Len(t, spec.Servers, 2)
	assert.Equal(t, "https://api.example.com", spec.Servers[0].URL)
	assert.Equal(t, "Production", spec.Servers[0].Description)
	assert.Equal(t, "https://staging.example.com", spec.Servers[1].URL)
	assert.Equal(t, "Staging", spec.Servers[1].Description)
}

func TestSpec_WithSecurityScheme_and_WithGlobalSecurity(t *testing.T) {
	t.Parallel()

	r := api.New(
		api.WithSecurityScheme("bearerAuth", api.SecurityScheme{
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
		}),
		api.WithSecurityScheme("apiKey", api.SecurityScheme{
			Type: "apiKey",
			Name: "X-API-Key",
			In:   "header",
		}),
		api.WithGlobalSecurity("bearerAuth"),
	)

	api.Get(r, "/secured", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	})

	spec := r.Spec()

	// Security schemes should appear in components.
	require.NotNil(t, spec.Components)
	require.Contains(t, spec.Components.SecuritySchemes, "bearerAuth")
	require.Contains(t, spec.Components.SecuritySchemes, "apiKey")

	bearerScheme := spec.Components.SecuritySchemes["bearerAuth"]
	assert.Equal(t, "http", bearerScheme.Type)
	assert.Equal(t, "bearer", bearerScheme.Scheme)
	assert.Equal(t, "JWT", bearerScheme.BearerFormat)

	apiKeyScheme := spec.Components.SecuritySchemes["apiKey"]
	assert.Equal(t, "apiKey", apiKeyScheme.Type)
	assert.Equal(t, "X-API-Key", apiKeyScheme.Name)
	assert.Equal(t, "header", apiKeyScheme.In)

	// Global security should appear at the top level.
	require.Len(t, spec.Security, 1)
	_, hasBearerAuth := spec.Security[0]["bearerAuth"]
	assert.True(t, hasBearerAuth)
}

func TestSpec_WithOperationID(t *testing.T) {
	t.Parallel()

	r := api.New()

	api.Get(r, "/users", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	}, api.WithOperationID("listAllUsers"))

	api.Post(r, "/users", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	}, api.WithOperationID("createUser"))

	spec := r.Spec()

	getOp := spec.Paths["/users"]["get"]
	assert.Equal(t, "listAllUsers", getOp.OperationID)

	postOp := spec.Paths["/users"]["post"]
	assert.Equal(t, "createUser", postOp.OperationID)
}

func TestSpec_WithTagDescriptions(t *testing.T) {
	t.Parallel()

	r := api.New(
		api.WithTagDescriptions(map[string]string{
			"users":    "User management endpoints",
			"products": "Product catalog endpoints",
		}),
	)

	api.Get(r, "/users", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	}, api.WithTags("users"))

	spec := r.Spec()

	require.Len(t, spec.Tags, 2)

	tagMap := make(map[string]string)
	for _, tag := range spec.Tags {
		tagMap[tag.Name] = tag.Description
	}

	assert.Equal(t, "User management endpoints", tagMap["users"])
	assert.Equal(t, "Product catalog endpoints", tagMap["products"])
}

func TestSpec_generateOperationID_auto(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		method string
		pattern string
		want   string
	}{
		"simple get": {
			method:  "GET",
			pattern: "/users",
			want:    "getUsers",
		},
		"get with path param": {
			method:  "GET",
			pattern: "/users/{id}",
			want:    "getUsersById",
		},
		"post nested": {
			method:  "POST",
			pattern: "/v1/users",
			want:    "postV1Users",
		},
		"delete with param": {
			method:  "DELETE",
			pattern: "/items/{id}",
			want:    "deleteItemsById",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := api.GenerateOperationID(tc.method, tc.pattern)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestSpec_generateOperationID_via_spec(t *testing.T) {
	t.Parallel()

	r := api.New()

	// Route without explicit operationID should use generated one.
	api.Get(r, "/users/{id}", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	})

	spec := r.Spec()
	op := spec.Paths["/users/{id}"]["get"]
	assert.Equal(t, "getUsersById", op.OperationID)
}

func TestSpec_schema_constraints_appear(t *testing.T) {
	t.Parallel()

	type CreateReq struct {
		Body struct {
			Name   string   `json:"name" minLength:"2" maxLength:"100" pattern:"^[a-zA-Z]+$"`
			Age    int      `json:"age" minimum:"0" maximum:"150"`
			Role   string   `json:"role" enum:"admin,user,guest"`
			Tags   []string `json:"tags" minItems:"1" maxItems:"10"`
		}
	}
	type Resp struct {
		OK bool `json:"ok"`
	}

	r := api.New()
	api.Post(r, "/users", func(_ context.Context, _ *CreateReq) (*Resp, error) {
		return &Resp{OK: true}, nil
	})

	spec := r.Spec()
	postOp := spec.Paths["/users"]["post"]
	require.NotNil(t, postOp.RequestBody)

	bodySchema := postOp.RequestBody.Content["application/json"].Schema
	require.NotNil(t, bodySchema)

	// Name constraints.
	nameProp := bodySchema.Properties["name"]
	require.NotNil(t, nameProp.MinLength)
	assert.Equal(t, 2, *nameProp.MinLength)
	require.NotNil(t, nameProp.MaxLength)
	assert.Equal(t, 100, *nameProp.MaxLength)
	assert.Equal(t, "^[a-zA-Z]+$", nameProp.Pattern)

	// Age constraints.
	ageProp := bodySchema.Properties["age"]
	require.NotNil(t, ageProp.Minimum)
	assert.InDelta(t, 0.0, *ageProp.Minimum, 0.001)
	require.NotNil(t, ageProp.Maximum)
	assert.InDelta(t, 150.0, *ageProp.Maximum, 0.001)

	// Role enum.
	roleProp := bodySchema.Properties["role"]
	assert.Equal(t, []string{"admin", "user", "guest"}, roleProp.Enum)

	// Tags items constraints.
	tagsProp := bodySchema.Properties["tags"]
	require.NotNil(t, tagsProp.MinItems)
	assert.Equal(t, 1, *tagsProp.MinItems)
	require.NotNil(t, tagsProp.MaxItems)
	assert.Equal(t, 10, *tagsProp.MaxItems)
}

func TestSpec_WithExtension(t *testing.T) {
	t.Parallel()

	r := api.New()

	api.Get(r, "/internal", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	},
		api.WithExtension("x-internal", true),
		api.WithExtension("x-rate-limit", 100),
	)

	spec := r.Spec()
	op := spec.Paths["/internal"]["get"]

	require.NotNil(t, op.Extensions)
	assert.Equal(t, true, op.Extensions["x-internal"])
	assert.Equal(t, 100, op.Extensions["x-rate-limit"])
}

func TestSpec_WithWebhook(t *testing.T) {
	t.Parallel()

	r := api.New(
		api.WithWebhook("orderCreated", api.PathItem{
			"post": api.Operation{
				Summary:     "Order created webhook",
				OperationID: "orderCreatedWebhook",
				Responses: api.OperationResp{
					"200": api.ResponseObj{
						Description: "Webhook processed",
					},
				},
			},
		}),
	)

	api.Get(r, "/health", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	})

	spec := r.Spec()

	require.NotNil(t, spec.Webhooks)
	require.Contains(t, spec.Webhooks, "orderCreated")

	webhookPost, ok := spec.Webhooks["orderCreated"]["post"]
	require.True(t, ok)
	assert.Equal(t, "Order created webhook", webhookPost.Summary)
	assert.Equal(t, "orderCreatedWebhook", webhookPost.OperationID)
	assert.Contains(t, webhookPost.Responses, "200")
}

func TestSpec_route_level_security(t *testing.T) {
	t.Parallel()

	r := api.New(
		api.WithSecurityScheme("bearerAuth", api.SecurityScheme{
			Type:   "http",
			Scheme: "bearer",
		}),
		api.WithGlobalSecurity("bearerAuth"),
	)

	// Route with noSecurity overrides global.
	api.Get(r, "/public", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	}, api.WithNoSecurity())

	// Route with specific security.
	api.Get(r, "/admin", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	}, api.WithSecurity("bearerAuth"))

	spec := r.Spec()

	// Public route: empty security array.
	publicOp := spec.Paths["/public"]["get"]
	require.NotNil(t, publicOp.Security)
	assert.Empty(t, *publicOp.Security)

	// Admin route: explicit security.
	adminOp := spec.Paths["/admin"]["get"]
	require.NotNil(t, adminOp.Security)
	require.Len(t, *adminOp.Security, 1)
	_, hasBearerAuth := (*adminOp.Security)[0]["bearerAuth"]
	assert.True(t, hasBearerAuth)
}

func TestSpec_no_servers_omitted(t *testing.T) {
	t.Parallel()

	r := api.New(api.WithTitle("No Servers"))
	api.Get(r, "/health", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	})

	spec := r.Spec()
	assert.Nil(t, spec.Servers)
}

func TestSpec_multiple_global_security(t *testing.T) {
	t.Parallel()

	r := api.New(
		api.WithGlobalSecurity("bearerAuth", "apiKey"),
	)
	api.Get(r, "/health", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	})

	spec := r.Spec()

	require.Len(t, spec.Security, 2)
	_, has0 := spec.Security[0]["bearerAuth"]
	assert.True(t, has0)
	_, has1 := spec.Security[1]["apiKey"]
	assert.True(t, has1)
}

func TestSpec_stream_response(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/download", func(_ context.Context, _ *api.Void) (*api.Stream, error) {
		return &api.Stream{ContentType: "application/octet-stream"}, nil
	})

	spec := r.Spec()
	op := spec.Paths["/download"]["get"]
	resp200, ok := op.Responses["200"]
	require.True(t, ok)
	assert.Contains(t, resp200.Content, "application/octet-stream")
}

func TestSpec_sse_stream_response(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/events", func(_ context.Context, _ *api.Void) (*api.SSEStream, error) {
		ch := make(chan api.SSEEvent)
		close(ch)
		return &api.SSEStream{Events: ch}, nil
	})

	spec := r.Spec()
	op := spec.Paths["/events"]["get"]
	resp200, ok := op.Responses["200"]
	require.True(t, ok)
	assert.Contains(t, resp200.Content, "text/event-stream")
	assert.Equal(t, "string", resp200.Content["text/event-stream"].Schema.Type)
}

type respWithHeaders struct {
	Value string `json:"value"`
}

func (*respWithHeaders) ResponseHeaders() map[string]api.HeaderObj {
	return map[string]api.HeaderObj{
		"X-Rate-Limit": {
			Description: "Rate limit remaining",
			Schema:      api.JSONSchema{Type: "integer"},
		},
	}
}

func TestSpec_response_headers_interface(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/rate-limited", func(_ context.Context, _ *api.Void) (*respWithHeaders, error) {
		return &respWithHeaders{Value: "ok"}, nil
	})

	spec := r.Spec()
	op := spec.Paths["/rate-limited"]["get"]
	resp200, ok := op.Responses["200"]
	require.True(t, ok)
	require.NotNil(t, resp200.Headers)
	assert.Contains(t, resp200.Headers, "X-Rate-Limit")
	assert.Equal(t, "integer", resp200.Headers["X-Rate-Limit"].Schema.Type)
}

func TestSpec_header_and_cookie_params(t *testing.T) {
	t.Parallel()

	type Req struct {
		Auth    string `header:"Authorization" doc:"Bearer token"`
		Session string `cookie:"session_id" required:"true"`
	}
	type Resp struct {
		OK bool `json:"ok"`
	}

	r := api.New()
	api.Get(r, "/auth", func(_ context.Context, _ *Req) (*Resp, error) {
		return &Resp{OK: true}, nil
	})

	spec := r.Spec()
	op := spec.Paths["/auth"]["get"]

	require.Len(t, op.Parameters, 2)

	paramMap := make(map[string]api.Parameter)
	for _, p := range op.Parameters {
		paramMap[p.Name] = p
	}

	auth := paramMap["Authorization"]
	assert.Equal(t, "header", auth.In)
	assert.Equal(t, "Bearer token", auth.Description)

	session := paramMap["session_id"]
	assert.Equal(t, "cookie", session.In)
	assert.True(t, session.Required)
}

func TestSpec_unexported_field_ignored_in_params(t *testing.T) {
	t.Parallel()

	type Req struct {
		internal string //nolint:unused
		Name     string `query:"name"`
	}
	type Resp struct {
		OK bool `json:"ok"`
	}

	r := api.New()
	api.Get(r, "/search", func(_ context.Context, _ *Req) (*Resp, error) {
		return &Resp{OK: true}, nil
	})

	spec := r.Spec()
	op := spec.Paths["/search"]["get"]

	// Only the exported Name field should appear as a parameter.
	require.Len(t, op.Parameters, 1)
	assert.Equal(t, "name", op.Parameters[0].Name)
	assert.Equal(t, "query", op.Parameters[0].In)
}
