package api

import (
	"context"
	"errors"
	"net/http"
	"reflect"
)

// Registrar is the interface accepted by the registration functions.
// Both *Router and *Group implement it.
type Registrar interface {
	addRoute(ri routeInfo)
	getValidator() ValidatorFunc
	getErrorHandler() ErrorHandler
	getMode() ValidationMode
	getCodecs() *codecRegistry
	getValidateResponses() bool
	routeMiddleware() []Middleware
	// errorOptionChain returns the scope's error-option list, outermost
	// first. For a Router this is just the router's own options; for a
	// Group it is the parent's chain followed by the group's own.
	errorOptionChain() []ErrorOption
}

func (r *Router) getValidator() ValidatorFunc     { return r.validator }
func (r *Router) getErrorHandler() ErrorHandler   { return r.errorHandler }
func (r *Router) getMode() ValidationMode         { return r.mode }
func (r *Router) getCodecs() *codecRegistry       { return r.codecs }
func (r *Router) getValidateResponses() bool      { return r.validateResponses }
func (r *Router) routeMiddleware() []Middleware   { return nil }
func (r *Router) errorOptionChain() []ErrorOption { return r.errorOpts }

// handlerConfig bundles the router-level configuration that buildHandler needs.
type handlerConfig struct {
	defaultStatus     int
	mode              ValidationMode
	validator         ValidatorFunc
	errHandler        ErrorHandler
	codecs            *codecRegistry
	requestDesc       *requestDescriptor
	responseDesc      *responseDescriptor
	errorTemplate     *Err
	validateResponses bool
}

// register is the internal generic registration function.
func register[Req, Resp any](reg Registrar, method, pattern string, h Handler[Req, Resp], opts ...RouteOption) {
	ri := routeInfo{
		method:   method,
		pattern:  pattern,
		reqType:  reflect.TypeFor[Req](),
		respType: reflect.TypeFor[Resp](),
		mode:     reg.getMode(),
	}

	for _, opt := range opts {
		opt.applyRoute(&ri)
	}

	// Determine default status: Void response → 204, otherwise 200.
	if ri.status == 0 {
		if ri.respType == reflect.TypeFor[Void]() {
			ri.status = http.StatusNoContent
		} else {
			ri.status = http.StatusOK
		}
	}

	// Void is a special "no response body" marker; it does not carry tags
	// and does not need descriptor-driven emission.
	if ri.respType != reflect.TypeFor[Void]() {
		d, err := buildResponseDescriptor(ri.respType)
		if err != nil {
			panic(err)
		}
		ri.responseDesc = d
	}

	reqDesc, err := buildRequestDescriptor(ri.reqType)
	if err != nil {
		panic(err)
	}
	ri.requestDesc = reqDesc

	// Merge scope error options: router chain → group chain → route options.
	// Apply them to a fresh *Err that serves as the per-route template.
	chain := reg.errorOptionChain()
	ri.errorTemplate = &Err{}
	for _, opt := range chain {
		opt.applyErr(ri.errorTemplate)
	}
	for _, opt := range ri.errorOpts {
		opt.applyErr(ri.errorTemplate)
	}
	// Default body mapper: RFC 9457 ProblemDetails. Consumers opt out
	// with WithoutErrorBody or override with WithErrorBody.
	if ri.errorTemplate.body == nil {
		ri.errorTemplate.body = &typedBodyMapper[ProblemDetails]{fn: ErrorBodyProblemDetails}
	}
	ri.errorCodes = append([]Code{}, ri.errorTemplate.documentedCodes...)

	cfg := handlerConfig{
		defaultStatus:     ri.status,
		mode:              ri.mode,
		validator:         reg.getValidator(),
		errHandler:        reg.getErrorHandler(),
		codecs:            reg.getCodecs(),
		requestDesc:       ri.requestDesc,
		responseDesc:      ri.responseDesc,
		errorTemplate:     ri.errorTemplate,
		validateResponses: reg.getValidateResponses(),
	}

	ri.handler = buildHandler(h, cfg)

	// Apply per-route body limit.
	if ri.bodyLimit > 0 {
		ri.handler = BodyLimit(ri.bodyLimit)(ri.handler)
	}

	// Apply route-level middleware (from Group).
	routeMW := reg.routeMiddleware()
	for i := len(routeMW) - 1; i >= 0; i-- {
		ri.handler = routeMW[i](ri.handler)
	}

	reg.addRoute(ri)
}

