package api

import "reflect"

// Test-only exports for internal functions.
var (
	HasParamTags  = hasParamTags
	HasFormTags   = hasFormTags
	HasBodyField  = hasBodyField
	HasRawRequest = hasRawRequest
	TagOptions    = tagOptions
	TagContains   = tagContains

	TypeToSchema        = typeToSchema
	StructToSchema      = structToSchema
	JSONFieldName       = jsonFieldName
	ApplyConstraintTags = applyConstraintTags

	ErrorResponseSchema = errorResponseSchema
	ErrorSchemaName     = errorSchemaName

	ValidateConstraints = validateConstraints
	GenerateOperationID = generateOperationID

	WriteEvent = writeEvent
)

// BuildResponseDescriptor exposes the internal descriptor builder to tests,
// wrapped so the external test package can inspect it without importing
// unexported types.
func BuildResponseDescriptor(t reflect.Type) (*ResponseDescriptor, error) {
	d, err := buildResponseDescriptor(t)
	if err != nil {
		return nil, err
	}
	return &ResponseDescriptor{desc: d}, nil
}

// Exported body kind constants for tests.
const (
	BodyKindNone   = bodyKindNone
	BodyKindCodec  = bodyKindCodec
	BodyKindReader = bodyKindReader
	BodyKindChan   = bodyKindChan
)

// BodyKind is the exported name for bodyKind in tests.
type BodyKind = bodyKind

// ResponseDescriptor is the exported wrapper for tests.
type ResponseDescriptor struct {
	desc *responseDescriptor
}

// HasStatus reports whether the descriptor tracks a status field.
func (r *ResponseDescriptor) HasStatus() bool { return r.desc.status != nil }

// HeaderNames returns the ordered header names the descriptor emits.
func (r *ResponseDescriptor) HeaderNames() []string {
	out := make([]string, len(r.desc.headers))
	for i, h := range r.desc.headers {
		out[i] = h.name
	}
	return out
}

// CookieNames returns the ordered cookie names the descriptor emits.
func (r *ResponseDescriptor) CookieNames() []string {
	out := make([]string, len(r.desc.cookies))
	for i, c := range r.desc.cookies {
		out[i] = c.name
	}
	return out
}

// BodyKind returns the body emission kind, or BodyKindNone if no Body field.
func (r *ResponseDescriptor) BodyKind() BodyKind {
	if r.desc.body == nil {
		return BodyKindNone
	}
	return r.desc.body.kind
}

// TestSchemaRegistry wraps schemaRegistry for external tests.
type TestSchemaRegistry struct {
	reg  *schemaRegistry
	Defs map[string]JSONSchema
}

// NewSchemaRegistry creates a TestSchemaRegistry for testing.
func NewSchemaRegistry() *TestSchemaRegistry {
	r := newSchemaRegistry()
	return &TestSchemaRegistry{reg: r, Defs: r.defs}
}

// TypeToSchema delegates to the internal registry.
func (t *TestSchemaRegistry) TypeToSchema(typ reflect.Type) JSONSchema {
	return t.reg.typeToSchema(typ)
}
