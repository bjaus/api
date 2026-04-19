package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjaus/api"
)

type validatedReq struct {
	Body struct {
		Name string `json:"name"`
	}
}

func (r *validatedReq) Validate(_ context.Context) error {
	if r.Body.Name == "" {
		return api.Error(http.StatusBadRequest, "name required")
	}
	return nil
}

func TestValidator_perType(t *testing.T) {
	t.Parallel()

	type Resp struct {
		Name string `json:"name"`
	}

	r := api.New()
	api.Post(r, "/users", func(_ context.Context, req *validatedReq) (*Resp, error) {
		return &Resp{Name: req.Body.Name}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	tests := map[string]struct {
		body       string
		wantStatus int
	}{
		"valid":                 {`{"name":"Alice"}`, http.StatusOK},
		"invalid - empty name":  {`{"name":""}`, http.StatusBadRequest},
		"invalid - missing":     {`{}`, http.StatusBadRequest},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/users", strings.NewReader(tc.body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			assert.Equal(t, tc.wantStatus, resp.StatusCode)
		})
	}
}

type ctxValidatedReq struct {
	Body struct {
		Value string `json:"value"`
	}
}

type tenantKey struct{}

func (r *ctxValidatedReq) Validate(ctx context.Context) error {
	if ctx.Value(tenantKey{}) == nil {
		return api.ValidationErrors{
			{Field: "ctx.tenant", Message: "tenant missing from context"},
		}
	}
	return nil
}

func TestValidator_usesContext(t *testing.T) {
	t.Parallel()

	type Resp struct {
		OK bool `json:"ok"`
	}

	r := api.New()

	// Middleware that stashes a tenant in context when header is present.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if t := req.Header.Get("X-Tenant"); t != "" {
				req = req.WithContext(context.WithValue(req.Context(), tenantKey{}, t))
			}
			next.ServeHTTP(w, req)
		})
	})

	api.Post(r, "/ctx", func(_ context.Context, _ *ctxValidatedReq) (*Resp, error) {
		return &Resp{OK: true}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	tests := map[string]struct {
		tenant     string
		wantStatus int
	}{
		"with tenant":    {"acme", http.StatusOK},
		"missing tenant": {"", http.StatusBadRequest},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/ctx", strings.NewReader(`{"value":"x"}`))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			if tc.tenant != "" {
				req.Header.Set("X-Tenant", tc.tenant)
			}

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			assert.Equal(t, tc.wantStatus, resp.StatusCode)
		})
	}
}

type nonValidationErrReq struct {
	Body struct {
		Name string `json:"name"`
	}
}

func (r *nonValidationErrReq) Validate(_ context.Context) error {
	return api.Error(http.StatusUnauthorized, "login required")
}

