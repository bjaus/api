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

func TestRegister_all_methods(t *testing.T) {
	t.Parallel()

	type Resp struct {
		Method string `json:"method"`
	}

	handler := func(method string) api.Handler[api.Void, api.Resp[Resp]] {
		return func(_ context.Context, _ *api.Void) (*api.Resp[Resp], error) {
			return &api.Resp[Resp]{Body: Resp{Method: method}}, nil
		}
	}

	tests := map[string]struct {
		register func(reg api.Registrar)
		method   string
	}{
		"GET": {
			register: func(reg api.Registrar) {
				api.Get(reg, "/test", handler("GET"))
			},
			method: http.MethodGet,
		},
		"POST": {
			register: func(reg api.Registrar) {
				api.Post(reg, "/test", handler("POST"))
			},
			method: http.MethodPost,
		},
		"PUT": {
			register: func(reg api.Registrar) {
				api.Put(reg, "/test", handler("PUT"))
			},
			method: http.MethodPut,
		},
		"PATCH": {
			register: func(reg api.Registrar) {
				api.Patch(reg, "/test", handler("PATCH"))
			},
			method: http.MethodPatch,
		},
		"DELETE": {
			register: func(reg api.Registrar) {
				api.Delete(reg, "/test", handler("DELETE"))
			},
			method: http.MethodDelete,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r := api.New()
			tc.register(r)

			srv := httptest.NewServer(r)
			defer srv.Close()

			req, err := http.NewRequestWithContext(context.Background(), tc.method, srv.URL+"/test", nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var body Resp
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
			assert.Equal(t, tc.method, body.Method)
		})
	}
}

func TestRegister_WithStatus(t *testing.T) {
	t.Parallel()

	type Resp struct {
		ID string `json:"id"`
	}

	r := api.New()
	api.Post(r, "/items", func(_ context.Context, _ *api.Void) (*api.Resp[Resp], error) {
		return &api.Resp[Resp]{Body: Resp{ID: "123"}}, nil
	}, api.WithStatus(http.StatusCreated))

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/items", strings.NewReader(`{}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestRegister_Void_response_204(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Delete(r, "/items/{id}", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, srv.URL+"/items/123", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestRegister_Raw(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Raw(r, http.MethodGet, "/ws", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Raw", "true")
		w.WriteHeader(http.StatusOK)
	}, api.OperationInfo{
		Summary: "WebSocket endpoint",
		Tags:    []string{"ws"},
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/ws", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "true", resp.Header.Get("X-Raw"))
}

func TestBuildHandler_constraint_validation_failure(t *testing.T) {
	t.Parallel()

	type Req struct {
		Body struct {
			Name string `json:"name" minLength:"5"`
		}
	}
	type Resp struct {
		OK bool `json:"ok"`
	}

	r := api.New()
	api.Post(r, "/validate", func(_ context.Context, req *Req) (*api.Resp[Resp], error) {
		_ = req
		return &api.Resp[Resp]{Body: Resp{OK: true}}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/validate", strings.NewReader(`{"name":"ab"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "application/problem+json", resp.Header.Get("Content-Type"))

	var body api.ProblemDetail
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "Validation Failed", body.Title)
	require.Len(t, body.Errors, 1)
	assert.Equal(t, "body.name", body.Errors[0].Field)
}

func failValidator(_ any) error {
	return api.Error(http.StatusUnprocessableEntity, "global validator rejected")
}

func TestBuildHandler_global_validator_failure(t *testing.T) {
	t.Parallel()

	type Resp struct {
		OK bool `json:"ok"`
	}

	r := api.New(api.WithValidator(failValidator))
	api.Post(r, "/check", func(_ context.Context, _ *api.Void) (*api.Resp[Resp], error) {
		return &api.Resp[Resp]{Body: Resp{OK: true}}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/check", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestRegister_panicsOnNonStructResponseType(t *testing.T) {
	t.Parallel()

	r := api.New()

	assert.Panics(t, func() {
		api.Get(r, "/bad", func(_ context.Context, _ *api.Void) (*int, error) {
			n := 0
			return &n, nil
		})
	}, "registering a handler with a non-struct response type must panic")
}

func TestRaw_on_group_with_middleware(t *testing.T) {
	t.Parallel()

	r := api.New()

	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Group-MW", "applied")
			next.ServeHTTP(w, req)
		})
	}

	g := r.Group("/api", api.WithGroupMiddleware(mw))
	api.Raw(g, http.MethodGet, "/raw", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, api.OperationInfo{Summary: "Raw endpoint"})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/api/raw", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "applied", resp.Header.Get("X-Group-MW"))
}
