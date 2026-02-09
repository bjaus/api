package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestRequest_void_request(t *testing.T) {
	t.Parallel()

	type Resp struct {
		Message string `json:"message"`
	}

	r := api.New()
	api.Get(r, "/void", func(_ context.Context, _ *api.Void) (*Resp, error) {
		return &Resp{Message: "ok"}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/void", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Message string `json:"message"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "ok", body.Message)
}

func TestRequest_cookie_binding(t *testing.T) {
	t.Parallel()

	type Req struct {
		Session string `cookie:"session_id"`
	}
	type Resp struct {
		Session string `json:"session"`
	}

	r := api.New()
	api.Get(r, "/session", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{Session: req.Session}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/session", nil)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "abc123"})

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	var body Resp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "abc123", body.Session)
}

func TestRequest_cookie_default(t *testing.T) {
	t.Parallel()

	type Req struct {
		Session string `cookie:"session_id" default:"default-session"`
	}
	type Resp struct {
		Session string `json:"session"`
	}

	r := api.New()
	api.Get(r, "/session-default", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{Session: req.Session}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/session-default", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	var body Resp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "default-session", body.Session)
}

func TestRequest_header_default(t *testing.T) {
	t.Parallel()

	type Req struct {
		Accept string `header:"Accept" default:"application/json"`
	}
	type Resp struct {
		Accept string `json:"accept"`
	}

	r := api.New()
	api.Get(r, "/header-default", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{Accept: req.Accept}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/header-default", nil)
	require.NoError(t, err)
	// Do not set Accept header, let default apply.

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	var body Resp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "application/json", body.Accept)
}

func TestRequest_setFieldValue_types(t *testing.T) {
	t.Parallel()

	type Req struct {
		Duration string  `query:"dur"`
		Price    float64 `query:"price"`
		Active   bool    `query:"active"`
		Count    int     `query:"count"`
	}
	type Resp struct {
		Duration string  `json:"duration"`
		Price    float64 `json:"price"`
		Active   bool    `json:"active"`
		Count    int     `json:"count"`
	}

	r := api.New()
	api.Get(r, "/types", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{
			Duration: req.Duration,
			Price:    req.Price,
			Active:   req.Active,
			Count:    req.Count,
		}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		srv.URL+"/types?dur=5s&price=19.99&active=true&count=42", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body Resp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "5s", body.Duration)
	assert.InDelta(t, 19.99, body.Price, 0.001)
	assert.True(t, body.Active)
	assert.Equal(t, 42, body.Count)
}

func TestRequest_setFieldValue_duration(t *testing.T) {
	t.Parallel()

	type Req struct {
		Timeout time.Duration `query:"timeout"`
	}
	type Resp struct {
		TimeoutNs int64 `json:"timeout_ns"`
	}

	r := api.New()
	api.Get(r, "/duration", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{TimeoutNs: int64(req.Timeout)}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		srv.URL+"/duration?timeout=5s", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body Resp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, int64(5*time.Second), body.TimeoutNs)
}

func TestRequest_setFieldValue_invalid_int(t *testing.T) {
	t.Parallel()

	type Req struct {
		Count int `query:"count"`
	}

	r := api.New()
	api.Get(r, "/bad-int", func(_ context.Context, _ *Req) (*api.Void, error) {
		return &api.Void{}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		srv.URL+"/bad-int?count=notanumber", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRequest_setFieldValue_invalid_float(t *testing.T) {
	t.Parallel()

	type Req struct {
		Price float64 `query:"price"`
	}

	r := api.New()
	api.Get(r, "/bad-float", func(_ context.Context, _ *Req) (*api.Void, error) {
		return &api.Void{}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		srv.URL+"/bad-float?price=notafloat", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRequest_setFieldValue_invalid_bool(t *testing.T) {
	t.Parallel()

	type Req struct {
		Active bool `query:"active"`
	}

	r := api.New()
	api.Get(r, "/bad-bool", func(_ context.Context, _ *Req) (*api.Void, error) {
		return &api.Void{}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		srv.URL+"/bad-bool?active=notabool", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRequest_setFieldValue_invalid_duration(t *testing.T) {
	t.Parallel()

	type Req struct {
		Timeout time.Duration `query:"timeout"`
	}

	r := api.New()
	api.Get(r, "/bad-dur", func(_ context.Context, _ *Req) (*api.Void, error) {
		return &api.Void{}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		srv.URL+"/bad-dur?timeout=notaduration", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRequest_decodeBody_nil_body(t *testing.T) {
	t.Parallel()

	type Req struct {
		Name string `json:"name"`
	}
	type Resp struct {
		Name string `json:"name"`
	}

	r := api.New()
	api.Post(r, "/nil-body", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{Name: req.Name}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/nil-body", nil)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = 0

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	// Should succeed with empty body (no JSON decode error).
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRequest_decodeBody_empty_body(t *testing.T) {
	t.Parallel()

	type Req struct {
		Name string `json:"name"`
	}
	type Resp struct {
		Name string `json:"name"`
	}

	r := api.New()
	api.Post(r, "/empty-body", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{Name: req.Name}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	// Send request with empty body (EOF).
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/empty-body",
		strings.NewReader(""))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRequest_decodeBody_invalid_json(t *testing.T) {
	t.Parallel()

	type Req struct {
		Name string `json:"name"`
	}

	r := api.New()
	api.Post(r, "/bad-json", func(_ context.Context, _ *Req) (*api.Void, error) {
		return &api.Void{}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/bad-json",
		strings.NewReader("{invalid json"))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRequest_setFieldValue_unsupported_type(t *testing.T) {
	t.Parallel()

	type Req struct {
		Data uint `query:"data"`
	}

	r := api.New()
	api.Get(r, "/unsupported", func(_ context.Context, _ *Req) (*api.Void, error) {
		return &api.Void{}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		srv.URL+"/unsupported?data=42", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	// uint is not supported by setFieldValue, should get 400.
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRequest_params_only_no_body(t *testing.T) {
	t.Parallel()

	type Req struct {
		ID   string `path:"id"`
		Lang string `query:"lang" default:"en"`
	}
	type Resp struct {
		ID   string `json:"id"`
		Lang string `json:"lang"`
	}

	r := api.New()
	api.Get(r, "/items/{id}", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{ID: req.ID, Lang: req.Lang}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/items/xyz", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body Resp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "xyz", body.ID)
	assert.Equal(t, "en", body.Lang)
}

func TestRequest_body_only_nil_body(t *testing.T) {
	t.Parallel()

	type Req struct {
		Name string `json:"name"`
	}
	type Resp struct {
		Name string `json:"name"`
	}

	r := api.New()
	api.Post(r, "/nilbody", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{Name: req.Name}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	// Create a POST request with nil body to trigger r.Body == nil path.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/nilbody", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRequest_mixed_body_nil(t *testing.T) {
	t.Parallel()

	type Req struct {
		ID   string `path:"id"`
		Body struct {
			Name string `json:"name"`
		}
	}
	type Resp struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	r := api.New()
	api.Post(r, "/mixed-nil/{id}", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{ID: req.ID, Name: req.Body.Name}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	// POST with path param but no body.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/mixed-nil/abc", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body Resp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "abc", body.ID)
}

func TestRequest_path_binding_error(t *testing.T) {
	t.Parallel()

	type Req struct {
		ID int `path:"id"`
	}

	r := api.New()
	api.Get(r, "/items/{id}", func(_ context.Context, req *Req) (*api.Void, error) {
		return &api.Void{}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/items/notanint", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRequest_header_binding_error(t *testing.T) {
	t.Parallel()

	type Req struct {
		Count int `header:"X-Count"`
	}

	r := api.New()
	api.Get(r, "/header-err", func(_ context.Context, req *Req) (*api.Void, error) {
		return &api.Void{}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/header-err", nil)
	require.NoError(t, err)
	req.Header.Set("X-Count", "notanint")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRequest_cookie_binding_error(t *testing.T) {
	t.Parallel()

	type Req struct {
		Count int `cookie:"count"`
	}

	r := api.New()
	api.Get(r, "/cookie-err", func(_ context.Context, req *Req) (*api.Void, error) {
		return &api.Void{}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/cookie-err", nil)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: "count", Value: "notanint"})

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRequest_body_only_with_nil_http_body(t *testing.T) {
	t.Parallel()

	type Req struct {
		Name string `json:"name"`
	}
	type Resp struct {
		Name string `json:"name"`
	}

	r := api.New()
	api.Post(r, "/nilhttpbody", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{Name: req.Name}, nil
	})

	// Directly call ServeHTTP with a request that has nil Body but non-zero ContentLength.
	rec := httptest.NewRecorder()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/nilhttpbody", nil)
	require.NoError(t, err)
	req.Body = nil          // explicitly nil
	req.ContentLength = 100 // pretend there's content to avoid ContentLength==0 shortcut

	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

type reqWithUnexported struct {
	ID       string `path:"id"`
	internal string //nolint:unused
}

func TestRequest_bindParams_skips_unexported(t *testing.T) {
	t.Parallel()

	type Resp struct {
		ID string `json:"id"`
	}

	r := api.New()
	api.Get(r, "/unexported/{id}", func(_ context.Context, req *reqWithUnexported) (*Resp, error) {
		return &Resp{ID: req.ID}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/unexported/abc", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body Resp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "abc", body.ID)
}

func TestRequest_decodeBody_eof_body(t *testing.T) {
	t.Parallel()

	type Req struct {
		Name string `json:"name"`
	}
	type Resp struct {
		Name string `json:"name"`
	}

	r := api.New()
	api.Post(r, "/eof-body", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{Name: req.Name}, nil
	})

	// Create a request with an empty body but non-zero ContentLength to hit the EOF path.
	rec := httptest.NewRecorder()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/eof-body",
		strings.NewReader(""))
	require.NoError(t, err)
	req.ContentLength = -1 // unknown length, forces json.Decode to read, which gets EOF

	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequest_mixed_body_invalid_json(t *testing.T) {
	t.Parallel()

	type Req struct {
		ID   string `path:"id"`
		Body struct {
			Name string `json:"name"`
		}
	}

	r := api.New()
	api.Post(r, "/mixed-badjson/{id}", func(_ context.Context, req *Req) (*api.Void, error) {
		return &api.Void{}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/mixed-badjson/abc",
		strings.NewReader("{invalid"))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
