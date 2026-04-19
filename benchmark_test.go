package api_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bjaus/api"
)

// --- Small JSON response (3 fields, no metadata) ---

type benchSmallResp struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func BenchmarkEmit_smallJSON(b *testing.B) {
	r := api.New()
	api.Get(r, "/s", func(_ context.Context, _ *api.Void) (*api.Resp[benchSmallResp], error) {
		return &api.Resp[benchSmallResp]{Body: benchSmallResp{ID: "x", Name: "n", Age: 42}}, nil
	})
	srv := httptest.NewServer(r)
	b.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/s", nil)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}
}

// --- Medium JSON response (10 tagged fields, embedded type) ---

type benchCacheHeaders struct {
	ETag         string    `header:"ETag"`
	CacheControl string    `header:"Cache-Control"`
	LastModified time.Time `header:"Last-Modified"`
}

type benchMediumRespBody struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Tags     []string `json:"tags"`
	Views    int      `json:"views"`
	Archived bool     `json:"archived"`
}

type benchMediumResp struct {
	benchCacheHeaders
	Count   int        `header:"X-Count"`
	Session api.Cookie `cookie:"session"`
	Status  int        `status:""`
	Body    benchMediumRespBody
}

func BenchmarkEmit_mediumWithMetadata(b *testing.B) {
	r := api.New()
	api.Get(r, "/m", func(_ context.Context, _ *api.Void) (*benchMediumResp, error) {
		return &benchMediumResp{
			benchCacheHeaders: benchCacheHeaders{
				ETag:         `"abc"`,
				CacheControl: "max-age=60",
				LastModified: time.Unix(1_700_000_000, 0),
			},
			Count:   42,
			Session: api.Cookie{Value: "tok", HttpOnly: true},
			Status:  http.StatusOK,
			Body: benchMediumRespBody{
				ID:    "i",
				Title: "t",
				Tags:  []string{"a", "b", "c"},
				Views: 1000,
			},
		}, nil
	})
	srv := httptest.NewServer(r)
	b.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/m", nil)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}
}

// --- Stream body (io.Reader) ---

type benchStreamResp struct {
	Type string `header:"Content-Type"`
	Body io.Reader
}

func BenchmarkEmit_readerBody(b *testing.B) {
	payload := strings.Repeat("x", 1024)

	r := api.New()
	api.Get(r, "/stream", func(_ context.Context, _ *api.Void) (*benchStreamResp, error) {
		return &benchStreamResp{Type: "text/plain", Body: strings.NewReader(payload)}, nil
	})
	srv := httptest.NewServer(r)
	b.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/stream", nil)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}
}

// --- SSE body (<-chan Event) ---

type benchSSEResp struct {
	Body <-chan api.Event
}

func BenchmarkEmit_sseBody(b *testing.B) {
	r := api.New()
	api.Get(r, "/sse", func(_ context.Context, _ *api.Void) (*benchSSEResp, error) {
		ch := make(chan api.Event, 3)
		ch <- api.Event{Name: "tick", Data: "1"}
		ch <- api.Event{Name: "tick", Data: "2"}
		ch <- api.Event{Name: "tick", Data: "3"}
		close(ch)
		return &benchSSEResp{Body: ch}, nil
	})
	srv := httptest.NewServer(r)
	b.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/sse", nil)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}
}

// --- Request binding: small ---

type benchSmallReq struct {
	ID    string `path:"id"`
	Page  int    `query:"page"`
	Auth  string `header:"Authorization"`
}

func BenchmarkBind_smallRequest(b *testing.B) {
	r := api.New()
	api.Get(r, "/items/{id}", func(_ context.Context, _ *benchSmallReq) (*api.Resp[benchSmallResp], error) {
		return &api.Resp[benchSmallResp]{Body: benchSmallResp{ID: "x"}}, nil
	})

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/items/abc?page=3", nil)
	if err != nil {
		b.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer token")

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}
}

// --- Request binding: medium with body ---

type benchMediumReq struct {
	OrgID string `path:"org_id"`
	Page  int    `query:"page"`
	Sort  string `query:"sort"`
	Lang  string `header:"Accept-Language"`
	Auth  string `header:"Authorization"`
	Body  struct {
		Name    string   `json:"name"`
		Email   string   `json:"email"`
		Tags    []string `json:"tags"`
		Active  bool     `json:"active"`
		Quota   int      `json:"quota"`
	}
}

func BenchmarkBind_mediumRequestWithBody(b *testing.B) {
	r := api.New()
	api.Post(r, "/orgs/{org_id}/users", func(_ context.Context, _ *benchMediumReq) (*api.Resp[benchSmallResp], error) {
		return &api.Resp[benchSmallResp]{Body: benchSmallResp{ID: "x"}}, nil
	})

	body := []byte(`{"name":"Alice","email":"a@b.com","tags":["x","y","z"],"active":true,"quota":100}`)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/orgs/acme/users?page=3&sort=name", bytes.NewReader(body))
		if err != nil {
			b.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept-Language", "en-US")
		req.Header.Set("Authorization", "Bearer token")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}
}
