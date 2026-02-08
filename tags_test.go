package api_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bjaus/api"
)

func TestHasParamTags(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		typ    reflect.Type
		expect bool
	}{
		"with path tag": {
			typ: reflect.TypeOf(struct {
				ID string `path:"id"`
			}{}),
			expect: true,
		},
		"with query tag": {
			typ: reflect.TypeOf(struct {
				Page int `query:"page"`
			}{}),
			expect: true,
		},
		"no param tags": {
			typ: reflect.TypeOf(struct {
				Name string `json:"name"`
			}{}),
			expect: false,
		},
		"pointer to struct": {
			typ: reflect.TypeOf(&struct {
				ID string `path:"id"`
			}{}),
			expect: true,
		},
		"non-struct": {
			typ:    reflect.TypeFor[string](),
			expect: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expect, api.HasParamTags(tc.typ))
		})
	}
}

func TestHasBodyField(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		typ    reflect.Type
		expect bool
	}{
		"with Body field": {
			typ: reflect.TypeOf(struct {
				Body struct{ Name string }
			}{}),
			expect: true,
		},
		"without Body field": {
			typ: reflect.TypeOf(struct {
				Name string
			}{}),
			expect: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expect, api.HasBodyField(tc.typ))
		})
	}
}

func TestTagOptions(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input      string
		expectName string
		expectOpts string
	}{
		"name only": {
			input:      "field",
			expectName: "field",
			expectOpts: "",
		},
		"name with options": {
			input:      "field,omitempty",
			expectName: "field",
			expectOpts: "omitempty",
		},
		"name with multiple options": {
			input:      "field,omitempty,string",
			expectName: "field",
			expectOpts: "omitempty,string",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			gotName, gotOpts := api.TagOptions(tc.input)
			assert.Equal(t, tc.expectName, gotName)
			assert.Equal(t, tc.expectOpts, gotOpts)
		})
	}
}

func TestTagContains(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		opts   string
		name   string
		expect bool
	}{
		"contains": {
			opts:   "omitempty,string",
			name:   "omitempty",
			expect: true,
		},
		"not contains": {
			opts:   "omitempty,string",
			name:   "required",
			expect: false,
		},
		"single option match": {
			opts:   "omitempty",
			name:   "omitempty",
			expect: true,
		},
		"empty opts": {
			opts:   "",
			name:   "omitempty",
			expect: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expect, api.TagContains(tc.opts, tc.name))
		})
	}
}
