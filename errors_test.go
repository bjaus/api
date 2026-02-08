package api_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjaus/api"
)

func TestError(t *testing.T) {
	t.Parallel()

	err := api.Error(http.StatusNotFound, "not found")
	assert.EqualError(t, err, "not found")

	var sc api.StatusCoder
	require.ErrorAs(t, err, &sc)
	assert.Equal(t, http.StatusNotFound, sc.StatusCode())
}

func TestErrorf(t *testing.T) {
	t.Parallel()

	err := api.Errorf(http.StatusBadRequest, "invalid %s", "email")
	assert.EqualError(t, err, "invalid email")
}

func TestErrorStatus(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		err    error
		expect int
	}{
		"with StatusCoder": {
			err:    api.Error(http.StatusForbidden, "forbidden"),
			expect: http.StatusForbidden,
		},
		"without StatusCoder": {
			err:    errors.New("plain error"),
			expect: http.StatusInternalServerError,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expect, api.ErrorStatus(tc.err))
		})
	}
}

func TestHTTPError_fields(t *testing.T) {
	t.Parallel()

	err := api.Error(http.StatusConflict, "conflict")

	var apiErr *api.HTTPError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusConflict, apiErr.Status)
	assert.Equal(t, "conflict", apiErr.Message)
}
