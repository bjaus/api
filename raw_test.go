package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjaus/api"
)

func TestRaw_handler(t *testing.T) {
	t.Parallel()

	r := api.New()
	api.Raw(r, http.MethodGet, "/custom", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("custom response"))
		require.NoError(t, err)
	}, api.OperationInfo{
		Summary:     "Custom endpoint",
		Description: "A fully custom endpoint",
		Tags:        []string{"custom"},
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/custom", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/plain", resp.Header.Get("Content-Type"))

	// Verify it shows in the spec.
	spec := r.Spec()
	op, ok := spec.Paths["/custom"]["get"]
	require.True(t, ok)
	assert.Equal(t, "Custom endpoint", op.Summary)
	assert.Contains(t, op.Tags, "custom")
}

func TestRawRequest_embedding(t *testing.T) {
	t.Parallel()

	// RawRequest should be embeddable and have a Request field.
	var rr api.RawRequest
	assert.Nil(t, rr.Request)
}

func TestOperationInfo_fields(t *testing.T) {
	t.Parallel()

	info := api.OperationInfo{
		Summary:     "summary",
		Description: "desc",
		Tags:        []string{"a", "b"},
	}

	assert.Equal(t, "summary", info.Summary)
	assert.Equal(t, "desc", info.Description)
	assert.Equal(t, []string{"a", "b"}, info.Tags)
}
