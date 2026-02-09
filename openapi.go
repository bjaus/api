package api

import (
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// Link describes an OpenAPI link object.
type Link struct {
	OperationID string         `json:"operationId,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	Description string         `json:"description,omitempty"`
}

// HeaderObj describes a response header for OpenAPI.
type HeaderObj struct {
	Description string     `json:"description,omitempty"`
	Schema      JSONSchema `json:"schema"`
}

// ResponseHeaders is implemented by response types that declare headers in the OpenAPI spec.
type ResponseHeaders interface {
	ResponseHeaders() map[string]HeaderObj
}

// OpenAPISpec is the top-level OpenAPI 3.1 document.
type OpenAPISpec struct {
	OpenAPI    string                `json:"openapi"`
	Info       OpenAPIInfo           `json:"info"`
	Servers    []Server              `json:"servers,omitempty"`
	Paths      map[string]PathItem   `json:"paths"`
	Components *Components           `json:"components,omitempty"`
	Tags       []TagObj              `json:"tags,omitempty"`
	Security   []SecurityRequirement `json:"security,omitempty"`
	Webhooks   map[string]PathItem   `json:"webhooks,omitempty"`
	Extensions map[string]any        `json:"extensions,omitempty"`
}

// Server describes an API server.
type Server struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

// TagObj describes a tag with an optional description.
type TagObj struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// SecurityRequirement maps security scheme names to required scopes.
type SecurityRequirement map[string][]string

// SecurityScheme describes an OpenAPI security scheme.
type SecurityScheme struct {
	Type             string      `json:"type"`
	Description      string      `json:"description,omitempty"`
	Name             string      `json:"name,omitempty"`
	In               string      `json:"in,omitempty"`
	Scheme           string      `json:"scheme,omitempty"`
	BearerFormat     string      `json:"bearerFormat,omitempty"`
	Flows            *OAuthFlows `json:"flows,omitempty"`
	OpenIDConnectURL string      `json:"openIdConnectUrl,omitempty"`
}

// OAuthFlows describes the available OAuth2 flows.
type OAuthFlows struct {
	Implicit          *OAuthFlow `json:"implicit,omitempty"`
	Password          *OAuthFlow `json:"password,omitempty"`
	ClientCredentials *OAuthFlow `json:"clientCredentials,omitempty"`
	AuthorizationCode *OAuthFlow `json:"authorizationCode,omitempty"`
}

// OAuthFlow describes an OAuth2 flow.
type OAuthFlow struct {
	AuthorizationURL string            `json:"authorizationUrl,omitempty"`
	TokenURL         string            `json:"tokenUrl,omitempty"`
	RefreshURL       string            `json:"refreshUrl,omitempty"`
	Scopes           map[string]string `json:"scopes,omitempty"`
}

// Components holds reusable schema definitions and security schemes.
type Components struct {
	Schemas         map[string]JSONSchema      `json:"schemas,omitempty"`
	SecuritySchemes map[string]SecurityScheme   `json:"securitySchemes,omitempty"`
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
	Summary     string                         `json:"summary,omitempty"`
	Description string                         `json:"description,omitempty"`
	Tags        []string                       `json:"tags,omitempty"`
	OperationID string                         `json:"operationId,omitempty"`
	Parameters  []Parameter                    `json:"parameters,omitempty"`
	RequestBody *RequestBody                   `json:"requestBody,omitempty"`
	Responses   OperationResp                  `json:"responses"`
	Deprecated  bool                           `json:"deprecated,omitempty"`
	Security    *[]SecurityRequirement         `json:"security,omitempty"`
	Callbacks   map[string]map[string]PathItem `json:"callbacks,omitempty"`
	Extensions  map[string]any                 `json:"extensions,omitempty"`
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
	Description string                `json:"description"`
	Content     map[string]MediaObj   `json:"content,omitempty"`
	Headers     map[string]HeaderObj  `json:"headers,omitempty"`
	Links       map[string]Link       `json:"links,omitempty"`
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

	if len(r.servers) > 0 {
		spec.Servers = r.servers
	}

	if len(r.security) > 0 {
		for _, name := range r.security {
			spec.Security = append(spec.Security, SecurityRequirement{name: {}})
		}
	}

	if len(r.tagDescs) > 0 {
		names := make([]string, 0, len(r.tagDescs))
		for name := range r.tagDescs {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			spec.Tags = append(spec.Tags, TagObj{Name: name, Description: r.tagDescs[name]})
		}
	}

	reg := newSchemaRegistry()
	reg.defs[errorSchemaName] = errorResponseSchema()

	for i := range r.routes {
		ri := &r.routes[i]
		path := toOpenAPIPath(ri.pattern)
		method := strings.ToLower(ri.method)

		op := buildOperation(ri, reg)

		if spec.Paths[path] == nil {
			spec.Paths[path] = make(PathItem)
		}
		spec.Paths[path][method] = op
	}

	comp := &Components{Schemas: reg.defs}
	if len(r.securitySchemes) > 0 {
		comp.SecuritySchemes = r.securitySchemes
	}
	spec.Components = comp

	if len(r.webhooks) > 0 {
		spec.Webhooks = r.webhooks
	}

	return spec
}

// buildOperation creates an Operation from a routeInfo.
func buildOperation(ri *routeInfo, reg *schemaRegistry) Operation {
	op := Operation{
		Summary:     ri.summary,
		Description: ri.desc,
		Tags:        ri.tags,
		Deprecated:  ri.deprecated,
		Responses:   make(OperationResp),
	}

	if ri.operationID != "" {
		op.OperationID = ri.operationID
	} else {
		op.OperationID = generateOperationID(ri.method, ri.pattern)
	}

	if ri.noSecurity {
		empty := make([]SecurityRequirement, 0)
		op.Security = &empty
	} else if len(ri.security) > 0 {
		reqs := make([]SecurityRequirement, 0, len(ri.security))
		for _, name := range ri.security {
			reqs = append(reqs, SecurityRequirement{name: {}})
		}
		op.Security = &reqs
	}

	// Build parameters and request body from Req type.
	if ri.reqType != nil && ri.reqType != reflect.TypeFor[Void]() {
		op.Parameters = extractParameters(ri.reqType)
		op.RequestBody = extractRequestBody(ri.reqType, ri.method, reg)
	}

	// Build success response.
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
		respSchema := reg.typeToSchema(ri.respType)
		op.Responses[statusToString(status)] = ResponseObj{
			Description: "Successful response",
			Content: map[string]MediaObj{
				"application/json": {Schema: &respSchema},
			},
		}
	}

	// Build error responses.
	errorCodes := map[int]struct{}{
		http.StatusBadRequest:          {},
		http.StatusInternalServerError: {},
	}
	if strings.Contains(ri.pattern, "{") {
		errorCodes[http.StatusNotFound] = struct{}{}
	}
	for _, code := range ri.errors {
		errorCodes[code] = struct{}{}
	}

	errRef := JSONSchema{Ref: "#/components/schemas/" + errorSchemaName}
	for code := range errorCodes {
		op.Responses[statusToString(code)] = ResponseObj{
			Description: http.StatusText(code),
			Content: map[string]MediaObj{
				"application/json": {Schema: &errRef},
			},
		}
	}

	// Add response headers if the response type implements ResponseHeaders.
	if ri.respType != nil && ri.respType.Kind() == reflect.Struct {
		ptr := reflect.New(ri.respType)
		if rh, ok := ptr.Interface().(ResponseHeaders); ok {
			statusKey := statusToString(status)
			if resp, exists := op.Responses[statusKey]; exists {
				resp.Headers = rh.ResponseHeaders()
				op.Responses[statusKey] = resp
			}
		}
	}

	// Add links to the success response.
	if len(ri.links) > 0 {
		statusKey := statusToString(status)
		if resp, exists := op.Responses[statusKey]; exists {
			resp.Links = ri.links
			op.Responses[statusKey] = resp
		}
	}

	// Add callbacks.
	if len(ri.callbacks) > 0 {
		op.Callbacks = ri.callbacks
	}

	// Add extensions.
	if len(ri.extensions) > 0 {
		op.Extensions = ri.extensions
	}

	return op
}

// extractParameters builds OpenAPI parameters from param-tagged fields.
func extractParameters(t reflect.Type) []Parameter {
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

			schema := typeToSchema(f.Type)
			applyConstraintTags(&schema, f)

			p := Parameter{
				Name:   val,
				In:     tagToIn(tagName),
				Schema: schema,
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
func extractRequestBody(t reflect.Type, method string, reg *schemaRegistry) *RequestBody {
	// Form-tagged struct → multipart/form-data.
	if hasFormTags(t) {
		schema := formFieldsToSchema(t)
		return &RequestBody{
			Required: true,
			Content: map[string]MediaObj{
				"multipart/form-data": {Schema: &schema},
			},
		}
	}

	// Has Body field → body is the Body field's type.
	if bodyField, ok := t.FieldByName("Body"); ok {
		schema := reg.typeToSchema(bodyField.Type)
		return &RequestBody{
			Required: true,
			Content: map[string]MediaObj{
				"application/json": {Schema: &schema},
			},
		}
	}

	// No param tags → entire struct is body (only for POST/PUT/PATCH).
	if !hasParamTags(t) && (method == "POST" || method == "PUT" || method == "PATCH") {
		schema := reg.typeToSchema(t)
		return &RequestBody{
			Required: true,
			Content: map[string]MediaObj{
				"application/json": {Schema: &schema},
			},
		}
	}

	return nil
}

// formFieldsToSchema builds a JSONSchema from form-tagged fields.
func formFieldsToSchema(t reflect.Type) JSONSchema {
	schema := JSONSchema{
		Type:       "object",
		Properties: make(map[string]JSONSchema),
	}

	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}

		name := f.Tag.Get("form")
		if name == "" {
			continue
		}

		prop := typeToSchema(f.Type)

		if doc := f.Tag.Get("doc"); doc != "" {
			prop.Description = doc
		}

		applyConstraintTags(&prop, f)

		schema.Properties[name] = prop

		if f.Tag.Get("required") == "true" {
			schema.Required = append(schema.Required, name)
		}
	}

	return schema
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
	default: // cookie
		return "cookie"
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

// generateOperationID creates an operationId from the HTTP method and pattern.
// Example: GET /v1/users/{id} → getV1UsersById.
func generateOperationID(method, pattern string) string {
	var b strings.Builder
	b.WriteString(strings.ToLower(method))

	parts := strings.Split(pattern, "/")
	for _, part := range parts {
		if part == "" {
			continue
		}
		// Path parameter: {id} → ById
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
			name = strings.TrimSuffix(name, "...")
			b.WriteString("By")
			b.WriteString(capitalize(name))
			continue
		}
		b.WriteString(capitalize(part))
	}

	return b.String()
}

// capitalize upper-cases the first letter of a string.
func capitalize(s string) string {
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
