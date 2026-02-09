package api_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/bjaus/api"
)

func TestTypeToSchema(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		typ    reflect.Type
		expect api.JSONSchema
	}{
		"string": {
			typ:    reflect.TypeFor[string](),
			expect: api.JSONSchema{Type: "string"},
		},
		"int": {
			typ:    reflect.TypeFor[int](),
			expect: api.JSONSchema{Type: "integer"},
		},
		"float64": {
			typ:    reflect.TypeFor[float64](),
			expect: api.JSONSchema{Type: "number"},
		},
		"bool": {
			typ:    reflect.TypeFor[bool](),
			expect: api.JSONSchema{Type: "boolean"},
		},
		"time.Time": {
			typ:    reflect.TypeFor[time.Time](),
			expect: api.JSONSchema{Type: "string", Format: "date-time"},
		},
		"time.Duration": {
			typ:    reflect.TypeFor[time.Duration](),
			expect: api.JSONSchema{Type: "string", Format: "duration"},
		},
		"Void": {
			typ:    reflect.TypeFor[api.Void](),
			expect: api.JSONSchema{},
		},
		"[]byte": {
			typ:    reflect.TypeFor[[]byte](),
			expect: api.JSONSchema{Type: "string", ContentEncoding: "base64"},
		},
		"[]string": {
			typ: reflect.TypeFor[[]string](),
			expect: api.JSONSchema{
				Type:  "array",
				Items: &api.JSONSchema{Type: "string"},
			},
		},
		"map[string]int": {
			typ: reflect.TypeFor[map[string]int](),
			expect: api.JSONSchema{
				Type:                 "object",
				AdditionalProperties: &api.JSONSchema{Type: "integer"},
			},
		},
		"pointer unwrap": {
			typ:    reflect.TypeFor[*string](),
			expect: api.JSONSchema{Type: "string"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := api.TypeToSchema(tc.typ)
			assert.Equal(t, tc.expect, got)
		})
	}
}

func TestStructToSchema(t *testing.T) {
	t.Parallel()

	type Example struct {
		Name  string `json:"name" required:"true" doc:"The name"`
		Email string `json:"email"`
		Age   int    `json:"age"`
	}

	schema := api.StructToSchema(reflect.TypeFor[Example]())

	assert.Equal(t, "object", schema.Type)
	assert.Len(t, schema.Properties, 3)
	assert.Equal(t, api.JSONSchema{Type: "string", Description: "The name"}, schema.Properties["name"])
	assert.Equal(t, api.JSONSchema{Type: "string"}, schema.Properties["email"])
	assert.Equal(t, api.JSONSchema{Type: "integer"}, schema.Properties["age"])
	assert.Equal(t, []string{"name"}, schema.Required)
}

func TestStructToSchema_skips_param_fields(t *testing.T) {
	t.Parallel()

	type Req struct {
		ID   string `path:"id"`
		Body struct {
			Name string `json:"name"`
		}
	}

	schema := api.StructToSchema(reflect.TypeFor[Req]())
	_, hasID := schema.Properties["ID"]
	assert.False(t, hasID, "path param fields should be excluded from body schema")
}

func TestSchemaRegistry_named_type_produces_ref(t *testing.T) {
	t.Parallel()

	type Thing struct {
		Name string `json:"name"`
	}

	reg := api.NewSchemaRegistry()
	schema := reg.TypeToSchema(reflect.TypeFor[Thing]())

	assert.Equal(t, "#/components/schemas/Thing", schema.Ref)
	assert.Contains(t, reg.Defs, "Thing")
	assert.Equal(t, "object", reg.Defs["Thing"].Type)
	assert.Contains(t, reg.Defs["Thing"].Properties, "name")
}

func TestSchemaRegistry_anonymous_struct_inlines(t *testing.T) {
	t.Parallel()

	reg := api.NewSchemaRegistry()
	typ := reflect.TypeOf(struct {
		X int `json:"x"`
	}{})

	schema := reg.TypeToSchema(typ)

	assert.Equal(t, "object", schema.Type)
	assert.Empty(t, schema.Ref)
	assert.Empty(t, reg.Defs)
}

func TestSchemaRegistry_dedup(t *testing.T) {
	t.Parallel()

	type Widget struct {
		ID string `json:"id"`
	}

	reg := api.NewSchemaRegistry()
	s1 := reg.TypeToSchema(reflect.TypeFor[Widget]())
	s2 := reg.TypeToSchema(reflect.TypeFor[Widget]())

	assert.Equal(t, s1, s2)
	assert.Len(t, reg.Defs, 1)
}

