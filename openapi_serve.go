package api

import (
	"encoding/json"
	"io"
	"net/http"

	"gopkg.in/yaml.v3"
)

// ServeSpec registers a GET handler at the given path that serves
// the OpenAPI spec as JSON.
func (r *Router) ServeSpec(pattern string) {
	r.mux.HandleFunc("GET "+pattern, func(w http.ResponseWriter, _ *http.Request) {
		spec := r.Spec()
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck,gosec // best-effort after WriteHeader
		json.NewEncoder(w).Encode(spec)
	})
}

// ServeSpecYAML registers a GET handler at the given path that serves
// the OpenAPI spec as YAML.
func (r *Router) ServeSpecYAML(pattern string) {
	r.mux.HandleFunc("GET "+pattern, func(w http.ResponseWriter, _ *http.Request) {
		spec := r.Spec()
		w.Header().Set("Content-Type", "application/yaml")
		//nolint:errcheck,gosec // best-effort after WriteHeader
		yaml.NewEncoder(w).Encode(spec)
	})
}

// WriteSpec writes the OpenAPI spec as indented JSON to w.
func (r *Router) WriteSpec(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r.Spec())
}

// WriteSpecYAML writes the OpenAPI spec as YAML to w.
func (r *Router) WriteSpecYAML(w io.Writer) error {
	return yaml.NewEncoder(w).Encode(r.Spec())
}
