package api

import (
	"reflect"
	"strconv"
	"strings"
	"time"
)

// JSONSchema represents a JSON Schema object (subset for OpenAPI 3.1).
type JSONSchema struct {
	Type            string                `json:"type,omitempty"`
	Format          string                `json:"format,omitempty"`
	ContentEncoding string                `json:"contentEncoding,omitempty"`
	Properties      map[string]JSONSchema `json:"properties,omitempty"`
	Items           *JSONSchema           `json:"items,omitempty"`
	Required        []string              `json:"required,omitempty"`
	Description     string                `json:"description,omitempty"`
	Enum            []string              `json:"enum,omitempty"`
	Ref             string                `json:"$ref,omitempty"`

	// AdditionalProperties can be true (any) or a schema.
	AdditionalProperties *JSONSchema `json:"additionalProperties,omitempty"`

	// Constraints.
	MinLength *int     `json:"minLength,omitempty"`
	MaxLength *int     `json:"maxLength,omitempty"`
	Minimum   *float64 `json:"minimum,omitempty"`
	Maximum   *float64 `json:"maximum,omitempty"`
	Pattern   string   `json:"pattern,omitempty"`
	MinItems  *int     `json:"minItems,omitempty"`
	MaxItems  *int     `json:"maxItems,omitempty"`

	// Defaults and examples.
	Default any `json:"default,omitempty"`
	Example any `json:"example,omitempty"`

	// Composition.
	OneOf         []JSONSchema   `json:"oneOf,omitempty"`
	AnyOf         []JSONSchema   `json:"anyOf,omitempty"`
	AllOf         []JSONSchema   `json:"allOf,omitempty"`
	Discriminator *Discriminator `json:"discriminator,omitempty"`

	// Extensions.
	Extensions map[string]any `json:"extensions,omitempty"`
}

// Discriminator maps a property to schema references for polymorphic types.
type Discriminator struct {
	PropertyName string            `json:"propertyName"`
	Mapping      map[string]string `json:"mapping,omitempty"`
}

// typeToSchema converts a reflect.Type to a JSONSchema.
func typeToSchema(t reflect.Type) JSONSchema {
	// Unwrap pointer.
	if t.Kind() == reflect.Pointer {
		return typeToSchema(t.Elem())
	}

	// Handle well-known types.
	switch t {
	case reflect.TypeFor[time.Time]():
		return JSONSchema{Type: "string", Format: "date-time"}
	case reflect.TypeFor[time.Duration]():
		return JSONSchema{Type: "string", Format: "duration"}
	case reflect.TypeFor[Void]():
		return JSONSchema{}
	case reflect.TypeFor[Stream](), reflect.TypeFor[SSEStream]():
		return JSONSchema{}
	case reflect.TypeFor[FileUpload]():
		return JSONSchema{Type: "string", Format: "binary"}
	}

	//exhaustive:ignore
	switch t.Kind() {
	case reflect.String:
		return JSONSchema{Type: "string"}
	case reflect.Bool:
		return JSONSchema{Type: "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return JSONSchema{Type: "integer"}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return JSONSchema{Type: "integer"}
	case reflect.Float32, reflect.Float64:
		return JSONSchema{Type: "number"}
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return JSONSchema{Type: "string", ContentEncoding: "base64"}
		}
		items := typeToSchema(t.Elem())
		return JSONSchema{Type: "array", Items: &items}
	case reflect.Array:
		items := typeToSchema(t.Elem())
		return JSONSchema{Type: "array", Items: &items}
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return JSONSchema{Type: "object"}
		}
		valSchema := typeToSchema(t.Elem())
		return JSONSchema{Type: "object", AdditionalProperties: &valSchema}
	case reflect.Struct:
		return structToSchema(t)
	case reflect.Interface:
		return JSONSchema{}
	default:
		return JSONSchema{}
	}
}

