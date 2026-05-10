package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	api.Get(r, "/items", func(_ context.Context, _ *api.Void) (*api.Resp[Resp], error) {
		return &api.Resp[Resp]{Body: Resp{Items: []string{"a", "b"}, Total: 2}}, nil
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

type asyncResp struct {
	Status int `status:""`
	Body   struct {
		OK bool `json:"ok"`
	}
}

func TestResponse_declarative_status_override(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Post(r, "/async", func(_ context.Context, _ *api.Void) (*asyncResp, error) {
		return &asyncResp{
			Status: http.StatusAccepted,
			Body:   struct{ OK bool `json:"ok"` }{OK: true},
		}, nil
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

	r := api.New()
	api.Get(r, "/old", func(_ context.Context, _ *api.Void) (*api.RedirectResp, error) {
		return api.Redirect("/new", 0), nil
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

	assert.Equal(t, http.StatusFound, resp.StatusCode)
	assert.Equal(t, "/new", resp.Header.Get("Location"))
}

func TestResponse_redirect_custom_301(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/moved", func(_ context.Context, _ *api.Void) (*api.RedirectResp, error) {
		return api.Redirect("/permanent", http.StatusMovedPermanently), nil
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

	assert.Equal(t, http.StatusMovedPermanently, resp.StatusCode)
	assert.Equal(t, "/permanent", resp.Header.Get("Location"))
}

type declCookieResp struct {
	Session api.Cookie `cookie:"session"`
	Body    struct {
		Name string `json:"name"`
	}
}

func TestResponse_declarative_cookie(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/cookie", func(_ context.Context, _ *api.Void) (*declCookieResp, error) {
		return &declCookieResp{
			Session: api.Cookie{Value: "val", HttpOnly: true},
			Body:    struct{ Name string `json:"name"` }{Name: "hello"},
		}, nil
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
	assert.Equal(t, "session", cookies[0].Name)
	assert.Equal(t, "val", cookies[0].Value)
	assert.True(t, cookies[0].HttpOnly)

	var body struct {
		Name string `json:"name"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "hello", body.Name)
}

func TestResponse_zero_cookie_not_emitted(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/no-cookie", func(_ context.Context, _ *api.Void) (*declCookieResp, error) {
		return &declCookieResp{}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/no-cookie", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Empty(t, resp.Cookies(), "zero-value Cookie must not produce a Set-Cookie header")
}

type declHeaderResp struct {
	Custom  string `header:"X-Custom-Header"`
	Another string `header:"X-Another"`
	Body    struct {
		OK bool `json:"ok"`
	}
}

func TestResponse_declarative_headers(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/headers", func(_ context.Context, _ *api.Void) (*declHeaderResp, error) {
		return &declHeaderResp{
			Custom:  "custom-value",
			Another: "another-value",
			Body:    struct{ OK bool `json:"ok"` }{OK: true},
		}, nil
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

func TestResponse_empty_string_header_not_emitted(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/empty-header", func(_ context.Context, _ *api.Void) (*declHeaderResp, error) {
		return &declHeaderResp{}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/empty-header", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Empty(t, resp.Header.Values("X-Custom-Header"), "empty string header must not be emitted")
	assert.Empty(t, resp.Header.Values("X-Another"), "empty string header must not be emitted")
}

func TestResponse_defaultError_problemDetails(t *testing.T) {
	t.Parallel()

	r := api.New() // default is ErrorBodyProblemDetails
	api.Get(r, "/fail", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return nil, api.Error(api.CodeUnprocessableContent, api.WithMessage("bad data"))
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

	var pd api.ProblemDetails
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&pd))
	assert.Equal(t, api.CodeUnprocessableContent, pd.Code)
	assert.Equal(t, "bad data", pd.Detail)
	assert.Equal(t, "/fail", pd.Instance)
	assert.Equal(t, "Unprocessable Entity", pd.Title)
	assert.Equal(t, "about:blank", pd.Type)
}

func TestResponse_withoutErrorBody(t *testing.T) {
	t.Parallel()

	r := api.New(api.WithError(api.WithoutErrorBody()))
	api.Get(r, "/fail", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return nil, api.Error(api.CodeUnprocessableContent, api.WithMessage("bad data"))
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/fail", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Empty(t, body, "WithoutErrorBody should suppress the body")
}

func TestResponse_errorBodyText(t *testing.T) {
	t.Parallel()

	r := api.New(api.WithError(api.WithErrorBody(api.ErrorBodyText)))
	api.Get(r, "/fail", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return nil, api.Error(api.CodeUnprocessableContent, api.WithMessage("bad data"))
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/fail", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Equal(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "bad data", string(body))
}

func TestResponse_genericError_wrappedAsInternal(t *testing.T) {
	t.Parallel()

	// With ErrorBodyEnvelope configured, a plain errors.New is wrapped
	// into CodeInternal and rendered with the framework envelope.
	r := api.New(api.WithError(api.WithErrorBody(api.ErrorBodyProblemDetails)))
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

	var env api.ProblemDetails
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
	assert.Equal(t, api.CodeInternal, env.Code)
	assert.Equal(t, "something broke", env.Detail)
}

// --- Body kind: io.Reader (streaming) ---

type downloadResp struct {
	Type        string `header:"Content-Type"`
	Disposition string `header:"Content-Disposition"`
	Body        io.Reader
}

func TestResponse_readerBody_streams(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/download", func(_ context.Context, _ *api.Void) (*downloadResp, error) {
		return &downloadResp{
			Type:        "text/plain",
			Disposition: `attachment; filename="report.txt"`,
			Body:        strings.NewReader("file contents here"),
		}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/download", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/plain", resp.Header.Get("Content-Type"))
	assert.Equal(t, `attachment; filename="report.txt"`, resp.Header.Get("Content-Disposition"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "file contents here", string(body))
}

func TestResponse_readerBody_nil_emits_no_body(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/nil-reader", func(_ context.Context, _ *api.Void) (*downloadResp, error) {
		return &downloadResp{Type: "text/plain"}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/nil-reader", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Empty(t, body)
}

// --- Body kind: <-chan Event (SSE) ---

type eventsResponse struct {
	Body <-chan api.Event
}

func TestResponse_chanBody_emitsSSE(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/events", func(_ context.Context, _ *api.Void) (*eventsResponse, error) {
		ch := make(chan api.Event, 3)
		ch <- api.Event{Name: "message", Data: "hello", ID: "1"}
		ch <- api.Event{Name: "message", Data: map[string]string{"key": "value"}, ID: "2"}
		ch <- api.Event{Data: "plain"}
		close(ch)
		return &eventsResponse{Body: ch}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/events", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	content := string(body)

	assert.Contains(t, content, "id: 1")
	assert.Contains(t, content, "event: message")
	assert.Contains(t, content, "data: hello")
	assert.Contains(t, content, `data: {"key":"value"}`)
	assert.Contains(t, content, "data: plain")
}

func TestResponse_chanBody_nil_emits_no_body(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/nil-events", func(_ context.Context, _ *api.Void) (*eventsResponse, error) {
		return &eventsResponse{}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/nil-events", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Empty(t, body)
}

// --- Status-driven body suppression ---

type conditionalResp struct {
	Status int    `status:""`
	ETag   string `header:"ETag"`
	Body   struct {
		Content string `json:"content"`
	}
}

func TestResponse_status_304_suppresses_body(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/article", func(_ context.Context, _ *api.Void) (*conditionalResp, error) {
		return &conditionalResp{Status: http.StatusNotModified, ETag: `"abc"`}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/article", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusNotModified, resp.StatusCode)
	assert.Equal(t, `"abc"`, resp.Header.Get("ETag"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Empty(t, body, "304 response must have no body")
}

func TestResponse_status_204_suppresses_body(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/nocontent", func(_ context.Context, _ *api.Void) (*conditionalResp, error) {
		return &conditionalResp{Status: http.StatusNoContent}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/nocontent", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Empty(t, body)
}

// --- Header value types ---

type multiHeaderResp struct {
	Count int       `header:"X-Count"`
	When  time.Time `header:"Last-Modified"`
	Links []string  `header:"Link"`
	Body  struct {
		OK bool `json:"ok"`
	}
}

func TestResponse_intHeader(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/int-header", func(_ context.Context, _ *api.Void) (*multiHeaderResp, error) {
		return &multiHeaderResp{Count: 42}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/int-header", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, "42", resp.Header.Get("X-Count"))
}

func TestResponse_timeHeader(t *testing.T) {
	t.Parallel()

	r := api.New()
	ts := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	api.Get(r, "/time-header", func(_ context.Context, _ *api.Void) (*multiHeaderResp, error) {
		return &multiHeaderResp{When: ts}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/time-header", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	got := resp.Header.Get("Last-Modified")
	assert.Equal(t, ts.UTC().Format(http.TimeFormat), got)
}

func TestResponse_timeHeader_zeroTime_omitted(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/zero-time-header", func(_ context.Context, _ *api.Void) (*multiHeaderResp, error) {
		return &multiHeaderResp{}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/zero-time-header", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Empty(t, resp.Header.Get("Last-Modified"))
}

func TestResponse_sliceStringHeader(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/slice-header", func(_ context.Context, _ *api.Void) (*multiHeaderResp, error) {
		return &multiHeaderResp{Links: []string{`<prev>; rel="prev"`, `<next>; rel="next"`}}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/slice-header", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	values := resp.Header.Values("Link")
	require.Len(t, values, 2)
	assert.Contains(t, values, `<prev>; rel="prev"`)
	assert.Contains(t, values, `<next>; rel="next"`)
}

// --- Unsupported header field type falls back silently ---

type floatHeaderResp struct {
	Ratio float64 `header:"X-Ratio"`
	Body  struct {
		OK bool `json:"ok"`
	}
}

func TestResponse_unsupportedHeaderType_notEmitted(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/f", func(_ context.Context, _ *api.Void) (*floatHeaderResp, error) {
		return &floatHeaderResp{Ratio: 3.14}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/f", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Empty(t, resp.Header.Get("X-Ratio"), "unsupported header field kinds must not be emitted")
}

// --- Non-time.Time struct header field not emitted ---

type structHeaderResp struct {
	Opaque struct{ X int } `header:"X-Opaque"`
	Body   struct {
		OK bool `json:"ok"`
	}
}

func TestResponse_structHeaderType_notEmitted(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/s", func(_ context.Context, _ *api.Void) (*structHeaderResp, error) {
		return &structHeaderResp{}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/s", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Empty(t, resp.Header.Get("X-Opaque"), "non-time.Time struct fields must not be emitted")
}

// --- Unsigned integer statuses and headers ---

type unsignedResp struct {
	Status uint   `status:""`
	Count  uint32 `header:"X-Count"`
	Body   struct {
		OK bool `json:"ok"`
	}
}

func TestResponse_unsignedStatusAndHeader(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/u", func(_ context.Context, _ *api.Void) (*unsignedResp, error) {
		return &unsignedResp{
			Status: uint(http.StatusAccepted),
			Count:  uint32(123),
		}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/u", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	assert.Equal(t, "123", resp.Header.Get("X-Count"))
}

// --- Non-integer status field falls back to default ---

type badStatusResp struct {
	Status string `status:""`
	Body   struct {
		OK bool `json:"ok"`
	}
}

func TestResponse_nonIntStatusField_usesDefault(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/bs", func(_ context.Context, _ *api.Void) (*badStatusResp, error) {
		return &badStatusResp{Status: "this-is-ignored"}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/bs", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "non-int status field should fall back to route default")
}

// --- Handler returning nil ---

type simpleResp struct {
	Body struct {
		OK bool `json:"ok"`
	}
}

func TestResponse_nilReturn_writesDefaultStatus(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Get(r, "/nil", func(_ context.Context, _ *api.Void) (*simpleResp, error) {
		var resp *simpleResp // explicitly nil — exercise the "typed nil pointer" path
		return resp, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/nil", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestResponse_declarative_trailers(t *testing.T) {
	t.Parallel()

	type Resp struct {
		Checksum string `trailer:"X-Checksum"`
		Body     struct {
			Value string `json:"value"`
		}
	}

	r := api.New()
	api.Get(r, "/data", func(_ context.Context, _ *api.Void) (*Resp, error) {
		out := &Resp{Checksum: "deadbeef"}
		out.Body.Value = "ok"
		return out, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/data", nil)
	require.NoError(t, err)

	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Trailers populate on resp.Trailer after the body is fully read.
	_, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "deadbeef", resp.Trailer.Get("X-Checksum"))
}

func TestResponse_io_Reader_supports_range(t *testing.T) {
	t.Parallel()

	type Resp struct {
		Body io.Reader
	}

	const payload = "0123456789ABCDEFGHIJ"

	r := api.New()
	api.Get(r, "/file", func(_ context.Context, _ *api.Void) (*Resp, error) {
		return &Resp{Body: bytes.NewReader([]byte(payload))}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/file", nil)
	require.NoError(t, err)
	req.Header.Set("Range", "bytes=4-9")

	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusPartialContent, resp.StatusCode)
	assert.Equal(t, "bytes 4-9/20", resp.Header.Get("Content-Range"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "456789", string(body))
}

func TestResponse_validation_catches_bad_body(t *testing.T) {
	t.Parallel()

	type Resp struct {
		Body struct {
			Name string `json:"name" minLength:"3"`
		}
	}

	r := api.New(api.WithResponseValidation())
	api.Get(r, "/bad", func(_ context.Context, _ *api.Void) (*Resp, error) {
		out := &Resp{}
		out.Body.Name = "x" // too short — fails minLength:"3"
		return out, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/bad", nil)
	require.NoError(t, err)
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestResponse_validation_disabled_by_default(t *testing.T) {
	t.Parallel()

	type Resp struct {
		Body struct {
			Name string `json:"name" minLength:"3"`
		}
	}

	r := api.New() // no WithResponseValidation
	api.Get(r, "/lax", func(_ context.Context, _ *api.Void) (*Resp, error) {
		out := &Resp{}
		out.Body.Name = "x"
		return out, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/lax", nil)
	require.NoError(t, err)
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
