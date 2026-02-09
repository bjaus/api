package api

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// Router is the central type that holds routes, middleware, and configuration.
// It implements http.Handler.
type Router struct {
	mux        *http.ServeMux
	middleware []Middleware
	routes     []routeInfo

	title   string
	version string

	servers         []Server
	securitySchemes map[string]SecurityScheme
	security        []string
	tagDescs        map[string]string

	webhooks map[string]PathItem

	validator    Validator
	errorHandler ErrorHandler

	encoders []Encoder
	decoders []Decoder

	tracer SpanStarter

	mu sync.Mutex
}

// RouterOption configures a Router.
type RouterOption func(*Router)

// WithTitle sets the API title (used in OpenAPI spec).
func WithTitle(title string) RouterOption {
	return func(r *Router) {
		r.title = title
	}
}

// WithVersion sets the API version (used in OpenAPI spec).
func WithVersion(version string) RouterOption {
	return func(r *Router) {
		r.version = version
	}
}

// WithValidator sets a global request validator.
func WithValidator(v Validator) RouterOption {
	return func(r *Router) {
		r.validator = v
	}
}

// WithServers sets the OpenAPI servers array.
func WithServers(servers ...Server) RouterOption {
	return func(r *Router) {
		r.servers = servers
	}
}

// WithSecurityScheme registers a named security scheme for the OpenAPI spec.
func WithSecurityScheme(name string, scheme SecurityScheme) RouterOption {
	return func(r *Router) {
		if r.securitySchemes == nil {
			r.securitySchemes = make(map[string]SecurityScheme)
		}
		r.securitySchemes[name] = scheme
	}
}

// WithGlobalSecurity sets global security requirements by scheme name.
func WithGlobalSecurity(schemes ...string) RouterOption {
	return func(r *Router) {
		r.security = append(r.security, schemes...)
	}
}

// WithTagDescriptions sets tag descriptions for the OpenAPI spec.
func WithTagDescriptions(descs map[string]string) RouterOption {
	return func(r *Router) {
		r.tagDescs = descs
	}
}

// ErrorHandler is a custom error response writer.
type ErrorHandler func(w http.ResponseWriter, r *http.Request, err error)

// WithErrorHandler sets a custom error handler for the router.
func WithErrorHandler(h ErrorHandler) RouterOption {
	return func(r *Router) {
		r.errorHandler = h
	}
}

// WithEncoder registers an additional response encoder.
func WithEncoder(enc Encoder) RouterOption {
	return func(r *Router) {
		r.encoders = append(r.encoders, enc)
	}
}

// WithDecoder registers an additional request body decoder.
func WithDecoder(dec Decoder) RouterOption {
	return func(r *Router) {
		r.decoders = append(r.decoders, dec)
	}
}

// WithWebhook registers a webhook path item for the OpenAPI spec.
func WithWebhook(name string, item PathItem) RouterOption {
	return func(r *Router) {
		if r.webhooks == nil {
			r.webhooks = make(map[string]PathItem)
		}
		r.webhooks[name] = item
	}
}

// SpanStarter is a tracing hook interface for creating spans per request.
// Implement this with your preferred tracing backend (e.g., OpenTelemetry).
type SpanStarter interface {
	StartSpan(ctx context.Context, name string, attrs map[string]string) (context.Context, func())
}

// WithTracer sets a tracing hook for the router.
func WithTracer(s SpanStarter) RouterOption {
	return func(r *Router) {
		r.tracer = s
	}
}

// New creates a new Router with the given options.
func New(opts ...RouterOption) *Router {
	r := &Router{
		mux: http.NewServeMux(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Use adds middleware to the router. Middleware is applied in the order added.
func (r *Router) Use(mw ...Middleware) {
	r.middleware = append(r.middleware, mw...)
}

// ServeHTTP implements http.Handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	handler := http.Handler(r.mux)
	for i := len(r.middleware) - 1; i >= 0; i-- {
		handler = r.middleware[i](handler)
	}
	handler.ServeHTTP(w, req)
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
}
