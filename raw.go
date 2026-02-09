package api

import "net/http"

// RawRequest can be embedded in a request type to get access to
// the underlying *http.Request.
type RawRequest struct {
	Request *http.Request
}

// OperationInfo provides OpenAPI metadata for raw handlers that the
// framework cannot infer from types.
type OperationInfo struct {
	Summary     string
	Description string
	Tags        []string
	Status      int
}
