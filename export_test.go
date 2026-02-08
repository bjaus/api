package api

// Test-only exports for internal functions.
var (
	HasParamTags  = hasParamTags
	HasBodyField  = hasBodyField
	HasRawRequest = hasRawRequest
	TagOptions    = tagOptions
	TagContains   = tagContains

	TypeToSchema   = typeToSchema
	StructToSchema = structToSchema
	JSONFieldName  = jsonFieldName

	ParseFileUpload = parseFileUpload
)
