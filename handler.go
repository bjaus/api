package api

import (
	"context"
	"net/http"
)

// Void is used as a type parameter when a request has no parameters/body
// or a response has no body (results in 204 No Content).
type Void struct{}

// Handler is the core typed handler signature. The framework owns
// serialization — handlers never see http.ResponseWriter or *http.Request.
type Handler[Req, Resp any] func(ctx context.Context, req *Req) (*Resp, error)

// RawHandler is an escape hatch for WebSocket upgrades, SSE, or anything
// that needs direct access to the underlying http primitives.
type RawHandler func(w http.ResponseWriter, r *http.Request)
