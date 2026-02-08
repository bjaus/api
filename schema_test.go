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
			expect: api.JSONSchema{Type: "string", Format: "byte"},
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