func TestSchemaRegistry_nested_named_types(t *testing.T) {
	t.Parallel()

	type Inner struct {
		Val string `json:"val"`
	}
	type Outer struct {
		Child Inner `json:"child"`
	}

	reg := api.NewSchemaRegistry()
	schema := reg.TypeToSchema(reflect.TypeFor[Outer]())

	assert.Equal(t, "#/components/schemas/Outer", schema.Ref)
	assert.Contains(t, reg.Defs, "Outer")
	assert.Contains(t, reg.Defs, "Inner")

	outerSchema := reg.Defs["Outer"]
	assert.Equal(t, "#/components/schemas/Inner", outerSchema.Properties["child"].Ref)
}

func TestSchemaRegistry_primitives_not_registered(t *testing.T) {
	t.Parallel()

	reg := api.NewSchemaRegistry()
	schema := reg.TypeToSchema(reflect.TypeFor[string]())

	assert.Equal(t, "string", schema.Type)
	assert.Empty(t, reg.Defs)
}

func TestErrorResponseSchema(t *testing.T) {
	t.Parallel()

	schema := api.ErrorResponseSchema()
	assert.Equal(t, "object", schema.Type)
	assert.Contains(t, schema.Properties, "status")
	assert.Contains(t, schema.Properties, "title")
	assert.Contains(t, schema.Properties, "detail")
	assert.Contains(t, schema.Properties, "errors")
	assert.Equal(t, []string{"status"}, schema.Required)
}

func TestJsonFieldName(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		field  reflect.StructField
		expect string
	}{
		"json tag": {
			field:  reflect.StructField{Name: "Email", Tag: `json:"email_addr"`},
			expect: "email_addr",
		},
		"json tag with options": {
			field:  reflect.StructField{Name: "Name", Tag: `json:"name,omitempty"`},
			expect: "name",
		},
		"no json tag": {
			field:  reflect.StructField{Name: "Title", Tag: ``},
			expect: "Title",
		},
		"json dash": {
			field:  reflect.StructField{Name: "Hidden", Tag: `json:"-"`},
			expect: "-",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expect, api.JSONFieldName(tc.field))
		})
	}
}

func TestJsonFieldName_empty_name_with_options(t *testing.T) {
	t.Parallel()

	f := reflect.StructField{Name: "Foo", Tag: `json:",omitempty"`}
	assert.Equal(t, "Foo", api.JSONFieldName(f))
}

func TestTypeToSchema_additional_kinds(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		typ    reflect.Type
		expect api.JSONSchema
	}{
		"Stream": {
			typ:    reflect.TypeFor[api.Stream](),
			expect: api.JSONSchema{},
		},
		"SSEStream": {
			typ:    reflect.TypeFor[api.SSEStream](),
			expect: api.JSONSchema{},
		},
		"FileUpload": {
			typ:    reflect.TypeFor[api.FileUpload](),
			expect: api.JSONSchema{Type: "string", Format: "binary"},
		},
		"array type": {
			typ: reflect.TypeFor[[3]int](),
			expect: api.JSONSchema{
				Type:  "array",
				Items: &api.JSONSchema{Type: "integer"},
			},
		},
		"map with non-string key": {
			typ:    reflect.TypeFor[map[int]string](),
			expect: api.JSONSchema{Type: "object"},
		},
		"interface type": {
			typ:    reflect.TypeFor[any](),
			expect: api.JSONSchema{},
		},
		"uint": {
			typ:    reflect.TypeFor[uint](),
			expect: api.JSONSchema{Type: "integer"},
		},
		"float32": {
			typ:    reflect.TypeFor[float32](),
			expect: api.JSONSchema{Type: "number"},
		},
		"unknown kind chan": {
			typ:    reflect.TypeFor[chan int](),
			expect: api.JSONSchema{},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := api.TypeToSchema(tc.typ)
			assert.Equal(t, tc.expect, got)
		})
	}
}

func TestStructToSchema_skips_json_dash(t *testing.T) {
	t.Parallel()

	type S struct {
		Visible string `json:"visible"`
		Hidden  string `json:"-"`
	}

	schema := api.StructToSchema(reflect.TypeFor[S]())
	assert.Len(t, schema.Properties, 1)
	_, hasVisible := schema.Properties["visible"]
	assert.True(t, hasVisible)
}

func TestStructToSchema_skips_unexported_fields(t *testing.T) {
	t.Parallel()

	schema := api.StructToSchema(reflect.TypeOf(struct {
		Public string `json:"public"`
		_      string
	}{}))
	assert.Len(t, schema.Properties, 1)
	_, hasPub := schema.Properties["public"]
	assert.True(t, hasPub)
}

func TestStructToSchema_omitempty_empty_name(t *testing.T) {
	t.Parallel()

	type S struct {
		FieldName string `json:",omitempty"`
	}

	schema := api.StructToSchema(reflect.TypeFor[S]())
	_, hasFieldName := schema.Properties["FieldName"]
	assert.True(t, hasFieldName)
}

