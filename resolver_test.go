package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjaus/api"
)

// --- resolver request types ---

type resolverFieldErrors struct {
	Body struct {
		Email string `json:"email"`
	}
}

func (r *resolverFieldErrors) Resolve(_ context.Context, _ *http.Request) []error {
	return []error{
		&api.ValidationError{Field: "body.email", Message: "invalid email format", Value: r.Body.Email},
	}
}

type resolverNil struct {
	Body struct {
		Name string `json:"name"`
	}
}

func (r *resolverNil) Resolve(_ context.Context, _ *http.Request) []error {
	return nil
}

type resolverEmpty struct {
	Body struct {
		Name string `json:"name"`
	}
}

func (r *resolverEmpty) Resolve(_ context.Context, _ *http.Request) []error {
	return []error{}
}

type resolverMixed struct {
	Body struct {
		Name string `json:"name"`
	}
}

func (r *resolverMixed) Resolve(_ context.Context, _ *http.Request) []error {
	return []error{
		&api.ValidationError{Field: "body.name", Message: "too short"},
		fmt.Errorf("something went wrong"),
	}
}

type resolverTransform struct {
	Body struct {
		Name string `json:"name" minLength:"3"`
	}
}

func (r *resolverTransform) Resolve(_ context.Context, _ *http.Request) []error {
	r.Body.Name = strings.TrimSpace(r.Body.Name)
	return nil
}

type resolverReadsHeader struct {
	Body struct {
		Value string `json:"value"`
	}
}

func (r *resolverReadsHeader) Resolve(_ context.Context, req *http.Request) []error {
	if req.Header.Get("X-Tenant") == "" {
		return []error{
			&api.ValidationError{Field: "X-Tenant", Message: "tenant header required"},
		}
	}
	return nil
}

type resolverMultipleErrors struct {
	Body struct {
		A string `json:"a"`
		B string `json:"b"`
		C string `json:"c"`
	}
}

func (r *resolverMultipleErrors) Resolve(_ context.Context, _ *http.Request) []error {
	return []error{
		&api.ValidationError{Field: "body.a", Message: "a is bad"},
		&api.ValidationError{Field: "body.b", Message: "b is bad"},
		&api.ValidationError{Field: "body.c", Message: "c is bad"},
	}
}

// --- tests ---

func TestResolver_FieldErrors(t *testing.T) {
	t.Parallel()

	type Resp struct {
		OK bool `json:"ok"`
	}

	r := api.New()
	api.Post(r, "/check", func(_ context.Context, _ *resolverFieldErrors) (*Resp, error) {
		return &Resp{OK: true}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/check", strings.NewReader(`{"email":"bad"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var pd api.ProblemDetail
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&pd))
	assert.Equal(t, "Validation Failed", pd.Title)
	require.Len(t, pd.Errors, 1)
	assert.Equal(t, "body.email", pd.Errors[0].Field)
	assert.Equal(t, "invalid email format", pd.Errors[0].Message)
}

