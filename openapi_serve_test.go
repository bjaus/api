package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/bjaus/api"
)

func TestServeSpecYAML(t *testing.T) {
	t.Parallel()

	r := api.New(api.WithTitle("YAML Test"), api.WithVersion("1.0.0"))
	api.Get(r, "/health", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	})
	r.ServeSpecYAML("/openapi.yaml")

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/openapi.yaml", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/yaml", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal(body, &parsed))
	assert.Equal(t, "3.1.0", parsed["openapi"])

	info, ok := parsed["info"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "YAML Test", info["title"])
}

func TestWriteSpec(t *testing.T) {
	t.Parallel()

	r := api.New(api.WithTitle("Write Test"), api.WithVersion("2.0.0"))
	api.Get(r, "/ping", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	})

	var buf bytes.Buffer
	err := r.WriteSpec(&buf)
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &spec))
	assert.Equal(t, "3.1.0", spec["openapi"])

	info, ok := spec["info"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Write Test", info["title"])
	assert.Equal(t, "2.0.0", info["version"])
	assert.Contains(t, spec, "paths")
}

func TestWriteSpecYAML(t *testing.T) {
	t.Parallel()

	r := api.New(api.WithTitle("YAML Write"), api.WithVersion("3.0.0"))
	api.Get(r, "/status", func(_ context.Context, _ *api.Void) (*api.Void, error) {
		return &api.Void{}, nil
	})

	var buf bytes.Buffer
	err := r.WriteSpecYAML(&buf)
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, yaml.Unmarshal(buf.Bytes(), &spec))
	assert.Equal(t, "3.1.0", spec["openapi"])

	info, ok := spec["info"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "YAML Write", info["title"])
	assert.Equal(t, "3.0.0", info["version"])
	assert.Contains(t, spec, "paths")
}