func TestStructToSchema_skips_RawRequest(t *testing.T) {
	t.Parallel()

	type S struct {
		api.RawRequest
		Name string `json:"name"`
	}

	schema := api.StructToSchema(reflect.TypeFor[S]())
	assert.Len(t, schema.Properties, 1)
	_, hasName := schema.Properties["name"]
	assert.True(t, hasName)
}

func TestStructToSchema_doc_and_required(t *testing.T) {
	t.Parallel()

	type S struct {
		Title string `json:"title" doc:"The title" required:"true"`
	}

	schema := api.StructToSchema(reflect.TypeFor[S]())
	assert.Equal(t, "The title", schema.Properties["title"].Description)
	assert.Equal(t, []string{"title"}, schema.Required)
}

func TestApplyConstraintTags_default_and_example(t *testing.T) {
	t.Parallel()

	type S struct {
		Name string `json:"name" default:"hello" example:"world"`
	}

	schema := api.StructToSchema(reflect.TypeFor[S]())
	prop := schema.Properties["name"]
	assert.Equal(t, "hello", prop.Default)
	assert.Equal(t, "world", prop.Example)
}

func TestApplyConstraintTags_all_constraints(t *testing.T) {
	t.Parallel()

	type S struct {
		Name string   `json:"name" minLength:"2" maxLength:"50" pattern:"^[a-z]+$" enum:"a,b,c" default:"a" example:"b"`
		Age  int      `json:"age" minimum:"0" maximum:"200"`
		Tags []string `json:"tags" minItems:"1" maxItems:"10"`
	}

	schema := api.StructToSchema(reflect.TypeFor[S]())

	nameProp := schema.Properties["name"]
	assert.NotNil(t, nameProp.MinLength)
	assert.Equal(t, 2, *nameProp.MinLength)
	assert.NotNil(t, nameProp.MaxLength)
	assert.Equal(t, 50, *nameProp.MaxLength)
	assert.Equal(t, "^[a-z]+$", nameProp.Pattern)
	assert.Equal(t, []string{"a", "b", "c"}, nameProp.Enum)
	assert.Equal(t, "a", nameProp.Default)
	assert.Equal(t, "b", nameProp.Example)

	ageProp := schema.Properties["age"]
	assert.NotNil(t, ageProp.Minimum)
	assert.InDelta(t, 0.0, *ageProp.Minimum, 0.001)
	assert.NotNil(t, ageProp.Maximum)
	assert.InDelta(t, 200.0, *ageProp.Maximum, 0.001)

	tagsProp := schema.Properties["tags"]
	assert.NotNil(t, tagsProp.MinItems)
	assert.Equal(t, 1, *tagsProp.MinItems)
	assert.NotNil(t, tagsProp.MaxItems)
	assert.Equal(t, 10, *tagsProp.MaxItems)
}

func TestSchemaRegistry_typeToSchema_well_known_types(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		typ    reflect.Type
		expect api.JSONSchema
	}{
		"time.Time": {
			typ:    reflect.TypeFor[time.Time](),
			expect: api.JSONSchema{Type: "string", Format: "date-time"},
		},
		"time.Duration": {
			typ:    reflect.TypeFor[time.Duration](),
			expect: api.JSONSchema{Type: "string", Format: "duration"},
		},
		"Void": {
			typ:    reflect.TypeFor[api.Void](),
			expect: api.JSONSchema{},
		},
		"Stream": {
			typ:    reflect.TypeFor[api.Stream](),
			expect: api.JSONSchema{},
		},
		"SSEStream": {
			typ:    reflect.TypeFor[api.SSEStream](),
			expect: api.JSONSchema{},
		},
		"FileUpload": {
			typ:    reflect.TypeFor[api.FileUpload](),
			expect: api.JSONSchema{Type: "string", Format: "binary"},
		},
		"pointer unwrap": {
			typ:    reflect.TypeFor[*string](),
			expect: api.JSONSchema{Type: "string"},
		},
		"string": {
			typ:    reflect.TypeFor[string](),
			expect: api.JSONSchema{Type: "string"},
		},
		"bool": {
			typ:    reflect.TypeFor[bool](),
			expect: api.JSONSchema{Type: "boolean"},
		},
		"int": {
			typ:    reflect.TypeFor[int](),
			expect: api.JSONSchema{Type: "integer"},
		},
		"uint": {
			typ:    reflect.TypeFor[uint](),
			expect: api.JSONSchema{Type: "integer"},
		},
		"float64": {
			typ:    reflect.TypeFor[float64](),
			expect: api.JSONSchema{Type: "number"},
		},
		"float32": {
			typ:    reflect.TypeFor[float32](),
			expect: api.JSONSchema{Type: "number"},
		},
		"[]byte": {
			typ:    reflect.TypeFor[[]byte](),
			expect: api.JSONSchema{Type: "string", ContentEncoding: "base64"},
		},
		"[]string": {
			typ: reflect.TypeFor[[]string](),
			expect: api.JSONSchema{
				Type:  "array",
				Items: &api.JSONSchema{Type: "string"},
			},
		},
		"array type": {
			typ: reflect.TypeFor[[5]int](),
			expect: api.JSONSchema{
				Type:  "array",
				Items: &api.JSONSchema{Type: "integer"},
			},
		},
		"map non-string key": {
			typ:    reflect.TypeFor[map[int]string](),
			expect: api.JSONSchema{Type: "object"},
		},
		"map string key": {
			typ: reflect.TypeFor[map[string]int](),
			expect: api.JSONSchema{
				Type:                 "object",
				AdditionalProperties: &api.JSONSchema{Type: "integer"},
			},
		},
		"interface type": {
			typ:    reflect.TypeFor[any](),
			expect: api.JSONSchema{},
		},
		"unknown kind chan": {
			typ:    reflect.TypeFor[chan int](),
			expect: api.JSONSchema{},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			reg := api.NewSchemaRegistry()
			got := reg.TypeToSchema(tc.typ)
			assert.Equal(t, tc.expect, got)
		})
	}
}

