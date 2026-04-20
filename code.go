package api

import "net/http"

// Code classifies an HTTP error outcome. Each Code maps to a canonical
// HTTP status (always >= 400). Use the package-level Code* constants
// rather than constructing a Code from a literal string — unknown values
// fall back to 500 Internal Server Error.
type Code string

// 4xx client error codes.
const (
	CodeBadRequest                    Code = "bad_request"                      // 400
	CodeUnauthorized                  Code = "unauthorized"                     // 401
	CodePaymentRequired               Code = "payment_required"                 // 402
	CodeForbidden                     Code = "forbidden"                        // 403
	CodeNotFound                      Code = "not_found"                        // 404
	CodeMethodNotAllowed              Code = "method_not_allowed"               // 405
	CodeNotAcceptable                 Code = "not_acceptable"                   // 406
	CodeProxyAuthRequired             Code = "proxy_authentication_required"    // 407
	CodeRequestTimeout                Code = "request_timeout"                  // 408
	CodeConflict                      Code = "conflict"                         // 409
	CodeGone                          Code = "gone"                             // 410
	CodeLengthRequired                Code = "length_required"                  // 411
	CodePreconditionFailed            Code = "precondition_failed"              // 412
	CodeContentTooLarge               Code = "content_too_large"                // 413
	CodeURITooLong                    Code = "uri_too_long"                     // 414
	CodeUnsupportedMediaType          Code = "unsupported_media_type"           // 415
	CodeRangeNotSatisfiable           Code = "range_not_satisfiable"            // 416
	CodeExpectationFailed             Code = "expectation_failed"               // 417
	CodeTeapot                        Code = "teapot"                           // 418
	CodeMisdirectedRequest            Code = "misdirected_request"              // 421
	CodeUnprocessableContent          Code = "unprocessable_content"            // 422
	CodeLocked                        Code = "locked"                           // 423
	CodeFailedDependency              Code = "failed_dependency"                // 424
	CodeTooEarly                      Code = "too_early"                        // 425
	CodeUpgradeRequired               Code = "upgrade_required"                 // 426
	CodePreconditionRequired          Code = "precondition_required"            // 428
	CodeTooManyRequests               Code = "too_many_requests"                // 429
	CodeRequestHeaderFieldsTooLarge   Code = "request_header_fields_too_large"  // 431
	CodeUnavailableForLegalReasons    Code = "unavailable_for_legal_reasons"    // 451
)

// 5xx server error codes.
const (
	CodeInternal                      Code = "internal"                         // 500
	CodeNotImplemented                Code = "not_implemented"                  // 501
	CodeBadGateway                    Code = "bad_gateway"                      // 502
	CodeServiceUnavailable            Code = "service_unavailable"              // 503
	CodeGatewayTimeout                Code = "gateway_timeout"                  // 504
	CodeHTTPVersionNotSupported       Code = "http_version_not_supported"       // 505
	CodeVariantAlsoNegotiates         Code = "variant_also_negotiates"          // 506
	CodeInsufficientStorage           Code = "insufficient_storage"             // 507
	CodeLoopDetected                  Code = "loop_detected"                    // 508
	CodeNotExtended                   Code = "not_extended"                     // 510
	CodeNetworkAuthenticationRequired Code = "network_authentication_required"  // 511
)

// codeToStatus is the canonical mapping from Code to HTTP status.
var codeToStatus = map[Code]int{
	CodeBadRequest:                    http.StatusBadRequest,
	CodeUnauthorized:                  http.StatusUnauthorized,
	CodePaymentRequired:               http.StatusPaymentRequired,
	CodeForbidden:                     http.StatusForbidden,
	CodeNotFound:                      http.StatusNotFound,
	CodeMethodNotAllowed:              http.StatusMethodNotAllowed,
	CodeNotAcceptable:                 http.StatusNotAcceptable,
	CodeProxyAuthRequired:             http.StatusProxyAuthRequired,
	CodeRequestTimeout:                http.StatusRequestTimeout,
	CodeConflict:                      http.StatusConflict,
	CodeGone:                          http.StatusGone,
	CodeLengthRequired:                http.StatusLengthRequired,
	CodePreconditionFailed:            http.StatusPreconditionFailed,
	CodeContentTooLarge:               http.StatusRequestEntityTooLarge,
	CodeURITooLong:                    http.StatusRequestURITooLong,
	CodeUnsupportedMediaType:          http.StatusUnsupportedMediaType,
	CodeRangeNotSatisfiable:           http.StatusRequestedRangeNotSatisfiable,
	CodeExpectationFailed:             http.StatusExpectationFailed,
	CodeTeapot:                        http.StatusTeapot,
	CodeMisdirectedRequest:            http.StatusMisdirectedRequest,
	CodeUnprocessableContent:          http.StatusUnprocessableEntity,
	CodeLocked:                        http.StatusLocked,
	CodeFailedDependency:              http.StatusFailedDependency,
	CodeTooEarly:                      http.StatusTooEarly,
	CodeUpgradeRequired:               http.StatusUpgradeRequired,
	CodePreconditionRequired:          http.StatusPreconditionRequired,
	CodeTooManyRequests:               http.StatusTooManyRequests,
	CodeRequestHeaderFieldsTooLarge:   http.StatusRequestHeaderFieldsTooLarge,
	CodeUnavailableForLegalReasons:    http.StatusUnavailableForLegalReasons,

	CodeInternal:                      http.StatusInternalServerError,
	CodeNotImplemented:                http.StatusNotImplemented,
	CodeBadGateway:                    http.StatusBadGateway,
	CodeServiceUnavailable:            http.StatusServiceUnavailable,
	CodeGatewayTimeout:                http.StatusGatewayTimeout,
	CodeHTTPVersionNotSupported:       http.StatusHTTPVersionNotSupported,
	CodeVariantAlsoNegotiates:         http.StatusVariantAlsoNegotiates,
	CodeInsufficientStorage:           http.StatusInsufficientStorage,
	CodeLoopDetected:                  http.StatusLoopDetected,
	CodeNotExtended:                   http.StatusNotExtended,
	CodeNetworkAuthenticationRequired: http.StatusNetworkAuthenticationRequired,
}

// HTTPStatus returns the canonical HTTP status code for the Code. Codes
// not in the registered set return 500.
func (c Code) HTTPStatus() int {
	if s, ok := codeToStatus[c]; ok {
		return s
	}
	return http.StatusInternalServerError
}

// IsRegistered reports whether c is one of the package-defined Codes.
func (c Code) IsRegistered() bool {
	_, ok := codeToStatus[c]
	return ok
}
