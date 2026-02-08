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

	validator Validator

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
