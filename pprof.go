package api

import "net/http/pprof"

// Pprof registers pprof profiling endpoints under the given prefix.
// Default prefix is "/debug/pprof". Routes are hidden from the OpenAPI spec.
func Pprof(r *Router, prefix string) {
	if prefix == "" {
		prefix = "/debug/pprof"
	}

	r.mux.HandleFunc("GET "+prefix+"/", pprof.Index)
	r.mux.HandleFunc("GET "+prefix+"/cmdline", pprof.Cmdline)
	r.mux.HandleFunc("GET "+prefix+"/profile", pprof.Profile)
	r.mux.HandleFunc("GET "+prefix+"/symbol", pprof.Symbol)
	r.mux.HandleFunc("GET "+prefix+"/trace", pprof.Trace)
	r.mux.Handle("GET "+prefix+"/goroutine", pprof.Handler("goroutine"))
	r.mux.Handle("GET "+prefix+"/heap", pprof.Handler("heap"))
	r.mux.Handle("GET "+prefix+"/allocs", pprof.Handler("allocs"))
	r.mux.Handle("GET "+prefix+"/block", pprof.Handler("block"))
	r.mux.Handle("GET "+prefix+"/mutex", pprof.Handler("mutex"))
	r.mux.Handle("GET "+prefix+"/threadcreate", pprof.Handler("threadcreate"))
}
