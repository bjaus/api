package api_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bjaus/api"
)

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
