package api

import (
	"io/fs"
	"net/http"
)

// Static serves static files from the given filesystem under the URL path.
// The route is hidden from the OpenAPI spec.
func (r *Router) Static(urlPath string, fsys fs.FS) {
	handler := http.StripPrefix(urlPath, http.FileServerFS(fsys))
	r.mux.Handle("GET "+urlPath+"/{path...}", handler)
}
