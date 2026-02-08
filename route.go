package api

import (
	"net/http"
	"reflect"
)

// routeInfo holds metadata for a registered route, used for both
// request dispatch and OpenAPI spec generation.
type routeInfo struct {
	method  string
	pattern string
	summary string
	desc    string
	tags    []string
	status  int
	deprecated bool

	reqType  reflect.Type
	respType reflect.Type

	handler http.Handler
}

// RouteOption configures a route at registration time.
type RouteOption func(*routeInfo)

// WithStatus sets the default HTTP status code for the response.
func WithStatus(code int) RouteOption {
	return func(ri *routeInfo) {
		ri.status = code
	}
}

// WithSummary sets the OpenAPI summary for the route.
func WithSummary(s string) RouteOption {
	return func(ri *routeInfo) {
		ri.summary = s
	}
}

// WithDescription sets the OpenAPI description for the route.
func WithDescription(d string) RouteOption {
	return func(ri *routeInfo) {
		ri.desc = d
	}
}

// WithTags adds OpenAPI tags to the route.
func WithTags(tags ...string) RouteOption {
	return func(ri *routeInfo) {
		ri.tags = append(ri.tags, tags...)
	}
}

// WithDeprecated marks the route as deprecated in the OpenAPI spec.
func WithDeprecated() RouteOption {
	return func(ri *routeInfo) {
		ri.deprecated = true
	}
}
