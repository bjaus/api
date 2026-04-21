package api

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"time"
)

// Resp is a body-only response type parameterized by the body value.
// Use it when a handler needs to return just a body and none of the
// declarative metadata (status override, response headers, cookies).
// The T type parameter drives emission the same way a Body field's
// type does on a hand-written response:
//
//	func(...) (*api.Resp[User], error)           // JSON/XML body
//	func(...) (*api.Resp[io.Reader], error)      // streamed body
//	func(...) (*api.Resp[<-chan api.Event], error) // SSE body
//
// For responses that also carry status, headers, or cookies, declare
// a custom response struct with tagged fields plus a Body field.
type Resp[T any] struct {
	Body T
}

// encodeResponse writes a non-error handler response to w using the
// route's precomputed descriptor. It applies cookies, headers, resolves
// status, and dispatches the body by kind.
func encodeResponse(
	w http.ResponseWriter,
	r *http.Request,
	resp any,
	desc *responseDescriptor,
	defaultStatus int,
	codecs *codecRegistry,
) {
	rv := reflect.ValueOf(resp)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}

	status := defaultStatus
	if desc.status != nil {
		if s := intFieldValue(rv.FieldByIndex(desc.status.index)); s != 0 {
			status = s
		}
	}

	for _, ck := range desc.cookies {
		fv := rv.FieldByIndex(ck.index)
		c, ok := fv.Interface().(Cookie)
		if !ok || c.IsZero() {
			continue
		}
		http.SetCookie(w, c.ToHTTPCookie(ck.name))
	}

	for _, h := range desc.headers {
		fv := rv.FieldByIndex(h.index)
		values := headerFieldValues(fv)
		for _, v := range values {
			if v == "" {
				continue
			}
			w.Header().Add(h.name, v)
		}
	}

	if isNoBodyStatus(status) || desc.body == nil {
		w.WriteHeader(status)
		return
	}

	bv := rv.FieldByIndex(desc.body.index)

	switch desc.body.kind {
	case bodyKindCodec:
		writeCodecBody(w, r, bv, status, codecs)
	case bodyKindReader:
		writeReaderBody(w, bv, status)
	case bodyKindChan:
		writeChanBody(r.Context(), w, bv, status)
	}
}

// isNoBodyStatus reports whether the HTTP status requires an empty body
// per RFC 9110 §6.4.1 (1xx informational, 204 No Content, 304 Not Modified).
func isNoBodyStatus(status int) bool {
	return (status >= 100 && status < 200) || status == http.StatusNoContent || status == http.StatusNotModified
}

// writeCodecBody encodes a value via the negotiated response codec.
func writeCodecBody(w http.ResponseWriter, r *http.Request, bv reflect.Value, status int, codecs *codecRegistry) {
	enc, _ := codecs.negotiate(r.Header.Get("Accept"))
	w.Header().Set("Content-Type", enc.ContentType())
	w.WriteHeader(status)
	//nolint:errcheck,gosec // best-effort after WriteHeader
	enc.Encode(w, bv.Interface())
}

// writeReaderBody copies bytes from an io.Reader body to w.
func writeReaderBody(w http.ResponseWriter, bv reflect.Value, status int) {
	if bv.IsNil() {
		w.WriteHeader(status)
		return
	}
	w.WriteHeader(status)
	reader := bv.Interface().(io.Reader) //nolint:errcheck,forcetypeassert // descriptor guarantees io.Reader
	//nolint:errcheck,gosec // best-effort streaming copy
	io.Copy(w, reader)
}

// writeChanBody consumes events from a channel and emits them as SSE. It
// exits when the channel closes or the request context is cancelled.
func writeChanBody(ctx context.Context, w http.ResponseWriter, bv reflect.Value, status int) {
	if bv.IsNil() {
		w.WriteHeader(status)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(status)

	flusher, _ := w.(http.Flusher) //nolint:errcheck // ok being false means no flushing

	for {
		chosen, recv, ok := reflect.Select([]reflect.SelectCase{
			{Dir: reflect.SelectRecv, Chan: bv},
			{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ctx.Done())},
		})
		if chosen == 1 || !ok {
			return
		}
		ev := recv.Interface().(Event) //nolint:errcheck,forcetypeassert // descriptor guarantees chan Event
		//nolint:errcheck,gosec // best-effort SSE write
		writeEvent(w, ev)
		if flusher != nil {
			flusher.Flush()
		}
	}
}

// intFieldValue extracts an int from a field that may be any signed or
// unsigned integer kind. Returns 0 for non-integer fields.
func intFieldValue(fv reflect.Value) int {
	//exhaustive:ignore
	switch fv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return int(fv.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int(fv.Uint())
	default:
		return 0
	}
}

