package api_test

import (
	"context"
	"encoding/json"
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

func (r *validatedReq) Validate() error {
	if r.Body.Name == "" {
		return api.Error(http.StatusBadRequest, "name required")
	}
	return nil
}

func TestSelfValidator(t *testing.T) {
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
		wantErr    require.ErrorAssertionFunc
	}{
		"valid": {
			body:       `{"name":"Alice"}`,
			wantStatus: http.StatusOK,
			wantErr:    require.NoError,
		},
		"invalid - empty name": {
			body:       `{"name":""}`,
			wantStatus: http.StatusBadRequest,
			wantErr:    require.NoError,
		},
		"invalid - missing name": {
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
			wantErr:    require.NoError,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/users", strings.NewReader(tc.body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			tc.wantErr(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			assert.Equal(t, tc.wantStatus, resp.StatusCode)
		})
	}
}

type globalValidator struct{}

func (globalValidator) Validate(_ any) error {
	return nil
}

func TestGlobalValidator(t *testing.T) {
	t.Parallel()

	type Req struct {
		Name string `json:"name"`
	}
	type Resp struct {
		Name string `json:"name"`
	}

	r := api.New(api.WithValidator(globalValidator{}))
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

	var body Resp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "Bob", body.Name)
}
