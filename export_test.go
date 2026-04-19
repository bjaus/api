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
)

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
