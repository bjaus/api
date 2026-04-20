package api

// Envelope is the framework's default JSON error body shape. It
// combines the error's semantic code, human message, and any attached
// typed details into one object. Use ErrorBodyEnvelope as the mapper in
// WithErrorBody to opt routes (or the whole router) into this shape.
type Envelope struct {
	Code    Code   `json:"code"`
	Message string `json:"message,omitempty"`
	Details []any  `json:"details,omitempty"`
}

// ErrorBodyEnvelope is a ready-to-use body mapper that produces the
// framework's default Envelope shape from an ErrorInfo. Pass it to
// WithErrorBody when you want a consistent JSON envelope across all
// error responses:
//
//	api.New(api.WithError(api.WithErrorBody(api.ErrorBodyEnvelope)))
func ErrorBodyEnvelope(e ErrorInfo) *Envelope {
	return &Envelope{
		Code:    e.Code(),
		Message: e.Message(),
		Details: e.Details(),
	}
}

// ErrorBodyText is a ready-to-use body mapper that renders the error's
// message as a text/plain response body. Use when an API expects plain
// string errors rather than JSON envelopes:
//
//	api.New(api.WithError(api.WithErrorBody(api.ErrorBodyText)))
func ErrorBodyText(e ErrorInfo) *string {
	return new(e.Message())
}
