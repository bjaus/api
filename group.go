package api

// Group is a collection of routes under a shared prefix with shared middleware and tags.
type Group struct {
	router     *Router
	prefix     string
	middleware []Middleware
	tags       []string
	security   []string
}

// GroupOption configures a Group.
type GroupOption func(*Group)

// WithGroupTags adds default tags to all routes registered on the group.
func WithGroupTags(tags ...string) GroupOption {
	return func(g *Group) {
		g.tags = append(g.tags, tags...)
	}
}

// WithGroupMiddleware adds middleware to the group.
func WithGroupMiddleware(mw ...Middleware) GroupOption {
	return func(g *Group) {
		g.middleware = append(g.middleware, mw...)
	}
}

// WithGroupSecurity sets security requirements for all routes in the group.
func WithGroupSecurity(schemes ...string) GroupOption {
	return func(g *Group) {
		g.security = append(g.security, schemes...)
	}
}

// Group creates a new route group with the given prefix and options.
func (r *Router) Group(prefix string, opts ...GroupOption) *Group {
	g := &Group{
		router: r,
		prefix: prefix,
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// addRoute implements Registrar for Group.
func (g *Group) addRoute(ri routeInfo) {
	ri.pattern = g.prefix + ri.pattern
	ri.tags = append(g.tags, ri.tags...)
	if len(g.security) > 0 && len(ri.security) == 0 && !ri.noSecurity {
		ri.security = g.security
	}
	g.router.addRoute(ri)
}

func (g *Group) getValidator() Validator       { return g.router.validator }
func (g *Group) getErrorHandler() ErrorHandler { return g.router.errorHandler }
func (g *Group) routeMiddleware() []Middleware { return g.middleware }
