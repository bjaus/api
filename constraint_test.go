package api_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjaus/api"
)

func TestValidateConstraints_minLength(t *testing.T) {
	t.Parallel()

	type req struct {
		Name string `json:"name" minLength:"3"`
	}

	tests := map[string]struct {
		input   req
		wantErr bool
	}{
		"too short": {
			input:   req{Name: "ab"},
			wantErr: true,
		},
		"exact minimum": {
			input:   req{Name: "abc"},
			wantErr: false,
		},
		"longer than minimum": {
			input:   req{Name: "abcdef"},
			wantErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := api.ValidateConstraints(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				var pd *api.ProblemDetail
				require.True(t, errors.As(err, &pd))
				assert.Equal(t, "Validation Failed", pd.Title)
				assert.Len(t, pd.Errors, 1)
				assert.Equal(t, "name", pd.Errors[0].Field)
				assert.Contains(t, pd.Errors[0].Message, "at least 3 characters")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateConstraints_maxLength(t *testing.T) {
	t.Parallel()

	type req struct {
		Name string `json:"name" maxLength:"5"`
	}

	tests := map[string]struct {
		input   req
		wantErr bool
	}{
		"too long": {
			input:   req{Name: "abcdef"},
			wantErr: true,
		},
		"exact maximum": {
			input:   req{Name: "abcde"},
			wantErr: false,
		},
		"shorter than maximum": {
			input:   req{Name: "abc"},
			wantErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := api.ValidateConstraints(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				var pd *api.ProblemDetail
				require.True(t, errors.As(err, &pd))
				assert.Len(t, pd.Errors, 1)
				assert.Equal(t, "name", pd.Errors[0].Field)
				assert.Contains(t, pd.Errors[0].Message, "at most 5 characters")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateConstraints_minimum(t *testing.T) {
	t.Parallel()

	type req struct {
		Age int `json:"age" minimum:"18"`
	}

	tests := map[string]struct {
		input   req
		wantErr bool
	}{
		"below minimum": {
			input:   req{Age: 10},
			wantErr: true,
		},
		"at minimum": {
			input:   req{Age: 18},
			wantErr: false,
		},
		"above minimum": {
			input:   req{Age: 25},
			wantErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := api.ValidateConstraints(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				var pd *api.ProblemDetail
				require.True(t, errors.As(err, &pd))
				assert.Len(t, pd.Errors, 1)
				assert.Equal(t, "age", pd.Errors[0].Field)
				assert.Contains(t, pd.Errors[0].Message, "at least 18")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateConstraints_maximum(t *testing.T) {
	t.Parallel()

	type req struct {
		Score float64 `json:"score" maximum:"100"`
	}

	tests := map[string]struct {
		input   req
		wantErr bool
	}{
		"above maximum": {
			input:   req{Score: 150},
			wantErr: true,
		},
		"at maximum": {
			input:   req{Score: 100},
			wantErr: false,
		},
		"below maximum": {
			input:   req{Score: 50},
			wantErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := api.ValidateConstraints(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				var pd *api.ProblemDetail
				require.True(t, errors.As(err, &pd))
				assert.Len(t, pd.Errors, 1)
				assert.Equal(t, "score", pd.Errors[0].Field)
				assert.Contains(t, pd.Errors[0].Message, "at most 100")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateConstraints_pattern(t *testing.T) {
	t.Parallel()

	type req struct {
		Email string `json:"email" pattern:"^[a-z]+@[a-z]+\\.[a-z]+$"`
	}

	tests := map[string]struct {
		input   req
		wantErr bool
	}{
		"does not match pattern": {
			input:   req{Email: "not-an-email"},
			wantErr: true,
		},
		"matches pattern": {
			input:   req{Email: "user@example.com"},
			wantErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := api.ValidateConstraints(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				var pd *api.ProblemDetail
				require.True(t, errors.As(err, &pd))
				assert.Len(t, pd.Errors, 1)
				assert.Equal(t, "email", pd.Errors[0].Field)
				assert.Contains(t, pd.Errors[0].Message, "must match pattern")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateConstraints_enum(t *testing.T) {
	t.Parallel()

	type req struct {
		Status string `json:"status" enum:"active,inactive,pending"`
	}

	tests := map[string]struct {
		input   req
		wantErr bool
	}{
		"invalid enum value": {
			input:   req{Status: "deleted"},
			wantErr: true,
		},
		"valid enum active": {
			input:   req{Status: "active"},
			wantErr: false,
		},
		"valid enum pending": {
			input:   req{Status: "pending"},
			wantErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := api.ValidateConstraints(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				var pd *api.ProblemDetail
				require.True(t, errors.As(err, &pd))
				assert.Len(t, pd.Errors, 1)
				assert.Equal(t, "status", pd.Errors[0].Field)
				assert.Contains(t, pd.Errors[0].Message, "must be one of")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateConstraints_minItems(t *testing.T) {
	t.Parallel()

	type req struct {
		Tags []string `json:"tags" minItems:"2"`
	}

	tests := map[string]struct {
		input   req
		wantErr bool
	}{
		"too few items": {
			input:   req{Tags: []string{"one"}},
			wantErr: true,
		},
		"exact minimum items": {
			input:   req{Tags: []string{"one", "two"}},
			wantErr: false,
		},
		"more than minimum items": {
			input:   req{Tags: []string{"one", "two", "three"}},
			wantErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := api.ValidateConstraints(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				var pd *api.ProblemDetail
				require.True(t, errors.As(err, &pd))
				assert.Len(t, pd.Errors, 1)
				assert.Equal(t, "tags", pd.Errors[0].Field)
				assert.Contains(t, pd.Errors[0].Message, "at least 2 items")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateConstraints_maxItems(t *testing.T) {
	t.Parallel()

	type req struct {
		Tags []string `json:"tags" maxItems:"3"`
	}

	tests := map[string]struct {
		input   req
		wantErr bool
	}{
		"too many items": {
			input:   req{Tags: []string{"a", "b", "c", "d"}},
			wantErr: true,
		},
		"exact maximum items": {
			input:   req{Tags: []string{"a", "b", "c"}},
			wantErr: false,
		},
		"fewer than maximum items": {
			input:   req{Tags: []string{"a"}},
			wantErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := api.ValidateConstraints(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				var pd *api.ProblemDetail
				require.True(t, errors.As(err, &pd))
				assert.Len(t, pd.Errors, 1)
				assert.Equal(t, "tags", pd.Errors[0].Field)
				assert.Contains(t, pd.Errors[0].Message, "at most 3 items")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateConstraints_exhaustive_collects_all(t *testing.T) {
	t.Parallel()

	type req struct {
		Name  string   `json:"name" minLength:"3" maxLength:"10"`
		Age   int      `json:"age" minimum:"18" maximum:"120"`
		Tags  []string `json:"tags" minItems:"1"`
		Role  string   `json:"role" enum:"admin,user"`
	}

	input := req{
		Name: "a",          // violates minLength
		Age:  5,            // violates minimum
		Tags: []string{},   // violates minItems
		Role: "superadmin", // violates enum
	}

	err := api.ValidateConstraints(input)
	require.Error(t, err)

	var pd *api.ProblemDetail
	require.True(t, errors.As(err, &pd))
	assert.Len(t, pd.Errors, 4)
	assert.Contains(t, pd.Detail, "4 constraint violation(s)")

	fields := make(map[string]bool)
	for _, e := range pd.Errors {
		fields[e.Field] = true
	}
	assert.True(t, fields["name"])
	assert.True(t, fields["age"])
	assert.True(t, fields["tags"])
	assert.True(t, fields["role"])
}

func TestValidateConstraints_valid_data_passes(t *testing.T) {
	t.Parallel()

	type req struct {
		Name  string   `json:"name" minLength:"2" maxLength:"50"`
		Age   int      `json:"age" minimum:"0" maximum:"200"`
		Email string   `json:"email" pattern:"^.+@.+$"`
		Role  string   `json:"role" enum:"admin,user,guest"`
		Tags  []string `json:"tags" minItems:"1" maxItems:"10"`
	}

	input := req{
		Name:  "Alice",
		Age:   30,
		Email: "alice@example.com",
		Role:  "admin",
		Tags:  []string{"go", "api"},
	}

	err := api.ValidateConstraints(input)
	require.NoError(t, err)
}

func TestValidateConstraints_non_struct_returns_nil(t *testing.T) {
	t.Parallel()

	err := api.ValidateConstraints("not a struct")
	require.NoError(t, err)
}

func TestValidateConstraints_pointer_to_struct(t *testing.T) {
	t.Parallel()

	type req struct {
		Name string `json:"name" minLength:"5"`
	}

	input := &req{Name: "ab"}
	err := api.ValidateConstraints(input)
	require.Error(t, err)

	var pd *api.ProblemDetail
	require.True(t, errors.As(err, &pd))
	assert.Len(t, pd.Errors, 1)
	assert.Equal(t, "name", pd.Errors[0].Field)
}

func TestValidateConstraints_uint_minimum(t *testing.T) {
	t.Parallel()

	type req struct {
		Count uint `json:"count" minimum:"5"`
	}

	tests := map[string]struct {
		input   req
		wantErr bool
	}{
		"below minimum": {
			input:   req{Count: 2},
			wantErr: true,
		},
		"at minimum": {
			input:   req{Count: 5},
			wantErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := api.ValidateConstraints(tc.input)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateConstraints_float64_maximum(t *testing.T) {
	t.Parallel()

	type req struct {
		Price float64 `json:"price" maximum:"99.99"`
	}

	tests := map[string]struct {
		input   req
		wantErr bool
	}{
		"above maximum": {
			input:   req{Price: 100.00},
			wantErr: true,
		},
		"below maximum": {
			input:   req{Price: 50.00},
			wantErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := api.ValidateConstraints(tc.input)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateConstraints_skips_RawRequest(t *testing.T) {
	t.Parallel()

	type req struct {
		api.RawRequest
		Name string `json:"name" minLength:"3"`
	}

	input := req{Name: "abcde"}
	err := api.ValidateConstraints(input)
	require.NoError(t, err)
}

func TestValidateConstraints_nested_struct(t *testing.T) {
	t.Parallel()

	type Address struct {
		City string `json:"city" minLength:"2"`
	}
	type req struct {
		Address Address `json:"address"`
	}

	tests := map[string]struct {
		input   req
		wantErr bool
	}{
		"nested violation": {
			input:   req{Address: Address{City: "a"}},
			wantErr: true,
		},
		"nested valid": {
			input:   req{Address: Address{City: "NYC"}},
			wantErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := api.ValidateConstraints(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				var pd *api.ProblemDetail
				require.True(t, errors.As(err, &pd))
				assert.Equal(t, "address.city", pd.Errors[0].Field)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateConstraints_body_field_recursion(t *testing.T) {
	t.Parallel()

	type req struct {
		ID   string `path:"id"`
		Body struct {
			Name string `json:"name" minLength:"3"`
		}
	}

	input := req{ID: "abc", Body: struct {
		Name string `json:"name" minLength:"3"`
	}{Name: "ab"}}
	err := api.ValidateConstraints(input)
	require.Error(t, err)

	var pd *api.ProblemDetail
	require.True(t, errors.As(err, &pd))
	assert.Equal(t, "body.name", pd.Errors[0].Field)
}

func TestValidateConstraints_json_dash_skipped(t *testing.T) {
	t.Parallel()

	type req struct {
		Skipped string `json:"-" minLength:"100"`
		Name    string `json:"name"`
	}

	input := req{Skipped: "short", Name: "valid"}
	err := api.ValidateConstraints(input)
	require.NoError(t, err)
}

func TestValidateConstraints_unexported_field_skipped(t *testing.T) {
	t.Parallel()

	type withUnexported struct {
		hidden string `minLength:"100"` //nolint:unused
		Name   string `json:"name"`
	}
	input := withUnexported{Name: "valid"}

	err := api.ValidateConstraints(input)
	require.NoError(t, err)
}