// structToSchema converts a struct type to a JSONSchema with properties.
func structToSchema(t reflect.Type) JSONSchema {
	schema := JSONSchema{
		Type:       "object",
		Properties: make(map[string]JSONSchema),
	}

	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}

		// Skip param/binding fields — they're not part of the body schema.
		if isParamField(f) {
			continue
		}

		// Skip embedded RawRequest.
		if f.Type == reflect.TypeFor[RawRequest]() {
			continue
		}

		name := jsonFieldName(f)
		if name == "-" {
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

// jsonFieldName returns the JSON field name for a struct field.
func jsonFieldName(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	if tag == "" {
		return f.Name
	}
	name, _, _ := strings.Cut(tag, ",")
	if name == "" {
		return f.Name
	}
	return name
}

// isParamField reports whether a struct field has parameter binding tags
// or form binding tags (form-tagged fields are part of the multipart body,
// not the JSON body schema).
func isParamField(f reflect.StructField) bool {
	for _, tag := range paramTags {
		if f.Tag.Get(tag) != "" {
			return true
		}
	}
	return f.Tag.Get("form") != ""
}

const errorSchemaName = "ProblemDetail"

// errorResponseSchema returns the JSON Schema for RFC 9457 ProblemDetail.
func errorResponseSchema() JSONSchema {
	return JSONSchema{
		Type: "object",
		Properties: map[string]JSONSchema{
			"type":     {Type: "string", Description: "URI reference identifying the problem type"},
			"title":    {Type: "string", Description: "Human-readable summary"},
			"status":   {Type: "integer", Description: "HTTP status code"},
			"detail":   {Type: "string", Description: "Explanation specific to this occurrence"},
			"instance": {Type: "string", Description: "URI reference to the specific occurrence"},
			"errors": {
				Type: "array",
				Items: &JSONSchema{
					Type: "object",
					Properties: map[string]JSONSchema{
						"field":   {Type: "string"},
						"message": {Type: "string"},
						"value":   {},
					},
					Required: []string{"field", "message"},
				},
				Description: "Field-level validation errors",
			},
		},
		Required: []string{"status"},
	}
}

// schemaRegistry deduplicates named types via $ref during spec generation.
type schemaRegistry struct {
	schemas map[reflect.Type]string
	defs    map[string]JSONSchema
}

func newSchemaRegistry() *schemaRegistry {
	return &schemaRegistry{
		schemas: make(map[reflect.Type]string),
		defs:    make(map[string]JSONSchema),
	}
}

// typeToSchema converts a reflect.Type to a JSONSchema, registering named types.
func (r *schemaRegistry) typeToSchema(t reflect.Type) JSONSchema {
	if t.Kind() == reflect.Pointer {
		return r.typeToSchema(t.Elem())
	}

	// Well-known types — return directly, no registration.
	switch t {
	case reflect.TypeFor[time.Time]():
		return JSONSchema{Type: "string", Format: "date-time"}
	case reflect.TypeFor[time.Duration]():
		return JSONSchema{Type: "string", Format: "duration"}
	case reflect.TypeFor[Void]():
		return JSONSchema{}
	case reflect.TypeFor[Stream](), reflect.TypeFor[SSEStream]():
		return JSONSchema{}
	case reflect.TypeFor[FileUpload]():
		return JSONSchema{Type: "string", Format: "binary"}
	}

	// Check SchemaProvider interface.
	if t.Kind() == reflect.Struct {
		ptr := reflect.New(t)
		if sp, ok := ptr.Interface().(SchemaProvider); ok {
			return sp.JSONSchema()
		}
	}

	// Primitives — return directly.
	//exhaustive:ignore
	switch t.Kind() {
	case reflect.String:
		return JSONSchema{Type: "string"}
	case reflect.Bool:
		return JSONSchema{Type: "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return JSONSchema{Type: "integer"}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return JSONSchema{Type: "integer"}
	case reflect.Float32, reflect.Float64:
		return JSONSchema{Type: "number"}
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return JSONSchema{Type: "string", ContentEncoding: "base64"}
		}
		items := r.typeToSchema(t.Elem())
		return JSONSchema{Type: "array", Items: &items}
	case reflect.Array:
		items := r.typeToSchema(t.Elem())
		return JSONSchema{Type: "array", Items: &items}
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return JSONSchema{Type: "object"}
		}
		valSchema := r.typeToSchema(t.Elem())
		return JSONSchema{Type: "object", AdditionalProperties: &valSchema}
	case reflect.Struct:
		// Named struct → register and return $ref.
		if t.Name() != "" {
			name := t.Name()
			if _, exists := r.schemas[t]; !exists {
				// Register name before recursing to handle circular refs.
				r.schemas[t] = name
				schema := r.structToSchema(t)
				// Apply SchemaTransformer if implemented.
				ptr := reflect.New(t)
				if st, ok := ptr.Interface().(SchemaTransformer); ok {
					schema = st.TransformSchema(schema)
				}
				r.defs[name] = schema
			}
			return JSONSchema{Ref: "#/components/schemas/" + name}
		}
		// Anonymous struct → inline.
		return r.structToSchema(t)
	case reflect.Interface:
		return JSONSchema{}
	default:
		return JSONSchema{}
	}
}

// structToSchema converts a struct type to a JSONSchema with properties.
func (r *schemaRegistry) structToSchema(t reflect.Type) JSONSchema {
	schema := JSONSchema{
		Type:       "object",
		Properties: make(map[string]JSONSchema),
	}

	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}

		if isParamField(f) {
			continue
		}

		if f.Type == reflect.TypeFor[RawRequest]() {
			continue
		}

		name := jsonFieldName(f)
		if name == "-" {
			continue
		}

		prop := r.typeToSchema(f.Type)

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

// SchemaProvider is implemented by types that control their own JSON Schema.
type SchemaProvider interface {
	JSONSchema() JSONSchema
}

// SchemaTransformer is implemented by types that modify the auto-generated schema.
type SchemaTransformer interface {
	TransformSchema(s JSONSchema) JSONSchema
}

// applyConstraintTags reads constraint struct tags and applies them to the schema.
func applyConstraintTags(schema *JSONSchema, f reflect.StructField) {
	if v := f.Tag.Get("minLength"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			schema.MinLength = &n
		}
	}
	if v := f.Tag.Get("maxLength"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			schema.MaxLength = &n
		}
	}
	if v := f.Tag.Get("minimum"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			schema.Minimum = &n
		}
	}
	if v := f.Tag.Get("maximum"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			schema.Maximum = &n
		}
	}
	if v := f.Tag.Get("pattern"); v != "" {
		schema.Pattern = v
	}
	if v := f.Tag.Get("minItems"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			schema.MinItems = &n
		}
	}
	if v := f.Tag.Get("maxItems"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			schema.MaxItems = &n
		}
	}
	if v := f.Tag.Get("enum"); v != "" {
		schema.Enum = strings.Split(v, ",")
	}
	if v := f.Tag.Get("default"); v != "" {
		schema.Default = v
	}
	if v := f.Tag.Get("example"); v != "" {
		schema.Example = v
	}
}
