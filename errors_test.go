package api_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjaus/api"
)

func TestProblemDetail_Error(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		pd     api.ProblemDetail
		expect string
	}{
		"with detail": {
			pd:     api.ProblemDetail{Detail: "something went wrong", Title: "Bad Request"},
			expect: "something went wrong",
		},
		"empty detail returns title": {
			pd:     api.ProblemDetail{Title: "Not Found"},
			expect: "Not Found",
		},
		"both empty": {
			pd:     api.ProblemDetail{},
			expect: "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expect, tc.pd.Error())
		})
	}
}

func TestProblemDetail_StatusCode(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		status int
	}{
		"400": {status: http.StatusBadRequest},
		"404": {status: http.StatusNotFound},
		"500": {status: http.StatusInternalServerError},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			pd := &api.ProblemDetail{Status: tc.status}
			assert.Equal(t, tc.status, pd.StatusCode())
		})
	}
}

func TestHTTPError_Error(t *testing.T) {
	t.Parallel()

	err := api.Error(http.StatusNotFound, "not found")
	assert.EqualError(t, err, "not found")

	var sc api.StatusCoder
	require.ErrorAs(t, err, &sc)
	assert.Equal(t, http.StatusNotFound, sc.StatusCode())
}

func TestHTTPError_StatusCode(t *testing.T) {
	t.Parallel()

	err := api.Error(http.StatusConflict, "conflict")

	var apiErr *api.HTTPError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusConflict, apiErr.Status)
	assert.Equal(t, "conflict", apiErr.Message)
}

func TestError_constructor(t *testing.T) {
	t.Parallel()

	err := api.Error(http.StatusForbidden, "forbidden")

	var he *api.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusForbidden, he.StatusCode())
	assert.Equal(t, "forbidden", he.Error())
}

func TestErrorf_constructor(t *testing.T) {
	t.Parallel()

	err := api.Errorf(http.StatusBadRequest, "invalid %s: %d", "age", 42)
	assert.EqualError(t, err, "invalid age: 42")

	var he *api.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusBadRequest, he.StatusCode())
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
		"with ProblemDetail StatusCoder": {
			err:    &api.ProblemDetail{Status: http.StatusConflict},
			expect: http.StatusConflict,
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
