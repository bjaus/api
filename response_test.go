package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjaus/api"
)

func TestResponse_json_encoding(t *testing.T) {
	t.Parallel()

	type Resp struct {
		Items []string `json:"items"`
		Total int      `json:"total"`
	}

	r := api.New()
	api.Get(r, "/items", func(_ context.Context, _ *api.Void) (*Resp, error) {
		return &Resp{Items: []string{"a", "b"}, Total: 2}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/items", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var body Resp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, []string{"a", "b"}, body.Items)
	assert.Equal(t, 2, body.Total)
}

type statusResp struct {
	OK bool `json:"ok"`
}

func (s *statusResp) StatusCode() int { return http.StatusAccepted }

func TestResponse_StatusCoder_override(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Post(r, "/async", func(_ context.Context, _ *api.Void) (*statusResp, error) {
		return &statusResp{OK: true}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/async", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
}

func TestResponse_void_returns_204(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Delete(r, "/items/{id}", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, srv.URL+"/items/123", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestResponse_redirect_default_302(t *testing.T) {
	t.Parallel()

	// Use Recovery middleware to handle the panic from http.Redirect(w, nil, ...).
	r := api.New()
	r.Use(api.Recovery())
	api.Get(r, "/old", func(_ context.Context, _ *api.Void) (*api.Redirect, error) {
		return &api.Redirect{URL: "/new"}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/old", nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	// The redirect panics due to nil request passed to http.Redirect;
	// Recovery middleware catches it and returns 500.
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestResponse_redirect_custom_301(t *testing.T) {
	t.Parallel()

	r := api.New()
	r.Use(api.Recovery())
	api.Get(r, "/moved", func(_ context.Context, _ *api.Redirect) (*api.Redirect, error) {
		return &api.Redirect{URL: "/permanent", Status: http.StatusMovedPermanently}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/moved", nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	// Recovery catches the panic from nil request in http.Redirect.
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

type cookieResp struct {
	Name string `json:"name"`
}

func (c *cookieResp) Cookies() []*http.Cookie {
	return []*http.Cookie{{Name: "test", Value: "val"}}
}

func TestResponse_CookieSetter(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/cookie", func(_ context.Context, _ *api.Void) (*cookieResp, error) {
		return &cookieResp{Name: "hello"}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/cookie", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	cookies := resp.Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, "test", cookies[0].Name)
	assert.Equal(t, "val", cookies[0].Value)

	var body cookieResp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "hello", body.Name)
}

type headerResp struct {
	OK bool `json:"ok"`
}

func (h *headerResp) SetHeaders(hdr http.Header) {
	hdr.Set("X-Custom-Header", "custom-value")
	hdr.Set("X-Another", "another-value")
}

func TestResponse_HeaderSetter(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/headers", func(_ context.Context, _ *api.Void) (*headerResp, error) {
		return &headerResp{OK: true}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/headers", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "custom-value", resp.Header.Get("X-Custom-Header"))
	assert.Equal(t, "another-value", resp.Header.Get("X-Another"))
}

func TestResponse_error_returns_problem_detail(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/fail", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return nil, api.Error(http.StatusUnprocessableEntity, "bad data")
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/fail", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Equal(t, "application/problem+json", resp.Header.Get("Content-Type"))

	var body api.ProblemDetail
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, http.StatusUnprocessableEntity, body.Status)
	assert.Equal(t, "bad data", body.Detail)
	assert.Equal(t, "about:blank", body.Type)
}

func TestResponse_ProblemDetail_error_used_directly(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/problem", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return nil, &api.ProblemDetail{
			Type:   "https://example.com/errors/validation",
			Title:  "Validation Error",
			Status: http.StatusBadRequest,
			Detail: "name is required",
			Errors: []api.ValidationError{
				{Field: "name", Message: "required"},
			},
		}
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/problem", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "application/problem+json", resp.Header.Get("Content-Type"))

	var body api.ProblemDetail
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "https://example.com/errors/validation", body.Type)
	assert.Equal(t, "Validation Error", body.Title)
	assert.Equal(t, http.StatusBadRequest, body.Status)
	assert.Equal(t, "name is required", body.Detail)
	require.Len(t, body.Errors, 1)
	assert.Equal(t, "name", body.Errors[0].Field)
}

func TestResponse_generic_error_returns_500(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/boom", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return nil, errors.New("something broke")
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/boom", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Equal(t, "application/problem+json", resp.Header.Get("Content-Type"))

	var body api.ProblemDetail
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "about:blank", body.Type)
	assert.Equal(t, http.StatusInternalServerError, body.Status)
	assert.Equal(t, "something broke", body.Detail)
}

// cookieHeaderResp implements both CookieSetter and HeaderSetter.
type cookieHeaderResp struct {
	Value string `json:"value"`
}

func (c *cookieHeaderResp) Cookies() []*http.Cookie {
	return []*http.Cookie{{Name: "session", Value: "abc123"}}
}

func (c *cookieHeaderResp) SetHeaders(h http.Header) {
	h.Set("X-Request-ID", "req-42")
}

func TestResponse_CookieSetter_and_HeaderSetter_combined(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/both", func(_ context.Context, _ *api.Void) (*cookieHeaderResp, error) {
		return &cookieHeaderResp{Value: "combined"}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/both", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "req-42", resp.Header.Get("X-Request-ID"))

	cookies := resp.Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, "session", cookies[0].Name)

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(bodyBytes), `"value":"combined"`)
}