// buildHandler wraps a typed Handler into an http.Handler. The validation
// pipeline runs in the order dictated by cfg.mode; any returned
// ValidationErrors is routed through cfg.errBuilder.
func buildHandler[Req, Resp any](h Handler[Req, Resp], cfg handlerConfig) http.Handler {
	writeErr := func(w http.ResponseWriter, r *http.Request, err error) {
		// ValidationErrors convert to an *Err with each violation
		// attached as a detail.
		var ve ValidationErrors
		if errors.As(err, &ve) {
			opts := make([]ErrorOption, 0, len(ve)+1)
			opts = append(opts, WithMessage("validation failed"))
			for _, v := range ve {
				opts = append(opts, WithDetail(v))
			}
			err = Error(CodeUnprocessableContent, opts...)
		}

		// Consumer-provided ErrorHandler wins when set.
		if cfg.errHandler != nil {
			cfg.errHandler(w, r, err)
			return
		}

		// Classify the error. Non-*Err errors are wrapped as CodeInternal.
		var apiErr *Err
		if !errors.As(err, &apiErr) {
			apiErr = &Err{code: CodeInternal, message: err.Error(), cause: err}
		}
		emitErr(w, r, mergeErr(cfg.errorTemplate, apiErr), cfg.codecs)
	}

	runConstraints := func(req *Req) error {
		return validateConstraints(req)
	}

	runPerTypeValidator := func(ctx context.Context, req *Req) error {
		v, ok := any(req).(Validator)
		if !ok {
			return nil
		}
		return v.Validate(ctx)
	}

	runRouterValidator := func(req *Req) error {
		if cfg.validator == nil {
			return nil
		}
		return cfg.validator(req)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 406 Not Acceptable: if Accept is explicit and no encoder matches.
		if accept := r.Header.Get("Accept"); accept != "" {
			if _, ok := cfg.codecs.negotiate(accept); !ok {
				writeErr(w, r, Error(CodeNotAcceptable, WithMessage("unsupported Accept media type")))
				return
			}
		}

		req, err := decodeRequest[Req](r, cfg.codecs, cfg.requestDesc)
		if err != nil {
			writeErr(w, r, Error(CodeBadRequest, WithMessage(err.Error())))
			return
		}

		ctx, bgQ := withBackgroundQueue(r.Context())
		//nolint:contextcheck // background tasks are intentionally detached
		defer runBackgroundTasks(bgQ)

		steps := validationSteps(ctx, cfg.mode, req, runConstraints, runPerTypeValidator, runRouterValidator)
		for _, step := range steps {
			if err := step(); err != nil {
				writeErr(w, r, err)
				return
			}
		}

		resp, err := h(ctx, req)
		if err != nil {
			writeErr(w, r, err)
			return
		}

		// Void response.
		if _, ok := any(resp).(*Void); ok || resp == nil {
			w.WriteHeader(cfg.defaultStatus)
			return
		}

		if cfg.validateResponses {
			if err := validateConstraints(resp); err != nil {
				opts := []ErrorOption{WithMessage("response failed validation")}
				var ve ValidationErrors
				if errors.As(err, &ve) {
					for _, v := range ve {
						opts = append(opts, WithDetail(v))
					}
				}
				writeErr(w, r, Error(CodeInternal, opts...))
				return
			}
		}

		encodeResponse(w, r, resp, cfg.responseDesc, cfg.defaultStatus, cfg.codecs)
	})
}

// validationSteps returns the validation closures in the order dictated by
// the configured ValidationMode. Steps that don't apply (e.g., constraints
// when mode is Off) are omitted.
func validationSteps[Req any](
	ctx context.Context,
	mode ValidationMode,
	req *Req,
	runConstraints func(*Req) error,
	runPerType func(context.Context, *Req) error,
	runRouter func(*Req) error,
) []func() error {
	constraints := func() error { return runConstraints(req) }
	perType := func() error { return runPerType(ctx, req) }
	router := func() error { return runRouter(req) }

	switch mode {
	case ValidateConstraintsFirst:
		return []func() error{constraints, perType, router}
	case ValidateConstraintsOff:
		return []func() error{perType, router}
	default: // ValidateConstraintsLast
		return []func() error{perType, router, constraints}
	}
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
