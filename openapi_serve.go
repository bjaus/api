package api

import (
	"encoding/json"
	"net/http"
)

// ServeSpec registers a GET handler at the given path that serves
// the OpenAPI spec as JSON.
func (r *Router) ServeSpec(pattern string) {
	r.mux.HandleFunc("GET "+pattern, func(w http.ResponseWriter, req *http.Request) {
		spec := r.Spec()
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck,gosec // best-effort after WriteHeader
		json.NewEncoder(w).Encode(spec)
	})
}