func TestValidator_nonValidationErrorForwarded(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Post(r, "/auth", func(_ context.Context, _ *nonValidationErrReq) (*api.Void, error) {
		return &api.Void{}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/auth", strings.NewReader(`{"name":"x"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestRouterValidator_runs(t *testing.T) {
	t.Parallel()

	type Req struct {
		Name string `json:"name"`
	}
	type Resp struct {
		Name string `json:"name"`
	}

	called := false
	r := api.New(api.WithValidator(func(_ any) error {
		called = true
		return nil
	}))
	api.Post(r, "/users", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{Name: req.Name}, nil
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/users", strings.NewReader(`{"name":"Bob"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.True(t, called, "router validator should have been invoked")
}

func TestRouterValidator_returnsValidationErrors(t *testing.T) {
	t.Parallel()

	type Req struct {
		Name string `json:"name"`
	}
	type Resp struct {
		OK bool `json:"ok"`
	}

	r := api.New(api.WithValidator(func(_ any) error {
		return api.ValidationErrors{
			{Field: "name", Message: "router says no"},
		}
	}))
	api.Post(r, "/x", func(_ context.Context, _ *Req) (*Resp, error) {
		return &Resp{OK: true}, nil
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/x", strings.NewReader(`{"name":"x"}`))
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
	assert.Equal(t, "name", pd.Errors[0].Field)
}

func TestValidationErrors_Error(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		ve   api.ValidationErrors
		want string
	}{
		"empty":    {api.ValidationErrors{}, "0 validation error(s)"},
		"single":   {api.ValidationErrors{{Field: "a"}}, "1 validation error(s)"},
		"multiple": {api.ValidationErrors{{Field: "a"}, {Field: "b"}, {Field: "c"}}, "3 validation error(s)"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.ve.Error())
		})
	}
}

// --- ValidationMode tests ---

type modeReq struct {
	Body struct {
		Name string `json:"name" minLength:"5"`
	}
}

func (r *modeReq) Validate(_ context.Context) error {
	return api.ValidationErrors{
		{Field: "body.name", Message: "from Validator"},
	}
}

func TestValidationMode_Last_PerTypeFirst(t *testing.T) {
	t.Parallel()

	r := api.New() // default: Last
	api.Post(r, "/m", func(_ context.Context, _ *modeReq) (*api.Void, error) {
		return &api.Void{}, nil
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/m", strings.NewReader(`{"name":"ab"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	var pd api.ProblemDetail
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&pd))
	require.Len(t, pd.Errors, 1)
	assert.Equal(t, "from Validator", pd.Errors[0].Message, "per-type Validator runs first in Last mode")
}

func TestValidationMode_First_ConstraintsFirst(t *testing.T) {
	t.Parallel()

	r := api.New(api.WithValidationMode(api.ValidateConstraintsFirst))
	api.Post(r, "/m", func(_ context.Context, _ *modeReq) (*api.Void, error) {
		return &api.Void{}, nil
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/m", strings.NewReader(`{"name":"ab"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	var pd api.ProblemDetail
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&pd))
	require.Len(t, pd.Errors, 1)
	assert.Contains(t, pd.Errors[0].Message, "at least 5 characters", "constraint fires first in First mode")
}

type offReq struct {
	Body struct {
		Name string `json:"name" minLength:"5"`
	}
}

func TestValidationMode_Off_NoConstraintRuntime(t *testing.T) {
	t.Parallel()

	type Resp struct {
		Name string `json:"name"`
	}

	r := api.New(api.WithValidationMode(api.ValidateConstraintsOff))
	api.Post(r, "/m", func(_ context.Context, req *offReq) (*Resp, error) {
		return &Resp{Name: req.Body.Name}, nil
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/m", strings.NewReader(`{"name":"ab"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "constraint tags must not fire at runtime in Off mode")
}

func TestValidationMode_Off_SpecStillHasConstraints(t *testing.T) {
	t.Parallel()

	r := api.New(
		api.WithTitle("Off Mode Spec"),
		api.WithValidationMode(api.ValidateConstraintsOff),
	)
	api.Post(r, "/m", func(_ context.Context, _ *offReq) (*api.Void, error) {
		return &api.Void{}, nil
	})

	spec := r.Spec()
	require.NotNil(t, spec)

	// Find the minLength in the generated schema — specifics of the structure
	// are framework-internal; we just assert the tag reached the spec.
	raw, err := json.Marshal(spec)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"minLength":5`, "OpenAPI schema must still include minLength in Off mode")
}

func TestValidationMode_PerRouteOverride(t *testing.T) {
	t.Parallel()

	r := api.New(api.WithValidationMode(api.ValidateConstraintsFirst))
	api.Post(r, "/override",
		func(_ context.Context, req *offReq) (*api.Void, error) {
			_ = req
			return &api.Void{}, nil
		},
		api.WithMode(api.ValidateConstraintsOff),
	)

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/override", strings.NewReader(`{"name":"ab"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

// --- ValidationErrorBuilder tests ---

type domainError struct {
	Code   string                  `json:"code"`
	Msg    string                  `json:"message"`
	Fields []api.ValidationError   `json:"fields,omitempty"`
}

func (e *domainError) Error() string   { return e.Msg }
func (e *domainError) StatusCode() int { return http.StatusUnprocessableEntity }

type domainBuilder struct{}

func (domainBuilder) Build(v api.ValidationErrors) error {
	return &domainError{Code: "INVALID_INPUT", Msg: "bad request", Fields: v}
}

func TestValidationErrorBuilder_constraintPath(t *testing.T) {
	t.Parallel()

	r := api.New(
		api.WithValidationErrorBuilder(domainBuilder{}),
		api.WithErrorHandler(func(w http.ResponseWriter, _ *http.Request, err error) {
			var de *domainError
			if errors.As(err, &de) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(de.StatusCode())
				assert.NoError(t, json.NewEncoder(w).Encode(de))
				return
			}
			assert.Fail(t, "unexpected error type", "got %T: %v", err, err)
		}),
	)
	api.Post(r, "/b",
		func(_ context.Context, _ *offReq) (*api.Void, error) {
			return &api.Void{}, nil
		},
		api.WithMode(api.ValidateConstraintsFirst),
	)

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/b", strings.NewReader(`{"name":"ab"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	var de domainError
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&de))
	assert.Equal(t, "INVALID_INPUT", de.Code)
	require.Len(t, de.Fields, 1)
	assert.Equal(t, "body.name", de.Fields[0].Field)
}

func TestValidationErrorBuilder_perTypePath(t *testing.T) {
	t.Parallel()

	r := api.New(
		api.WithValidationErrorBuilder(domainBuilder{}),
		api.WithErrorHandler(func(w http.ResponseWriter, _ *http.Request, err error) {
			var de *domainError
			if errors.As(err, &de) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(de.StatusCode())
				assert.NoError(t, json.NewEncoder(w).Encode(de))
				return
			}
			assert.Fail(t, "unexpected error type", "got %T: %v", err, err)
		}),
	)
	api.Post(r, "/b", func(_ context.Context, _ *modeReq) (*api.Void, error) {
		return &api.Void{}, nil
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/b", strings.NewReader(`{"name":"abcdef"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	var de domainError
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&de))
	assert.Equal(t, "INVALID_INPUT", de.Code)
	require.Len(t, de.Fields, 1)
	assert.Equal(t, "from Validator", de.Fields[0].Message)
}
