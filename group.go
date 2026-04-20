package api

// Group is a collection of routes under a shared prefix with shared middleware and tags.
// Groups can be nested: child groups inherit prefix, middleware, tags, and security
// from their parent unless explicitly reset.
type Group struct {
	parent          Registrar
	prefix          string
	middleware      []Middleware
	tags            []string
	security        []string
	resetMiddleware bool
	errorOpts       []ErrorOption
}

// GroupOption configures a Group at construction time. Implement this
// interface (or use the GroupOptionFunc adapter) to define custom options.
type GroupOption interface {
	applyGroup(*Group)
}

// GroupOptionFunc is a function adapter that satisfies GroupOption.
type GroupOptionFunc func(*Group)

func (f GroupOptionFunc) applyGroup(g *Group) { f(g) }

// WithGroupTags adds default tags to all routes registered on the group.
func WithGroupTags(tags ...string) GroupOption {
	return GroupOptionFunc(func(g *Group) {
		g.tags = append(g.tags, tags...)
	})
}

// WithGroupMiddleware adds middleware to the group. For nested groups, the
// child's middleware is appended to the parent's unless WithGroupMiddlewareReset
// is also supplied.
func WithGroupMiddleware(mw ...Middleware) GroupOption {
	return GroupOptionFunc(func(g *Group) {
		g.middleware = append(g.middleware, mw...)
	})
}

// WithGroupMiddlewareReset causes a nested group to ignore its parent's
// middleware stack and start with an isolated one. The group's own middleware
// (added via WithGroupMiddleware) still applies.
func WithGroupMiddlewareReset() GroupOption {
	return GroupOptionFunc(func(g *Group) {
		g.resetMiddleware = true
	})
}

// WithGroupSecurity sets security requirements for all routes in the group.
// For nested groups, the child's security replaces the parent's; absence
// inherits the parent's security.
func WithGroupSecurity(schemes ...string) GroupOption {
	return GroupOptionFunc(func(g *Group) {
		g.security = append(g.security, schemes...)
	})
}

// Group creates a new route group with the given prefix and options.
func (r *Router) Group(prefix string, opts ...GroupOption) *Group {
	return newGroup(r, prefix, opts...)
}

// Group creates a nested route group. The child's prefix is concatenated onto
// the parent's; tags, middleware (unless reset), and security (when child has
// none) inherit from the parent.
func (g *Group) Group(prefix string, opts ...GroupOption) *Group {
	return newGroup(g, prefix, opts...)
}

func newGroup(parent Registrar, prefix string, opts ...GroupOption) *Group {
	g := &Group{
		parent: parent,
		prefix: prefix,
	}
	for _, opt := range opts {
		opt.applyGroup(g)
	}
	return g
}

// addRoute implements Registrar for Group. It contributes the group's own
// prefix, tags, and security, then delegates to the parent so nested groups
// compose correctly.
func (g *Group) addRoute(ri routeInfo) {
	ri.pattern = g.prefix + ri.pattern
	ri.tags = append(append([]string{}, g.tags...), ri.tags...)
	if len(g.security) > 0 && len(ri.security) == 0 && !ri.noSecurity {
		ri.security = append([]string{}, g.security...)
	}
	g.parent.addRoute(ri)
}

func (g *Group) getValidator() ValidatorFunc             { return g.parent.getValidator() }
func (g *Group) getErrorHandler() ErrorHandler           { return g.parent.getErrorHandler() }
func (g *Group) getErrorBuilder() ValidationErrorBuilder { return g.parent.getErrorBuilder() }
func (g *Group) getMode() ValidationMode                 { return g.parent.getMode() }
func (g *Group) getCodecs() *codecRegistry               { return g.parent.getCodecs() }

// errorOptionChain returns the parent's chain followed by this group's
// own error options. Outer scopes come first so later scopes can
// override scalars and accumulate lists.
func (g *Group) errorOptionChain() []ErrorOption {
	parent := g.parent.errorOptionChain()
	out := make([]ErrorOption, 0, len(parent)+len(g.errorOpts))
	out = append(out, parent...)
	out = append(out, g.errorOpts...)
	return out
}

// routeMiddleware returns the combined middleware stack: parent's (unless
// reset) followed by this group's. The parent's middleware wraps the child's,
// so parent middleware runs first per request.
func (g *Group) routeMiddleware() []Middleware {
	if g.resetMiddleware {
		return append([]Middleware{}, g.middleware...)
	}
	parent := g.parent.routeMiddleware()
	out := make([]Middleware, 0, len(parent)+len(g.middleware))
	out = append(out, parent...)
	out = append(out, g.middleware...)
	return out
}
