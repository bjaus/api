package api_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjaus/api"
)

func TestHandler_signature(t *testing.T) {
	t.Parallel()

	type Req struct {
		Name string `json:"name"`
	}
	type Resp struct {
		Greeting string `json:"greeting"`
	}

	var h api.Handler[Req, Resp] = func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{Greeting: "hello " + req.Name}, nil
	}

	resp, err := h(context.Background(), &Req{Name: "world"})
	require.NoError(t, err)
	assert.Equal(t, "hello world", resp.Greeting)
}

func TestVoid_is_zero_size(t *testing.T) {
	t.Parallel()

	var v api.Void
	_ = v
}
