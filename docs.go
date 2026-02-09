package api

import (
	"html/template"
	"net/http"
)

// DocsOption configures the docs UI.
type DocsOption func(*docsConfig)

type docsConfig struct {
	title   string
	specURL string
}

// WithDocsTitle sets the page title for the docs UI.
func WithDocsTitle(title string) DocsOption {
	return func(c *docsConfig) {
		c.title = title
	}
}

// ServeDocs serves an interactive API documentation UI at the given path.
// It renders Stoplight Elements pointing at the router's OpenAPI spec.
func (r *Router) ServeDocs(path string, opts ...DocsOption) {
	cfg := &docsConfig{
		title:   r.title,
		specURL: "/openapi.json",
	}
	for _, opt := range opts {
		opt(cfg)
	}

	tmpl := template.Must(template.New("docs").Parse(docsHTML))

	r.mux.HandleFunc("GET "+path, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		//nolint:errcheck,gosec // best-effort template render
		tmpl.Execute(w, cfg)
	})
}

const docsHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <link rel="stylesheet" href="https://unpkg.com/@stoplight/elements/styles.min.css">
  <script src="https://unpkg.com/@stoplight/elements/web-components.min.js"></script>
</head>
<body>
  <elements-api
    apiDescriptionUrl="{{.SpecURL}}"
    router="hash"
    layout="sidebar"
  />
</body>
</html>`

// Title returns the docs config title (used in the template).
func (c *docsConfig) Title() string { return c.title }

// SpecURL returns the docs config spec URL (used in the template).
func (c *docsConfig) SpecURL() string { return c.specURL }
