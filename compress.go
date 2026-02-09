package api

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// CompressConfig configures the Compress middleware.
type CompressConfig struct {
	Level   int      // gzip level (1-9, default: 5)
	MinSize int      // minimum response size to compress (default: 1024)
	Types   []string // content types to compress (default: application/json, text/*)
}

// Compress returns middleware that gzip-compresses responses.
func Compress(cfg ...CompressConfig) Middleware {
	c := CompressConfig{
		Level:   5,
		MinSize: 1024,
		Types:   []string{"application/json", "text/"},
	}
	if len(cfg) > 0 {
		if cfg[0].Level > 0 {
			c.Level = cfg[0].Level
		}
		if cfg[0].MinSize > 0 {
			c.MinSize = cfg[0].MinSize
		}
		if len(cfg[0].Types) > 0 {
			c.Types = cfg[0].Types
		}
	}

	pool := &sync.Pool{
		New: func() any {
			gz, _ := gzip.NewWriterLevel(io.Discard, c.Level) //nolint:errcheck // level is pre-validated
			return gz
		},
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, r)
				return
			}

			gz := pool.Get().(*gzip.Writer) //nolint:errcheck,forcetypeassert // pool.New always returns *gzip.Writer
			gz.Reset(w)
			defer func() {
				//nolint:errcheck,gosec // best-effort flush
				gz.Close()
				pool.Put(gz)
			}()

			gw := &gzipResponseWriter{
				ResponseWriter: w,
				writer:         gz,
				minSize:        c.MinSize,
				types:          c.Types,
			}

			w.Header().Set("Vary", "Accept-Encoding")
			next.ServeHTTP(gw, r)

			// If gzip was activated, close and set headers.
			if gw.gzipActive {
				w.Header().Set("Content-Encoding", "gzip")
				w.Header().Del("Content-Length")
			}
		})
	}
}

type gzipResponseWriter struct {
	http.ResponseWriter
	writer     *gzip.Writer
	minSize    int
	types      []string
	gzipActive bool
	headerSent bool
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	if !g.headerSent {
		g.headerSent = true
		ct := g.Header().Get("Content-Type")
		if g.shouldCompress(ct) && len(b) >= g.minSize {
			g.gzipActive = true
			g.Header().Set("Content-Encoding", "gzip")
			g.Header().Del("Content-Length")
		}
	}

	if g.gzipActive {
		return g.writer.Write(b)
	}
	return g.ResponseWriter.Write(b)
}

func (g *gzipResponseWriter) shouldCompress(contentType string) bool {
	// Skip SSE and already-compressed responses.
	if strings.Contains(contentType, "event-stream") {
		return false
	}
	if g.Header().Get("Content-Encoding") != "" {
		return false
	}
	for _, t := range g.types {
		if strings.Contains(contentType, t) {
			return true
		}
	}
	return false
}

func (g *gzipResponseWriter) Unwrap() http.ResponseWriter {
	return g.ResponseWriter
}
