package api

import (
	"encoding/json"
	"errors"
	"net/http"
)

// CookieSetter is optionally implemented by response types to set cookies.
type CookieSetter interface {
	Cookies() []*http.Cookie
}

// HeaderSetter is optionally implemented by response types to set response headers.
type HeaderSetter interface {
	SetHeaders(h http.Header)
}

// Redirect is returned from a handler to issue an HTTP redirect.
type Redirect struct {
	URL    string
	Status int
}

// encodeResponse writes the response to the http.ResponseWriter.
// It handles Void (204), Redirect, Stream, SSEStream, CookieSetter, HeaderSetter,
// StatusCoder, and negotiated encoding.
func encodeResponse(w http.ResponseWriter, r *http.Request, resp any, defaultStatus int, codecs *codecRegistry) {
	// Redirect response.
	if rd, ok := resp.(*Redirect); ok {
		status := rd.Status
		if status == 0 {
			status = http.StatusFound
		}
		http.Redirect(w, r, rd.URL, status)
		return
	}

	// Stream response — caller controls content type and body.
	if s, ok := resp.(*Stream); ok {
		writeStream(w, s)
		return
	}

	// SSE stream — long-lived event stream.
	if s, ok := resp.(*SSEStream); ok {
		writeSSEStream(w, s)
		return
	}

	// Apply cookies and headers before writing status.
	if cs, ok := resp.(CookieSetter); ok {
		for _, c := range cs.Cookies() {
			http.SetCookie(w, c)
		}
	}
	if hs, ok := resp.(HeaderSetter); ok {
		hs.SetHeaders(w.Header())
	}

	status := defaultStatus

	// Let the response override the status dynamically.
	if sc, ok := resp.(StatusCoder); ok {
		status = sc.StatusCode()
	}

	// Negotiate response encoder from Accept header.
	enc, _ := codecs.negotiate(r.Header.Get("Accept"))

	w.Header().Set("Content-Type", enc.ContentType())
	w.WriteHeader(status)
	//nolint:errcheck,gosec // best-effort after WriteHeader
	enc.Encode(w, resp)
}

// writeErrorResponse writes an error as an RFC 9457 problem details response.
func writeErrorResponse(w http.ResponseWriter, err error) {
	status := ErrorStatus(err)

	// If the error is already a ProblemDetail, use it directly.
	var pd *ProblemDetail
	if ok := isProblemDetail(err, &pd); ok {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(pd.Status)
		//nolint:errcheck,errchkjson,gosec // best-effort after WriteHeader
		json.NewEncoder(w).Encode(pd)
		return
	}

	// Convert any error into a ProblemDetail.
	problem := &ProblemDetail{
		Type:   "about:blank",
		Title:  http.StatusText(status),
		Status: status,
		Detail: err.Error(),
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	//nolint:errcheck,errchkjson,gosec // best-effort after WriteHeader
	json.NewEncoder(w).Encode(problem)
}

func isProblemDetail(err error, target **ProblemDetail) bool {
	return errors.As(err, target)
}
