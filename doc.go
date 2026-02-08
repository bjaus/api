// Package api is a generics-first HTTP API framework for Go. Handler types
// are the source of truth — request parameters, bodies, and responses are
// all expressed as Go types, and the framework derives serialization, param
// binding, and OpenAPI 3.1 specs from them automatically.
//
// The core handler signature removes http.ResponseWriter and *http.Request:
//
//	type Handler[Req, Resp any] func(ctx context.Context, req *Req) (*Resp, error)
//
// Routes are registered with package-level generic functions:
//
//	r := api.New(api.WithTitle("My API"), api.WithVersion("1.0.0"))
//	api.Get[ListReq, ListResp](r, "/items", listItems)
//	api.Post[CreateReq, Item](r, "/items", createItem, api.WithStatus(http.StatusCreated))
//
// Request types use struct tags for parameter binding and a Body field for
// request bodies:
//
//	type CreateReq struct {
//	    OrgID string `path:"org_id"`
//	    Body  struct {
//	        Name string `json:"name" required:"true"`
//	    }
//	}
//
// Middleware uses the standard func(http.Handler) http.Handler signature,
// so the entire Go middleware ecosystem works natively.
//
// OpenAPI 3.1 specs are generated from registered routes:
//
//	r.ServeSpec("/openapi.json")
package api
