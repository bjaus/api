package api

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Router is the central type that holds routes, middleware, and configuration.
// It implements http.Handler.
type Router struct {
	mux        *http.ServeMux
	middleware []Middleware
	routes     []routeInfo

	// methodsByPattern tracks which HTTP methods have been registered for
	// each pattern. Used to auto-generate HEAD (from GET) and OPTIONS (Allow
	// header) responses without requiring per-route registration.
	methodsByPattern map[string]map[string]struct{}

	title   string
	version string

	servers         []Server
	securitySchemes map[string]SecurityScheme
	security        []string
	tagDescs        map[string]string

	webhooks map[string]PathItem

	validator         ValidatorFunc
	mode              ValidationMode
	errorHandler      ErrorHandler
	errorOpts         []ErrorOption
	validateResponses bool

	encoders []Encoder
	decoders []Decoder
	codecs   *codecRegistry

	tracer SpanStarter

	mu sync.Mutex
}

// RouterOption configures a Router at construction time. Implement this
// interface (or use the RouterOptionFunc adapter) to define custom
// options.
type RouterOption interface {
	applyRouter(*Router)
}

// RouterOptionFunc is a function adapter that satisfies RouterOption.
// Most WithXxx constructors in this package return a RouterOptionFunc.
type RouterOptionFunc func(*Router)

func (f RouterOptionFunc) applyRouter(r *Router) { f(r) }

// WithTitle sets the API title (used in OpenAPI spec).
func WithTitle(title string) RouterOption {
	return RouterOptionFunc(func(r *Router) {
		r.title = title
	})
}

// WithVersion sets the API version (used in OpenAPI spec).
func WithVersion(version string) RouterOption {
	return RouterOptionFunc(func(r *Router) {
		r.version = version
	})
}

// WithValidator sets a router-level request validator. Typically used to plug
// in a reflection-based library; see ValidatorFunc.
func WithValidator(v ValidatorFunc) RouterOption {
	return RouterOptionFunc(func(r *Router) {
		r.validator = v
	})
}

// WithValidationMode sets the router-wide default for when constraint-tag
// enforcement runs relative to the per-type Validator and ValidatorFunc.
// Default is ValidateConstraintsLast.
func WithValidationMode(m ValidationMode) RouterOption {
	return RouterOptionFunc(func(r *Router) {
		r.mode = m
	})
}

// WithResponseValidation enables constraint-tag validation of response
// structs before they are encoded. A failed response validation produces a
// 500 with the violations attached. Off by default; intended primarily for
// development to surface handler bugs that emit malformed shapes.
func WithResponseValidation() RouterOption {
	return RouterOptionFunc(func(r *Router) {
		r.validateResponses = true
	})
}

// WithServers sets the OpenAPI servers array.
func WithServers(servers ...Server) RouterOption {
	return RouterOptionFunc(func(r *Router) {
		r.servers = servers
	})
}

// WithSecurityScheme registers a named security scheme for the OpenAPI spec.
func WithSecurityScheme(name string, scheme SecurityScheme) RouterOption {
	return RouterOptionFunc(func(r *Router) {
		if r.securitySchemes == nil {
			r.securitySchemes = make(map[string]SecurityScheme)
		}
		r.securitySchemes[name] = scheme
	})
}

// WithGlobalSecurity sets global security requirements by scheme name.
func WithGlobalSecurity(schemes ...string) RouterOption {
	return RouterOptionFunc(func(r *Router) {
		r.security = append(r.security, schemes...)
	})
}

// WithTagDescriptions sets tag descriptions for the OpenAPI spec.
func WithTagDescriptions(descs map[string]string) RouterOption {
	return RouterOptionFunc(func(r *Router) {
		r.tagDescs = descs
	})
}

// ErrorHandler is a custom error response writer.
type ErrorHandler func(w http.ResponseWriter, r *http.Request, err error)

// WithErrorHandler sets a custom error handler for the router.
func WithErrorHandler(h ErrorHandler) RouterOption {
	return RouterOptionFunc(func(r *Router) {
		r.errorHandler = h
	})
}

// WithEncoder registers an additional response encoder.
func WithEncoder(enc Encoder) RouterOption {
	return RouterOptionFunc(func(r *Router) {
		r.encoders = append(r.encoders, enc)
	})
}

// WithDecoder registers an additional request body decoder.
func WithDecoder(dec Decoder) RouterOption {
	return RouterOptionFunc(func(r *Router) {
		r.decoders = append(r.decoders, dec)
	})
}

// WithWebhook registers a webhook path item for the OpenAPI spec.
func WithWebhook(name string, item PathItem) RouterOption {
	return RouterOptionFunc(func(r *Router) {
		if r.webhooks == nil {
			r.webhooks = make(map[string]PathItem)
		}
		r.webhooks[name] = item
	})
}

// SpanStarter is a tracing hook interface for creating spans per request.
// Implement this with your preferred tracing backend (e.g., OpenTelemetry).
type SpanStarter interface {
	StartSpan(ctx context.Context, name string, attrs map[string]string) (context.Context, func())
}

// WithTracer sets a tracing hook for the router.
func WithTracer(s SpanStarter) RouterOption {
	return RouterOptionFunc(func(r *Router) {
		r.tracer = s
	})
}

// New creates a new Router with the given options.
func New(opts ...RouterOption) *Router {
	r := &Router{
		mux:              http.NewServeMux(),
		methodsByPattern: make(map[string]map[string]struct{}),
	}
	for _, opt := range opts {
		opt.applyRouter(r)
	}
	r.codecs = newCodecRegistry(r.encoders, r.decoders)
	return r
}

