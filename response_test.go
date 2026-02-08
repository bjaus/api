package api_test

import (
	"context"
	"encoding/json"
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
	defer srv.Close()

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
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/async", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
}
