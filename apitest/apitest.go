// Package apitest provides typed test helpers for the api framework.
package apitest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bjaus/api"
)

// Client wraps an httptest.Server for convenient API testing.
type Client struct {
	Server *httptest.Server
}

// NewClient creates a test client from a router.
func NewClient(t testing.TB, r *api.Router) *Client {
	t.Helper()
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return &Client{Server: srv}
}

// Response holds a decoded API response.
type Response[T any] struct {
	Status  int
	Headers http.Header
	Body    *T
	Raw     *http.Response
}

// Get sends a typed GET request.
func Get[Resp any](t testing.TB, c *Client, path string) *Response[Resp] {
	t.Helper()
	return do[Resp](t, c, http.MethodGet, path, nil)
}

// Post sends a typed POST request with a JSON body.
func Post[Req, Resp any](t testing.TB, c *Client, path string, body *Req) *Response[Resp] {
	t.Helper()
	return do[Resp](t, c, http.MethodPost, path, body)
}

// Put sends a typed PUT request with a JSON body.
func Put[Req, Resp any](t testing.TB, c *Client, path string, body *Req) *Response[Resp] {
	t.Helper()
	return do[Resp](t, c, http.MethodPut, path, body)
}

// Patch sends a typed PATCH request with a JSON body.
func Patch[Req, Resp any](t testing.TB, c *Client, path string, body *Req) *Response[Resp] {
	t.Helper()
	return do[Resp](t, c, http.MethodPatch, path, body)
}

// Delete sends a typed DELETE request.
func Delete[Resp any](t testing.TB, c *Client, path string) *Response[Resp] {
	t.Helper()
	return do[Resp](t, c, http.MethodDelete, path, nil)
}

func do[Resp any](t testing.TB, c *Client, method, path string, body any) *Response[Resp] {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("apitest: marshal request body: %v", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(context.Background(), method, c.Server.URL+path, reqBody)
	if err != nil {
		t.Fatalf("apitest: create request: %v", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("apitest: execute request: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("apitest: close body: %v", closeErr)
		}
	}()

	result := &Response[Resp]{
		Status:  resp.StatusCode,
		Headers: resp.Header,
		Raw:     resp,
	}

	if resp.StatusCode != http.StatusNoContent && resp.ContentLength != 0 {
		var decoded Resp
		if decErr := json.NewDecoder(resp.Body).Decode(&decoded); decErr != nil && decErr != io.EOF {
			return result
		}
		result.Body = &decoded
	}

	return result
}