// Use adds middleware to the router. Middleware is applied in the order added.
func (r *Router) Use(mw ...Middleware) {
	r.middleware = append(r.middleware, mw...)
}

// ServeHTTP implements http.Handler. Middleware is applied in registration
// order, then dispatch goes through autoMethodsHandler so HEAD and OPTIONS
// requests get derived responses when no explicit handler exists.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	handler := http.Handler(http.HandlerFunc(r.dispatch))
	for i := len(r.middleware) - 1; i >= 0; i-- {
		handler = r.middleware[i](handler)
	}
	handler.ServeHTTP(w, req)
}

// dispatch routes the request through the mux, deriving HEAD from GET and
// auto-generating OPTIONS Allow responses when no explicit handler exists.
func (r *Router) dispatch(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodHead:
		if r.methodRegistered(req, http.MethodHead) {
			r.mux.ServeHTTP(w, req)
			return
		}
		if pattern, ok := r.matchPattern(req, http.MethodGet); ok {
			r.serveHEADFromGET(w, req, pattern)
			return
		}
	case http.MethodOptions:
		if r.methodRegistered(req, http.MethodOptions) {
			r.mux.ServeHTTP(w, req)
			return
		}
		if methods := r.allowedMethods(req); len(methods) > 0 {
			methods = appendMethod(methods, http.MethodOptions)
			w.Header().Set("Allow", strings.Join(methods, ", "))
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	r.mux.ServeHTTP(w, req)
}

// methodRegistered reports whether the given method is explicitly registered
// for the pattern that matches req's URL path.
func (r *Router) methodRegistered(req *http.Request, method string) bool {
	pattern, ok := r.matchPattern(req, method)
	if !ok {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if methods, exists := r.methodsByPattern[pattern]; exists {
		_, present := methods[method]
		return present
	}
	return false
}

// matchPattern returns the bare path pattern in r.mux that matches req's URL
// path for the given method, or "" if no match. mux.Handler returns the
// pattern with the method prefix ("GET /items"); we strip it.
func (r *Router) matchPattern(req *http.Request, method string) (string, bool) {
	probe := req.Clone(req.Context())
	probe.Method = method
	_, pattern := r.mux.Handler(probe)
	return stripMethodPrefix(pattern), pattern != ""
}

// stripMethodPrefix removes the leading "METHOD " token (if any) from a
// ServeMux pattern, leaving just the path.
func stripMethodPrefix(pattern string) string {
	if i := strings.IndexByte(pattern, ' '); i >= 0 {
		return pattern[i+1:]
	}
	return pattern
}

// allowedMethods returns the list of methods registered for the pattern that
// matches req's URL path, regardless of method.
func (r *Router) allowedMethods(req *http.Request) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for pattern, methods := range r.methodsByPattern {
		probe := req.Clone(req.Context())
		probe.Method = http.MethodGet
		// Re-resolve through mux to honor the same matching semantics; if any
		// method matches the path we report that pattern's methods.
		if matched := r.patternMatchesUnsafe(probe, pattern); matched {
			out := make([]string, 0, len(methods))
			for m := range methods {
				out = append(out, m)
			}
			sort.Strings(out)
			return out
		}
	}
	return nil
}

// patternMatchesUnsafe checks whether req's URL path matches pattern via mux
// resolution. Caller must hold r.mu.
func (r *Router) patternMatchesUnsafe(req *http.Request, pattern string) bool {
	for method := range r.methodsByPattern[pattern] {
		probe := req.Clone(req.Context())
		probe.Method = method
		_, matched := r.mux.Handler(probe)
		if stripMethodPrefix(matched) == pattern {
			return true
		}
	}
	return false
}

// serveHEADFromGET swaps a HEAD request to GET, runs the GET handler, and
// discards the body bytes — the headers and status are preserved.
func (r *Router) serveHEADFromGET(w http.ResponseWriter, req *http.Request, _ string) {
	getReq := req.Clone(req.Context())
	getReq.Method = http.MethodGet
	r.mux.ServeHTTP(noBodyWriter{ResponseWriter: w}, getReq)
}

// noBodyWriter discards body writes; headers and status pass through.
type noBodyWriter struct {
	http.ResponseWriter
}

func (noBodyWriter) Write(p []byte) (int, error) { return len(p), nil }

// appendMethod returns a sorted copy of methods with method added (if not
// already present). The input slice is not mutated.
func appendMethod(methods []string, method string) []string {
	for _, m := range methods {
		if m == method {
			return methods
		}
	}
	out := make([]string, 0, len(methods)+1)
	out = append(out, methods...)
	out = append(out, method)
	sort.Strings(out)
	return out
}

// ListenAndServe starts an HTTP server on the given address.
// It blocks until the context is cancelled, then shuts down gracefully.
func (r *Router) ListenAndServe(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

// addRoute registers a routeInfo with the router's mux and stores it
// for OpenAPI generation. Global middleware is applied in ServeHTTP,
// not here — only group middleware is baked into ri.handler.
func (r *Router) addRoute(ri routeInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.mux.Handle(ri.method+" "+ri.pattern, ri.handler)
	r.routes = append(r.routes, ri)

	if r.methodsByPattern[ri.pattern] == nil {
		r.methodsByPattern[ri.pattern] = make(map[string]struct{})
	}
	r.methodsByPattern[ri.pattern][ri.method] = struct{}{}
}
