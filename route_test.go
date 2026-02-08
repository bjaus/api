package api_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bjaus/api"
)

func TestRouteOptions_compile(t *testing.T) {
	t.Parallel()

	opts := []api.RouteOption{
		api.WithStatus(http.StatusCreated),
		api.WithSummary("Create a user"),
		api.WithDescription("Creates a new user account"),
		api.WithTags("users", "admin"),
		api.WithDeprecated(),
	}

	assert.Len(t, opts, 5)
}
