package api

// WithError bundles ErrorOption values and can be applied at router,
// group, or route scope. The returned value satisfies RouterOption,
// GroupOption, and RouteOption simultaneously — the scope is inferred
// from where it is passed.
//
// Options from outer scopes are applied first, then inner scopes, then
// any inline options on api.Error. Scalar options (message, headers,
// cookies by name, body mapper) are overridden by later scopes; list
// options (details, documented codes) accumulate.
func WithError(opts ...ErrorOption) *ErrorScope {
	return &ErrorScope{opts: opts}
}

// ErrorScope carries a bundle of error options that can be attached at
// any level of the registration hierarchy. It implements RouterOption,
// GroupOption, and RouteOption.
type ErrorScope struct {
	opts []ErrorOption
}

// applyRouter implements the router-level option interface.
func (s *ErrorScope) applyRouter(r *Router) {
	r.errorOpts = append(r.errorOpts, s.opts...)
}

// applyGroup implements the group-level option interface.
func (s *ErrorScope) applyGroup(g *Group) {
	g.errorOpts = append(g.errorOpts, s.opts...)
}

// applyRoute implements the route-level option interface.
func (s *ErrorScope) applyRoute(ri *routeInfo) {
	ri.errorOpts = append(ri.errorOpts, s.opts...)
}
