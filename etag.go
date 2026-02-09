package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
)

// ETagConfig configures the ETag middleware.
type ETagConfig struct {
	Weak bool // use weak ETags
}

// LastModifier is implemented by response types that report their last modification time.
type LastModifier interface {
	LastModified() time.Time
}

// ETag returns middleware that handles conditional requests via ETag and If-None-Match.
func ETag(cfg ...ETagConfig) Middleware {
	c := ETagConfig{}
	if len(cfg) > 0 {
		c = cfg[0]
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only apply to GET/HEAD.
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				next.ServeHTTP(w, r)
				return
			}

			buf := &bytes.Buffer{}
			rec := &etagRecorder{
				ResponseWriter: w,
				buf:            buf,
				status:         http.StatusOK,
			}

			next.ServeHTTP(rec, r)

			// Only compute etag for 2xx responses.
			if rec.status < 200 || rec.status >= 300 {
				w.WriteHeader(rec.status)
				//nolint:errcheck,gosec // best-effort write
				w.Write(buf.Bytes())
				return
			}

			hash := sha256.Sum256(buf.Bytes())
			etag := `"` + hex.EncodeToString(hash[:8]) + `"`
			if c.Weak {
				etag = "W/" + etag
			}

			w.Header().Set("ETag", etag)

			// Check If-None-Match.
			if match := r.Header.Get("If-None-Match"); match != "" {
				if strings.Contains(match, etag) {
					w.WriteHeader(http.StatusNotModified)
					return
				}
			}

			// Check If-Match (for PUT/DELETE, returns 412).
			if match := r.Header.Get("If-Match"); match != "" {
				if !strings.Contains(match, etag) && match != "*" {
					w.WriteHeader(http.StatusPreconditionFailed)
					return
				}
			}

			w.WriteHeader(rec.status)
			//nolint:errcheck,gosec // best-effort write
			w.Write(buf.Bytes())
		})
	}
}

type etagRecorder struct {
	http.ResponseWriter
	buf    *bytes.Buffer
	status int
}

func (e *etagRecorder) WriteHeader(code int) {
	e.status = code
}

func (e *etagRecorder) Write(b []byte) (int, error) {
	return e.buf.Write(b)
}