func TestResolver_Nil(t *testing.T) {
	t.Parallel()

	type Resp struct {
		Name string `json:"name"`
	}

	r := api.New()
	api.Post(r, "/ok", func(_ context.Context, req *resolverNil) (*Resp, error) {
		return &Resp{Name: req.Body.Name}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/ok", strings.NewReader(`{"name":"Alice"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body Resp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "Alice", body.Name)
}

func TestResolver_EmptySlice(t *testing.T) {
	t.Parallel()

	type Resp struct {
		Name string `json:"name"`
	}

	r := api.New()
	api.Post(r, "/ok", func(_ context.Context, req *resolverEmpty) (*Resp, error) {
		return &Resp{Name: req.Body.Name}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/ok", strings.NewReader(`{"name":"Bob"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestResolver_MixedErrors(t *testing.T) {
	t.Parallel()

	type Resp struct {
		OK bool `json:"ok"`
	}

	r := api.New()
	api.Post(r, "/mix", func(_ context.Context, _ *resolverMixed) (*Resp, error) {
		return &Resp{OK: true}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/mix", strings.NewReader(`{"name":"x"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var pd api.ProblemDetail
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&pd))
	require.Len(t, pd.Errors, 2)

	assert.Equal(t, "body.name", pd.Errors[0].Field)
	assert.Equal(t, "too short", pd.Errors[0].Message)

	assert.Equal(t, "", pd.Errors[1].Field)
	assert.Equal(t, "something went wrong", pd.Errors[1].Message)
}

func TestResolver_TransformBeforeConstraints(t *testing.T) {
	t.Parallel()

	type Resp struct {
		Name string `json:"name"`
	}

	r := api.New()
	api.Post(r, "/trim", func(_ context.Context, req *resolverTransform) (*Resp, error) {
		return &Resp{Name: req.Body.Name}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	// "  abc  " trimmed to "abc" (len 3) — passes minLength:3 constraint.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/trim", strings.NewReader(`{"name":"  abc  "}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body Resp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "abc", body.Name)
}

func TestResolver_NotImplemented(t *testing.T) {
	t.Parallel()

	type Req struct {
		Body struct {
			Name string `json:"name"`
		}
	}
	type Resp struct {
		Name string `json:"name"`
	}

	r := api.New()
	api.Post(r, "/plain", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{Name: req.Body.Name}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/plain", strings.NewReader(`{"name":"Eve"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestResolver_AccessContextAndRequest(t *testing.T) {
	t.Parallel()

	type Resp struct {
		Value string `json:"value"`
	}

	r := api.New()
	api.Post(r, "/tenant", func(_ context.Context, req *resolverReadsHeader) (*Resp, error) {
		return &Resp{Value: req.Body.Value}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	tests := map[string]struct {
		tenant     string
		wantStatus int
	}{
		"with tenant header": {
			tenant:     "acme",
			wantStatus: http.StatusOK,
		},
		"missing tenant header": {
			tenant:     "",
			wantStatus: http.StatusBadRequest,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/tenant", strings.NewReader(`{"value":"hello"}`))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			if tc.tenant != "" {
				req.Header.Set("X-Tenant", tc.tenant)
			}

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			assert.Equal(t, tc.wantStatus, resp.StatusCode)

			if tc.wantStatus == http.StatusBadRequest {
				var pd api.ProblemDetail
				require.NoError(t, json.NewDecoder(resp.Body).Decode(&pd))
				require.Len(t, pd.Errors, 1)
				assert.Equal(t, "X-Tenant", pd.Errors[0].Field)
			}
		})
	}
}

func TestResolver_MultipleErrors(t *testing.T) {
	t.Parallel()

	type Resp struct {
		OK bool `json:"ok"`
	}

	r := api.New()
	api.Post(r, "/multi", func(_ context.Context, _ *resolverMultipleErrors) (*Resp, error) {
		return &Resp{OK: true}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/multi", strings.NewReader(`{"a":"","b":"","c":""}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var pd api.ProblemDetail
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&pd))
	assert.Equal(t, "3 validation error(s)", pd.Detail)
	require.Len(t, pd.Errors, 3)
	assert.Equal(t, "body.a", pd.Errors[0].Field)
	assert.Equal(t, "body.b", pd.Errors[1].Field)
	assert.Equal(t, "body.c", pd.Errors[2].Field)
}

func TestResolveRequest_Unit(t *testing.T) {
	t.Parallel()

	t.Run("not a resolver", func(t *testing.T) {
		t.Parallel()

		type plain struct{ Name string }
		req := &plain{Name: "test"}
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		err := api.ResolveRequest(r.Context(), req, r)
		require.NoError(t, err)
	})

	t.Run("resolver returns nil", func(t *testing.T) {
		t.Parallel()

		req := &resolverNil{}
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		err := api.ResolveRequest(r.Context(), req, r)
		require.NoError(t, err)
	})

	t.Run("resolver returns errors", func(t *testing.T) {
		t.Parallel()

		req := &resolverFieldErrors{}
		req.Body.Email = "bad"
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		err := api.ResolveRequest(r.Context(), req, r)
		require.Error(t, err)

		var pd *api.ProblemDetail
		require.True(t, errors.As(err, &pd))
		assert.Equal(t, http.StatusBadRequest, pd.Status)
		require.Len(t, pd.Errors, 1)
		assert.Equal(t, "body.email", pd.Errors[0].Field)
	})

	t.Run("mixed validation and plain errors", func(t *testing.T) {
		t.Parallel()

		req := &resolverMixed{}
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		err := api.ResolveRequest(r.Context(), req, r)
		require.Error(t, err)

		var pd *api.ProblemDetail
		require.True(t, errors.As(err, &pd))
		require.Len(t, pd.Errors, 2)
		assert.Equal(t, "body.name", pd.Errors[0].Field)
		assert.Equal(t, "", pd.Errors[1].Field)
		assert.Equal(t, "something went wrong", pd.Errors[1].Message)
	})
}
