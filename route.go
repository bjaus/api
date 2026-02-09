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
	status     int
	deprecated bool
	errors     []int

	operationID string
	security    []string
	noSecurity  bool

	extensions map[string]any
	links      map[string]Link
	callbacks  map[string]map[string]PathItem

	bodyLimit int64

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

// WithErrors declares additional HTTP error status codes for the OpenAPI spec.
func WithErrors(codes ...int) RouteOption {
	return func(ri *routeInfo) {
		ri.errors = append(ri.errors, codes...)
	}
}

// WithOperationID sets a custom OpenAPI operationId.
func WithOperationID(id string) RouteOption {
	return func(ri *routeInfo) {
		ri.operationID = id
	}
}

// WithSecurity sets security scheme requirements for this route.
func WithSecurity(schemes ...string) RouteOption {
	return func(ri *routeInfo) {
		ri.security = append(ri.security, schemes...)
	}
}

// WithNoSecurity disables security for this route (overrides global security).
func WithNoSecurity() RouteOption {
	return func(ri *routeInfo) {
		ri.noSecurity = true
	}
}

// WithExtension adds an OpenAPI extension to the operation.
// The key must start with "x-".
func WithExtension(key string, value any) RouteOption {
	return func(ri *routeInfo) {
		if ri.extensions == nil {
			ri.extensions = make(map[string]any)
		}
		ri.extensions[key] = value
	}
}

// WithLink adds an OpenAPI link to the operation's response.
func WithLink(name string, link Link) RouteOption {
	return func(ri *routeInfo) {
		if ri.links == nil {
			ri.links = make(map[string]Link)
		}
		ri.links[name] = link
	}
}

// WithBodyLimit sets a per-route maximum request body size in bytes.
// This overrides any global BodyLimit middleware for this route.
func WithBodyLimit(maxBytes int64) RouteOption {
	return func(ri *routeInfo) {
		ri.bodyLimit = maxBytes
	}
}

// WithCallback adds an OpenAPI callback to the operation.
func WithCallback(name string, cb map[string]PathItem) RouteOption {
	return func(ri *routeInfo) {
		if ri.callbacks == nil {
			ri.callbacks = make(map[string]map[string]PathItem)
		}
		ri.callbacks[name] = cb
	}
}