// headerFieldValues converts a field value to one or more strings suitable
// for a response header. Supports string, []string, integer kinds, and
// time.Time (RFC 1123 format).
func headerFieldValues(fv reflect.Value) []string {
	//exhaustive:ignore
	switch fv.Kind() {
	case reflect.String:
		return []string{fv.String()}
	case reflect.Slice:
		if fv.Type().Elem().Kind() == reflect.String {
			out := make([]string, fv.Len())
			for i := range fv.Len() {
				out[i] = fv.Index(i).String()
			}
			return out
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return []string{strconv.FormatInt(fv.Int(), 10)}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return []string{strconv.FormatUint(fv.Uint(), 10)}
	case reflect.Struct:
		if t, ok := fv.Interface().(time.Time); ok {
			if t.IsZero() {
				return nil
			}
			return []string{t.UTC().Format(http.TimeFormat)}
		}
	}
	return nil
}

// mergeErr combines a route's scope-level error template with the
// inline state carried by the handler-returned *Err. Scalar options
// (message, body mapper, cause) from the inline state replace the
// template; map options (headers, cookies keyed by name) replace
// entries with the same key; list options (details) accumulate with
// template entries first.
func mergeErr(template, inline *Err) *Err {
	final := &Err{code: inline.code}

	if inline.message != "" {
		final.message = inline.message
	} else if template != nil {
		final.message = template.message
	}

	if inline.body != nil {
		final.body = inline.body
	} else if template != nil {
		final.body = template.body
	}

	if inline.cause != nil {
		final.cause = inline.cause
	} else if template != nil {
		final.cause = template.cause
	}

	if template != nil {
		for name, values := range template.headers {
			if final.headers == nil {
				final.headers = http.Header{}
			}
			final.headers[name] = append([]string{}, values...)
		}
		for name, c := range template.cookies {
			if final.cookies == nil {
				final.cookies = make(map[string]Cookie, len(template.cookies))
			}
			final.cookies[name] = c
		}
		final.details = append(final.details, template.details...)
	}

	for name, values := range inline.headers {
		if final.headers == nil {
			final.headers = http.Header{}
		}
		final.headers[name] = append([]string{}, values...)
	}
	for name, c := range inline.cookies {
		if final.cookies == nil {
			final.cookies = make(map[string]Cookie)
		}
		final.cookies[name] = c
	}
	final.details = append(final.details, inline.details...)

	return final
}

// errInfoView wraps *Err with the request URI so body mappers can read
// ErrorInfo.Instance() without the *Err itself knowing about HTTP. The
// framework constructs this view at emission time.
//
//nolint:errname // internal view type, not a distinct error.
type errInfoView struct {
	*Err
	instance string
}

func (v *errInfoView) Instance() string { return v.instance }

// emitErr renders a fully-resolved *Err to the response writer. Status
// comes from the Code. Cookies and headers are written first, then body
// (if any) is emitted via the configured mapper.
func emitErr(w http.ResponseWriter, r *http.Request, e *Err, codecs *codecRegistry) {
	status := e.code.HTTPStatus()

	for name, c := range e.cookies {
		http.SetCookie(w, c.ToHTTPCookie(name))
	}
	for name, values := range e.headers {
		for _, v := range values {
			w.Header().Add(name, v)
		}
	}

	// HTTP: 1xx, 204, 304 carry no body.
	if isNoBodyStatus(status) || e.body == nil {
		w.WriteHeader(status)
		return
	}

	info := &errInfoView{Err: e, instance: r.URL.RequestURI()}
	rv, skip := e.body.produce(r.Context(), info)
	if skip {
		w.WriteHeader(status)
		return
	}

	// Dispatch by the mapper's element type. *string → text/plain;
	// everything else → negotiated codec.
	if e.body.elemType().Kind() == reflect.String {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(status)
		s, _ := rv.Elem().Interface().(string) //nolint:errcheck // kind guarantees string
		//nolint:errcheck,gosec // best-effort after WriteHeader
		io.WriteString(w, s)
		return
	}

	bodyVal := rv.Interface()

	// Error bodies always emit regardless of whether the request's
	// Accept header matched a codec — clients benefit from seeing the
	// error even when their Accept is too narrow. Negotiate first, but
	// fall back to the router's default codec if negotiation failed.
	enc, ok := codecs.negotiate(r.Header.Get("Accept"))
	if !ok || enc == nil {
		enc = codecs.defaultEncoder()
	}
	contentType := enc.ContentType()
	// If the body value declares its own content type (e.g. ProblemDetails
	// emits application/problem+json per RFC 9457), honor it.
	if ct, ok := bodyVal.(interface{ ContentType() string }); ok {
		contentType = ct.ContentType()
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	//nolint:errcheck,gosec // best-effort after WriteHeader
	enc.Encode(w, bodyVal)
}
