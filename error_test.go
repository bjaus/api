package api_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjaus/api"
)

func TestCode_HTTPStatus(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		code api.Code
		want int
	}{
		"not found":              {api.CodeNotFound, http.StatusNotFound},
		"teapot":                 {api.CodeTeapot, http.StatusTeapot},
		"too many requests":      {api.CodeTooManyRequests, http.StatusTooManyRequests},
		"internal":               {api.CodeInternal, http.StatusInternalServerError},
		"unknown falls back 500": {api.Code("not_real"), http.StatusInternalServerError},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.code.HTTPStatus())
		})
	}
}

func TestCode_IsRegistered(t *testing.T) {
	t.Parallel()

	assert.True(t, api.CodeNotFound.IsRegistered())
	assert.True(t, api.CodeTeapot.IsRegistered())
	assert.True(t, api.CodeNotExtended.IsRegistered())
	assert.False(t, api.Code("fabricated").IsRegistered())
	assert.False(t, api.Code("").IsRegistered())
}

// --- Inline options on api.Error ---

func TestError_inlineHeader(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/rl", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return nil, api.Error(api.CodeTooManyRequests,
			api.WithHeader("Retry-After", "60"),
		)
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/rl", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.Equal(t, "60", resp.Header.Get("Retry-After"))
}

func TestError_inlineCookie(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/logout", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return nil, api.Error(api.CodeUnauthorized,
			api.WithCookie("session", api.Cookie{MaxAge: -1}),
		)
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/logout", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	cookies := resp.Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, "session", cookies[0].Name)
	assert.Equal(t, -1, cookies[0].MaxAge)
}

func TestError_inlineDetailWithEnvelope(t *testing.T) {
	t.Parallel()

	r := api.New(api.WithError(api.WithErrorBody(api.ErrorBodyProblemDetails)))

	type fieldErr struct {
		Field string `json:"field"`
		Msg   string `json:"msg"`
	}

	api.Post(r, "/x", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return nil, api.Error(api.CodeBadRequest,
			api.WithMessage("validation failed"),
			api.WithDetail(fieldErr{Field: "email", Msg: "invalid"}),
			api.WithDetail(fieldErr{Field: "age", Msg: "too young"}),
		)
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/x", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	var env api.ProblemDetails
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
	assert.Equal(t, "validation failed", env.Detail)
	require.Len(t, env.Errors, 2)
}

// --- Scope merging ---

func TestWithError_routerScopeHeaderApplied(t *testing.T) {
	t.Parallel()

	r := api.New(api.WithError(api.WithHeader("X-API-Version", "v1")))
	api.Get(r, "/fail", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return nil, api.Error(api.CodeNotFound)
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/fail", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, "v1", resp.Header.Get("X-API-Version"), "router-scope header should apply")
}

func TestWithError_groupScopeHeaderApplied(t *testing.T) {
	t.Parallel()

	r := api.New()
	g := r.Group("/admin", api.WithError(api.WithHeader("X-Admin", "true")))
	api.Get(g, "/x", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return nil, api.Error(api.CodeForbidden)
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/admin/x", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	assert.Equal(t, "true", resp.Header.Get("X-Admin"))
}

func TestWithError_inlineOverridesScope(t *testing.T) {
	t.Parallel()

	// Router sets X-Scope to "router"; inline sets it to "inline".
	// Inline must win.
	r := api.New(api.WithError(api.WithHeader("X-Scope", "router")))
	api.Get(r, "/fail", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return nil, api.Error(api.CodeBadRequest, api.WithHeader("X-Scope", "inline"))
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/fail", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, "inline", resp.Header.Get("X-Scope"))
}

func TestWithError_detailsAccumulate(t *testing.T) {
	t.Parallel()

	// Router-level detail + route-level detail + inline detail should all appear.
	r := api.New(api.WithError(
		api.WithErrorBody(api.ErrorBodyProblemDetails),
		api.WithDetail("router-level"),
	))
	api.Get(r, "/fail", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return nil, api.Error(api.CodeBadRequest, api.WithDetail("inline"))
	},
		api.WithError(api.WithDetail("route-level")),
	)

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/fail", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	var env api.ProblemDetails
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
	require.Len(t, env.Errors, 3)
	assert.Equal(t, "router-level", env.Errors[0])
	assert.Equal(t, "route-level", env.Errors[1])
	assert.Equal(t, "inline", env.Errors[2])
}

// --- WithErrors documents codes ---

func TestWithErrors_documentedCodes(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Post(r, "/users", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	}, api.WithError(api.WithErrors(api.CodeConflict, api.CodeUnprocessableContent)))

	spec := r.Spec()
	op := spec.Paths["/users"]["post"]
	assert.Contains(t, op.Responses, "409")
	assert.Contains(t, op.Responses, "422")
}

// --- Conditional body (nil return) ---

type condBody struct {
	Code string `json:"code"`
}

func TestWithErrorBody_conditionalNilSkipsBody(t *testing.T) {
	t.Parallel()

	// Body mapper returns nil for 401 (no body), envelope for others.
	r := api.New(api.WithError(api.WithErrorBody(func(_ context.Context, e api.ErrorInfo) *condBody {
		if e.Code() == api.CodeUnauthorized {
			return nil
		}
		return &condBody{Code: string(e.Code())}
	})))

	api.Get(r, "/unauth", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return nil, api.Error(api.CodeUnauthorized)
	})
	api.Get(r, "/notfound", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return nil, api.Error(api.CodeNotFound)
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	// 401 — no body
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/unauth", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Empty(t, body)

	// 404 — body present
	req, err = http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/notfound", nil)
	require.NoError(t, err)
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Contains(t, string(body), "not_found")
}

// --- Scope-level cookie merges with inline ---

func TestWithError_scopeCookie(t *testing.T) {
	t.Parallel()

	r := api.New(api.WithError(
		api.WithCookie("scope-cookie", api.Cookie{Value: "scope"}),
	))
	api.Get(r, "/e", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return nil, api.Error(api.CodeForbidden)
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/e", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	cookies := resp.Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, "scope-cookie", cookies[0].Name)
	assert.Equal(t, "scope", cookies[0].Value)
}

// --- Inline body mapper wins over scope body mapper ---

type altBody struct {
	Code string `json:"code"`
}

func TestWithErrorBody_inlineOverridesScope(t *testing.T) {
	t.Parallel()

	// Router says: use envelope body. Inline says: use altBody.
	r := api.New(api.WithError(api.WithErrorBody(api.ErrorBodyProblemDetails)))

	api.Get(r, "/e", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return nil, api.Error(api.CodeBadRequest,
			api.WithErrorBody(func(_ context.Context, e api.ErrorInfo) *altBody {
				return &altBody{Code: "INLINE_" + string(e.Code())}
			}),
		)
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/e", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	var body altBody
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "INLINE_bad_request", body.Code)
}

// --- Plain (non-api) error wraps as CodeInternal ---

func TestError_plainErrorWrappedAsInternal(t *testing.T) {
	t.Parallel()

	plainErr := plainStringError("boom")

	r := api.New(api.WithError(api.WithErrorBody(api.ErrorBodyProblemDetails)))
	api.Get(r, "/plain", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return nil, plainErr
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/plain", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	var env api.ProblemDetails
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
	assert.Equal(t, api.CodeInternal, env.Code)
	assert.Equal(t, "boom", env.Detail)
}

type plainStringError string

func (e plainStringError) Error() string { return string(e) }
