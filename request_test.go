package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjaus/api"
)

func TestRequest_path_params(t *testing.T) {
	t.Parallel()

	type Req struct {
		ID string `path:"id"`
	}
	type Resp struct {
		ID string `json:"id"`
	}

	r := api.New()
	api.Get(r, "/items/{id}", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{ID: req.ID}, nil
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/items/abc123", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	var body Resp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "abc123", body.ID)
}

func TestRequest_query_params(t *testing.T) {
	t.Parallel()

	type Req struct {
		Page int    `query:"page" default:"1"`
		Sort string `query:"sort" default:"name"`
	}
	type Resp struct {
		Page int    `json:"page"`
		Sort string `json:"sort"`
	}

	r := api.New()
	api.Get(r, "/items", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{Page: req.Page, Sort: req.Sort}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	tests := map[string]struct {
		query      string
		expectPage int
		expectSort string
	}{
		"explicit values": {
			query:      "?page=3&sort=date",
			expectPage: 3,
			expectSort: "date",
		},
		"defaults": {
			query:      "",
			expectPage: 1,
			expectSort: "name",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/items"+tc.query, nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			var body Resp
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
			assert.Equal(t, tc.expectPage, body.Page)
			assert.Equal(t, tc.expectSort, body.Sort)
		})
	}
}

func TestRequest_json_body(t *testing.T) {
	t.Parallel()

	type Req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	type Resp struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	r := api.New()
	api.Post(r, "/users", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{Name: req.Name, Email: req.Email}, nil
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	payload := `{"name":"Alice","email":"alice@example.com"}`
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/users", strings.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body Resp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "Alice", body.Name)
	assert.Equal(t, "alice@example.com", body.Email)
}

func TestRequest_mixed_params_and_body(t *testing.T) {
	t.Parallel()

	type Req struct {
		OrgID string `path:"org_id"`
		Body  struct {
			Name string `json:"name"`
		}
	}
	type Resp struct {
		OrgID string `json:"org_id"`
		Name  string `json:"name"`
	}

	r := api.New()
	api.Post(r, "/orgs/{org_id}/users", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{OrgID: req.OrgID, Name: req.Body.Name}, nil
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		srv.URL+"/orgs/org-42/users",
		strings.NewReader(`{"name":"Bob"}`),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	var body Resp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "org-42", body.OrgID)
	assert.Equal(t, "Bob", body.Name)
}

func TestRequest_header_binding(t *testing.T) {
	t.Parallel()

	type Req struct {
		Token string `header:"Authorization"`
	}
	type Resp struct {
		Token string `json:"token"`
	}

	r := api.New()
	api.Get(r, "/auth", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{Token: req.Token}, nil
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/auth", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer secret")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	var body Resp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "Bearer secret", body.Token)
}

func TestRequest_RawRequest_embedding(t *testing.T) {
	t.Parallel()

	type Req struct {
		api.RawRequest
	}
	type Resp struct {
		Method string `json:"method"`
		Path   string `json:"path"`
	}

	r := api.New()
	api.Get(r, "/raw", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{
			Method: req.Request.Method,
			Path:   req.Request.URL.Path,
		}, nil
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/raw", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	var body Resp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "GET", body.Method)
	assert.Equal(t, "/raw", body.Path)
}
