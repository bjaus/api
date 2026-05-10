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

	operationID string
	security    []string
	noSecurity  bool

	extensions map[string]any
	links      map[string]Link
	callbacks  map[string]map[string]PathItem

	bodyLimit int64

	mode ValidationMode

	reqType  reflect.Type
	respType reflect.Type

	requestDesc  *requestDescriptor
	responseDesc *responseDescriptor

	// errorOpts accumulates error-related options attached directly to
	// this route via api.WithError.
	errorOpts []ErrorOption

	// errorCodes is the set of Codes documented for this route via
	// WithError(WithErrors(...)). Populated at registration after
	// merging router/group/route scope options.
	errorCodes []Code

	// errorTemplate holds scope-level error options pre-applied. Used as
	// the base state when emitting an error response; inline options on
	// the returned *Err overlay this template.
	errorTemplate *Err

	handler http.Handler
}

// WithMode overrides the router's ValidationMode for this route.
func WithMode(m ValidationMode) RouteOption {
	return RouteOptionFunc(func(ri *routeInfo) {
		ri.mode = m
	})
}

// RouteOption configures a route at registration time. Implement this
// interface (or use the RouteOptionFunc adapter) to define custom options.
type RouteOption interface {
	applyRoute(*routeInfo)
}

// RouteOptionFunc is a function adapter that satisfies RouteOption.
type RouteOptionFunc func(*routeInfo)

func (f RouteOptionFunc) applyRoute(ri *routeInfo) { f(ri) }

// WithStatus sets the default HTTP status code for the response.
func WithStatus(code int) RouteOption {
	return RouteOptionFunc(func(ri *routeInfo) {
		ri.status = code
	})
}

// WithSummary sets the OpenAPI summary for the route.
func WithSummary(s string) RouteOption {
	return RouteOptionFunc(func(ri *routeInfo) {
		ri.summary = s
	})
}

// WithDescription sets the OpenAPI description for the route.
func WithDescription(d string) RouteOption {
	return RouteOptionFunc(func(ri *routeInfo) {
		ri.desc = d
	})
}

// WithTags adds OpenAPI tags to the route.
func WithTags(tags ...string) RouteOption {
	return RouteOptionFunc(func(ri *routeInfo) {
		ri.tags = append(ri.tags, tags...)
	})
}

// WithDeprecated marks the route as deprecated in the OpenAPI spec.
func WithDeprecated() RouteOption {
	return RouteOptionFunc(func(ri *routeInfo) {
		ri.deprecated = true
	})
}

// WithOperationID sets a custom OpenAPI operationId.
func WithOperationID(id string) RouteOption {
	return RouteOptionFunc(func(ri *routeInfo) {
		ri.operationID = id
	})
}

// WithSecurity sets security scheme requirements for this route.
func WithSecurity(schemes ...string) RouteOption {
	return RouteOptionFunc(func(ri *routeInfo) {
		ri.security = append(ri.security, schemes...)
	})
}

// WithNoSecurity disables security for this route (overrides global security).
func WithNoSecurity() RouteOption {
	return RouteOptionFunc(func(ri *routeInfo) {
		ri.noSecurity = true
	})
}

// WithExtension adds an OpenAPI extension to the operation.
// The key must start with "x-".
func WithExtension(key string, value any) RouteOption {
	return RouteOptionFunc(func(ri *routeInfo) {
		if ri.extensions == nil {
			ri.extensions = make(map[string]any)
		}
		ri.extensions[key] = value
	})
}

// WithLink adds an OpenAPI link to the operation's response.
func WithLink(name string, link Link) RouteOption {
	return RouteOptionFunc(func(ri *routeInfo) {
		if ri.links == nil {
			ri.links = make(map[string]Link)
		}
		ri.links[name] = link
	})
}

// WithBodyLimit sets a per-route maximum request body size in bytes.
// This overrides any global BodyLimit middleware for this route.
func WithBodyLimit(maxBytes int64) RouteOption {
	return RouteOptionFunc(func(ri *routeInfo) {
		ri.bodyLimit = maxBytes
	})
}

// WithCallback adds an OpenAPI callback to the operation.
func WithCallback(name string, cb map[string]PathItem) RouteOption {
	return RouteOptionFunc(func(ri *routeInfo) {
		if ri.callbacks == nil {
			ri.callbacks = make(map[string]map[string]PathItem)
		}
		ri.callbacks[name] = cb
	})
}
