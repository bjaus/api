package api_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjaus/api"
)

func TestErr_Error(t *testing.T) {
	t.Parallel()

	err := api.Error(api.CodeNotFound, api.WithMessage("not found"))
	assert.EqualError(t, err, "not found")

	var sc api.StatusCoder
	require.ErrorAs(t, err, &sc)
	assert.Equal(t, http.StatusNotFound, sc.StatusCode())
}

func TestErr_Error_fallsBackToStatusText(t *testing.T) {
	t.Parallel()

	err := api.Error(api.CodeNotFound)
	assert.EqualError(t, err, http.StatusText(http.StatusNotFound), "no message → status text")
}

func TestErr_Code(t *testing.T) {
	t.Parallel()

	err := api.Error(api.CodeConflict, api.WithMessage("conflict"))

	var apiErr *api.Err
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, api.CodeConflict, apiErr.Code())
	assert.Equal(t, http.StatusConflict, apiErr.StatusCode())
	assert.Equal(t, "conflict", apiErr.Message())
}

func TestErr_Details(t *testing.T) {
	t.Parallel()

	type fieldErr struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	}

	err := api.Error(api.CodeBadRequest,
		api.WithDetail(fieldErr{Field: "email", Message: "required"}),
		api.WithDetail(fieldErr{Field: "name", Message: "too short"}),
	)

	var apiErr *api.Err
	require.ErrorAs(t, err, &apiErr)
	require.Len(t, apiErr.Details(), 2)
}

func TestErr_Instance_emptyWithoutRequest(t *testing.T) {
	t.Parallel()

	// An Err built outside of a request pipeline has no Instance.
	err := api.Error(api.CodeNotFound)

	var apiErr *api.Err
	require.ErrorAs(t, err, &apiErr)
	assert.Empty(t, apiErr.Instance())
}

func TestErr_WithMessagef(t *testing.T) {
	t.Parallel()

	err := api.Error(api.CodeBadRequest, api.WithMessagef("invalid %s: %d", "age", 42))
	assert.EqualError(t, err, "invalid age: 42")
}

func TestErr_WithCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("underlying")
	err := api.Error(api.CodeInternal, api.WithCause(cause), api.WithMessage("wrapped"))

	assert.EqualError(t, err, "wrapped")
	assert.ErrorIs(t, err, cause)
}

func TestValidationError_Error(t *testing.T) {
	t.Parallel()

	ve := &api.ValidationError{Field: "email", Message: "required"}
	assert.EqualError(t, ve, "required")
}

func TestErrorStatus(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		err    error
		expect int
	}{
		"with *Err StatusCoder": {
			err:    api.Error(api.CodeForbidden, api.WithMessage("forbidden")),
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
