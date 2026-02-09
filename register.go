package api

import (
	"net/http"
	"reflect"
)

// Registrar is the interface accepted by the registration functions.
// Both *Router and *Group implement it.
type Registrar interface {
	addRoute(ri routeInfo)
	getValidator() Validator
	getErrorHandler() ErrorHandler
	routeMiddleware() []Middleware
}

func (r *Router) getValidator() Validator     { return r.validator }
func (r *Router) getErrorHandler() ErrorHandler { return r.errorHandler }
func (r *Router) routeMiddleware() []Middleware { return nil }

// register is the internal generic registration function.
func register[Req, Resp any](reg Registrar, method, pattern string, h Handler[Req, Resp], opts ...RouteOption) {
	ri := routeInfo{
		method:   method,
		pattern:  pattern,
		reqType:  reflect.TypeFor[Req](),
		respType: reflect.TypeFor[Resp](),
	}

	for _, opt := range opts {
		opt(&ri)
	}

	// Determine default status: Void response → 204, otherwise 200.
	if ri.status == 0 {
		if ri.respType == reflect.TypeFor[Void]() {
			ri.status = http.StatusNoContent
		} else {
			ri.status = http.StatusOK
		}
	}

	validator := reg.getValidator()
	errHandler := reg.getErrorHandler()
	routeMW := reg.routeMiddleware()

	ri.handler = buildHandler(h, ri.status, validator, errHandler)

	// Apply route-level middleware (from Group).
	for i := len(routeMW) - 1; i >= 0; i-- {
		ri.handler = routeMW[i](ri.handler)
	}

	reg.addRoute(ri)
}

// buildHandler wraps a typed Handler into an http.Handler.
func buildHandler[Req, Resp any](h Handler[Req, Resp], defaultStatus int, validator Validator, errHandler ErrorHandler) http.Handler {
	writeErr := func(w http.ResponseWriter, r *http.Request, err error) {
		if errHandler != nil {
			errHandler(w, r, err)
			return
		}
		writeErrorResponse(w, err)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, err := decodeRequest[Req](r)
		if err != nil {
			writeErr(w, r, Error(http.StatusBadRequest, err.Error()))
			return
		}

		// Run constraint validation on struct tags.
		if err := validateConstraints(req); err != nil {
			writeErr(w, r, err)
			return
		}

		// Run SelfValidator if implemented.
		if sv, ok := any(req).(SelfValidator); ok {
			if err := sv.Validate(); err != nil {
				writeErr(w, r, err)
				return
			}
		}

		// Run global validator if set.
		if validator != nil {
			if err := validator.Validate(req); err != nil {
				writeErr(w, r, err)
				return
			}
		}

		resp, err := h(r.Context(), req)
		if err != nil {
			writeErr(w, r, err)
			return
		}

		// Void response.
		if _, ok := any(resp).(*Void); ok || resp == nil {
			w.WriteHeader(defaultStatus)
			return
		}

		encodeResponse(w, resp, defaultStatus)
	})
}

// Get registers a GET handler.
func Get[Req, Resp any](reg Registrar, pattern string, h Handler[Req, Resp], opts ...RouteOption) {
	register(reg, http.MethodGet, pattern, h, opts...)
}

// Post registers a POST handler.
func Post[Req, Resp any](reg Registrar, pattern string, h Handler[Req, Resp], opts ...RouteOption) {
	register(reg, http.MethodPost, pattern, h, opts...)
}

// Put registers a PUT handler.
func Put[Req, Resp any](reg Registrar, pattern string, h Handler[Req, Resp], opts ...RouteOption) {
	register(reg, http.MethodPut, pattern, h, opts...)
}

// Patch registers a PATCH handler.
func Patch[Req, Resp any](reg Registrar, pattern string, h Handler[Req, Resp], opts ...RouteOption) {
	register(reg, http.MethodPatch, pattern, h, opts...)
}

// Delete registers a DELETE handler.
func Delete[Req, Resp any](reg Registrar, pattern string, h Handler[Req, Resp], opts ...RouteOption) {
	register(reg, http.MethodDelete, pattern, h, opts...)
}

// Raw registers a raw http.Handler with manual OperationInfo for the OpenAPI spec.
func Raw(reg Registrar, method, pattern string, h RawHandler, info OperationInfo) {
	ri := routeInfo{
		method:  method,
		pattern: pattern,
		summary: info.Summary,
		desc:    info.Description,
		tags:    info.Tags,
		status:  info.Status,
		handler: http.HandlerFunc(h),
	}

	routeMW := reg.routeMiddleware()
	for i := len(routeMW) - 1; i >= 0; i-- {
		ri.handler = routeMW[i](ri.handler)
	}

	reg.addRoute(ri)
}
