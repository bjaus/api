package api

import (
	"reflect"
	"strings"
	"time"
)

// JSONSchema represents a JSON Schema object (subset for OpenAPI 3.1).
type JSONSchema struct {
	Type        string                `json:"type,omitempty"`
	Format      string                `json:"format,omitempty"`
	Properties  map[string]JSONSchema `json:"properties,omitempty"`
	Items       *JSONSchema           `json:"items,omitempty"`
	Required    []string              `json:"required,omitempty"`
	Description string                `json:"description,omitempty"`
	Enum        []string              `json:"enum,omitempty"`
	Ref         string                `json:"$ref,omitempty"`

	// AdditionalProperties can be true (any) or a schema.
	AdditionalProperties *JSONSchema `json:"additionalProperties,omitempty"`
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
	case reflect.TypeFor[Stream]():
		return JSONSchema{Type: "string", Format: "binary"}
	case reflect.TypeFor[SSEStream]():
		return JSONSchema{Type: "string", Format: "event-stream"}
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
			return JSONSchema{Type: "string", Format: "byte"}
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

// isParamField reports whether a struct field has parameter binding tags.
func isParamField(f reflect.StructField) bool {
	for _, tag := range paramTags {
		if f.Tag.Get(tag) != "" {
			return true
		}
	}
	return false
}