func TestSchemaRegistry_pointer_to_time(t *testing.T) {
	t.Parallel()

	reg := api.NewSchemaRegistry()
	got := reg.TypeToSchema(reflect.TypeFor[*time.Time]())
	assert.Equal(t, api.JSONSchema{Type: "string", Format: "date-time"}, got)
}

// schemaProviderType implements SchemaProvider for testing.
type schemaProviderType struct {
	Value string
}

func (s *schemaProviderType) JSONSchema() api.JSONSchema {
	return api.JSONSchema{
		Type:   "string",
		Format: "custom-provider",
	}
}

func TestSchemaRegistry_SchemaProvider(t *testing.T) {
	t.Parallel()

	reg := api.NewSchemaRegistry()
	got := reg.TypeToSchema(reflect.TypeFor[schemaProviderType]())
	assert.Equal(t, "string", got.Type)
	assert.Equal(t, "custom-provider", got.Format)
}

// schemaTransformerType implements SchemaTransformer for testing.
type schemaTransformerType struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func (s *schemaTransformerType) TransformSchema(schema api.JSONSchema) api.JSONSchema {
	schema.Description = "transformed"
	return schema
}

func TestSchemaRegistry_SchemaTransformer(t *testing.T) {
	t.Parallel()

	reg := api.NewSchemaRegistry()
	schema := reg.TypeToSchema(reflect.TypeFor[schemaTransformerType]())

	assert.NotEmpty(t, schema.Ref)

	def, ok := reg.Defs["schemaTransformerType"]
	assert.True(t, ok)
	assert.Equal(t, "transformed", def.Description)
}

func TestSchemaRegistry_structToSchema_all_field_types(t *testing.T) {
	t.Parallel()

	type Nested struct {
		Visible    string `json:"visible" doc:"a field" required:"true"`
		Hidden     string `json:"-"`
		api.RawRequest
		ParamField string `path:"id"`
		EmptyName  string `json:",omitempty"`
	}

	reg := api.NewSchemaRegistry()
	schema := reg.TypeToSchema(reflect.TypeFor[Nested]())

	assert.NotEmpty(t, schema.Ref)

	def, ok := reg.Defs["Nested"]
	assert.True(t, ok)
	assert.Equal(t, "object", def.Type)

	_, hasVisible := def.Properties["visible"]
	assert.True(t, hasVisible)
	assert.Equal(t, "a field", def.Properties["visible"].Description)
	assert.Equal(t, []string{"visible"}, def.Required)

	_, hasHidden := def.Properties["-"]
	assert.False(t, hasHidden)

	_, hasEmptyName := def.Properties["EmptyName"]
	assert.True(t, hasEmptyName)
}

func TestSchemaRegistry_structToSchema_unexported(t *testing.T) {
	t.Parallel()

	// Use an anonymous struct so it doesn't register as $ref, allowing
	// direct inspection of the schema.
	reg := api.NewSchemaRegistry()
	typ := reflect.TypeOf(struct {
		Public string `json:"public"`
		_      string
	}{})

	schema := reg.TypeToSchema(typ)
	assert.Equal(t, "object", schema.Type)
	assert.Len(t, schema.Properties, 1)
	_, hasPub := schema.Properties["public"]
	assert.True(t, hasPub)
}

func TestErrorSchemaName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "ProblemDetail", api.ErrorSchemaName)
}
