package api

import (
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

// OpenAPISpec is the top-level OpenAPI 3.1 document.
type OpenAPISpec struct {
	OpenAPI string              `json:"openapi"`
	Info    OpenAPIInfo         `json:"info"`
	Paths   map[string]PathItem `json:"paths"`
}

// OpenAPIInfo holds API metadata.
type OpenAPIInfo struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

// PathItem maps HTTP methods to operations.
type PathItem map[string]Operation

// Operation describes a single API operation on a path.
type Operation struct {
	Summary     string        `json:"summary,omitempty"`
	Description string        `json:"description,omitempty"`
	Tags        []string      `json:"tags,omitempty"`
	OperationID string        `json:"operationId,omitempty"`
	Parameters  []Parameter   `json:"parameters,omitempty"`
	RequestBody *RequestBody  `json:"requestBody,omitempty"`
	Responses   OperationResp `json:"responses"`
	Deprecated  bool          `json:"deprecated,omitempty"`
}

// Parameter describes a single operation parameter.
type Parameter struct {
	Name        string     `json:"name"`
	In          string     `json:"in"`
	Description string     `json:"description,omitempty"`
	Required    bool       `json:"required,omitempty"`
	Schema      JSONSchema `json:"schema"`
}

// RequestBody describes the request body.
type RequestBody struct {
	Required bool                `json:"required"`
	Content  map[string]MediaObj `json:"content"`
}

// MediaObj is a media type object with an optional schema.
type MediaObj struct {
	Schema *JSONSchema `json:"schema,omitempty"`
}

// OperationResp maps HTTP status codes to response objects.
type OperationResp map[string]ResponseObj

// ResponseObj describes a single response.
type ResponseObj struct {
	Description string              `json:"description"`
	Content     map[string]MediaObj `json:"content,omitempty"`
}

// Spec generates the full OpenAPI 3.1 specification from registered routes.
func (r *Router) Spec() OpenAPISpec {
	spec := OpenAPISpec{
		OpenAPI: "3.1.0",
		Info: OpenAPIInfo{
			Title:   r.title,
			Version: r.version,
		},
		Paths: make(map[string]PathItem),
	}

	for i := range r.routes {
		ri := &r.routes[i]
		path := toOpenAPIPath(ri.pattern)
		method := strings.ToLower(ri.method)

		op := buildOperation(ri)

		if spec.Paths[path] == nil {
			spec.Paths[path] = make(PathItem)
		}
		spec.Paths[path][method] = op
	}

	return spec
}

// buildOperation creates an Operation from a routeInfo.
func buildOperation(ri *routeInfo) Operation {
	op := Operation{
		Summary:     ri.summary,
		Description: ri.desc,
		Tags:        ri.tags,
		Deprecated:  ri.deprecated,
		Responses:   make(OperationResp),
	}

	// Build parameters and request body from Req type.
	if ri.reqType != nil && ri.reqType != reflect.TypeFor[Void]() {
		op.Parameters = extractParameters(ri.reqType)
		op.RequestBody = extractRequestBody(ri.reqType, ri.method)
	}

	// Build response.
	status := ri.status
	if status == 0 {
		status = http.StatusOK
	}

	switch {
	case ri.respType == nil || ri.respType == reflect.TypeFor[Void]():
		if status == 0 || status == http.StatusOK {
			status = http.StatusNoContent
		}
		op.Responses[statusToString(status)] = ResponseObj{
			Description: "No content",
		}

	case ri.respType == reflect.TypeFor[Stream]():
		op.Responses[statusToString(status)] = ResponseObj{
			Description: "Successful response",
			Content: map[string]MediaObj{
				"application/octet-stream": {},
			},
		}

	case ri.respType == reflect.TypeFor[SSEStream]():
		op.Responses[statusToString(status)] = ResponseObj{
			Description: "Successful response",
			Content: map[string]MediaObj{
				"text/event-stream": {Schema: &JSONSchema{Type: "string"}},
			},
		}

	default:
		respSchema := typeToSchema(ri.respType)
		op.Responses[statusToString(status)] = ResponseObj{
			Description: "Successful response",
			Content: map[string]MediaObj{
				"application/json": {Schema: &respSchema},
			},
		}
	}

	return op
}

// extractParameters builds OpenAPI parameters from param-tagged fields.
func extractParameters(t reflect.Type) []Parameter {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}

	var params []Parameter
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}

		for _, tagName := range paramTags {
			val := f.Tag.Get(tagName)
			if val == "" {
				continue
			}

			p := Parameter{
				Name:   val,
				In:     tagToIn(tagName),
				Schema: typeToSchema(f.Type),
			}

			if doc := f.Tag.Get("doc"); doc != "" {
				p.Description = doc
			}

			if f.Tag.Get("required") == "true" || tagName == "path" {
				p.Required = true
			}

			params = append(params, p)
		}
	}

	return params
}

// extractRequestBody builds an OpenAPI RequestBody if the request type has a body.
func extractRequestBody(t reflect.Type, method string) *RequestBody {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}

	// Has Body field → body is the Body field's type.
	if bodyField, ok := t.FieldByName("Body"); ok {
		schema := typeToSchema(bodyField.Type)
		return &RequestBody{
			Required: true,
			Content: map[string]MediaObj{
				"application/json": {Schema: &schema},
			},
		}
	}

	// No param tags → entire struct is body (only for POST/PUT/PATCH).
	if !hasParamTags(t) && (method == "POST" || method == "PUT" || method == "PATCH") {
		schema := typeToSchema(t)
		return &RequestBody{
			Required: true,
			Content: map[string]MediaObj{
				"application/json": {Schema: &schema},
			},
		}
	}

	return nil
}

// tagToIn converts a struct tag name to the OpenAPI "in" field.
func tagToIn(tag string) string {
	//exhaustive:ignore
	switch tag {
	case "path":
		return "path"
	case "query":
		return "query"
	case "header":
		return "header"
	case "cookie":
		return "cookie"
	default:
		return tag
	}
}

// toOpenAPIPath converts a Go 1.22 pattern like "/users/{id}" to
// an OpenAPI path. Strips the method prefix and wildcard suffixes.
func toOpenAPIPath(pattern string) string {
	// Go's mux patterns can include {name...} for wildcards.
	// OpenAPI uses {name} without the ellipsis.
	result := strings.ReplaceAll(pattern, "...", "")
	return result
}

// statusToString converts an HTTP status code to its string representation.
func statusToString(code int) string {
	return strconv.Itoa(code)
}
