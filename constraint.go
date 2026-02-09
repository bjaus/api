package api

import (
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// validateConstraints checks all constraint tags on the struct fields and returns
// a ProblemDetail with all violations if any are found.
func validateConstraints(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil
	}

	var errs []ValidationError
	collectConstraintErrors(rv, "", &errs)

	if len(errs) > 0 {
		return &ProblemDetail{
			Type:   "about:blank",
			Title:  "Validation Failed",
			Status: http.StatusBadRequest,
			Detail: fmt.Sprintf("%d constraint violation(s)", len(errs)),
			Errors: errs,
		}
	}

	return nil
}

func collectConstraintErrors(rv reflect.Value, prefix string, errs *[]ValidationError) {
	t := rv.Type()

	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}

		fv := rv.Field(i)

		// Determine field path.
		name := jsonFieldName(f)
		if name == "-" {
			continue
		}

		path := name
		if prefix != "" {
			path = prefix + "." + name
		}

		// If this is the Body field, recurse into it.
		if f.Name == "Body" && f.Type.Kind() == reflect.Struct {
			collectConstraintErrors(fv, "body", errs)
			continue
		}

		// Skip RawRequest.
		if f.Type == reflect.TypeFor[RawRequest]() {
			continue
		}

		checkFieldConstraints(f, fv, path, errs)

		// Recurse into nested structs.
		if fv.Kind() == reflect.Struct && f.Type != reflect.TypeFor[RawRequest]() && !isParamField(f) {
			collectConstraintErrors(fv, path, errs)
		}
	}
}

func checkFieldConstraints(f reflect.StructField, fv reflect.Value, path string, errs *[]ValidationError) {
	// minLength / maxLength — strings.
	if fv.Kind() == reflect.String {
		val := fv.String()
		if tag := f.Tag.Get("minLength"); tag != "" {
			if n, err := strconv.Atoi(tag); err == nil && len(val) < n {
				*errs = append(*errs, ValidationError{
					Field:   path,
					Message: fmt.Sprintf("must be at least %d characters", n),
					Value:   val,
				})
			}
		}
		if tag := f.Tag.Get("maxLength"); tag != "" {
			if n, err := strconv.Atoi(tag); err == nil && len(val) > n {
				*errs = append(*errs, ValidationError{
					Field:   path,
					Message: fmt.Sprintf("must be at most %d characters", n),
					Value:   val,
				})
			}
		}
		if tag := f.Tag.Get("pattern"); tag != "" {
			if matched, err := regexp.MatchString(tag, val); err == nil && !matched {
				*errs = append(*errs, ValidationError{
					Field:   path,
					Message: fmt.Sprintf("must match pattern %s", tag),
					Value:   val,
				})
			}
		}
	}

	// minimum / maximum — numeric types.
	if isNumericKind(fv.Kind()) {
		floatVal := toFloat64(fv)
		if tag := f.Tag.Get("minimum"); tag != "" {
			if lower, err := strconv.ParseFloat(tag, 64); err == nil && floatVal < lower {
				*errs = append(*errs, ValidationError{
					Field:   path,
					Message: fmt.Sprintf("must be at least %s", tag),
					Value:   floatVal,
				})
			}
		}
		if tag := f.Tag.Get("maximum"); tag != "" {
			if upper, err := strconv.ParseFloat(tag, 64); err == nil && floatVal > upper {
				*errs = append(*errs, ValidationError{
					Field:   path,
					Message: fmt.Sprintf("must be at most %s", tag),
					Value:   floatVal,
				})
			}
		}
	}

	// enum — strings.
	if fv.Kind() == reflect.String {
		if tag := f.Tag.Get("enum"); tag != "" {
			val := fv.String()
			allowed := strings.Split(tag, ",")
			found := false
			for _, a := range allowed {
				if a == val {
					found = true
					break
				}
			}
			if !found {
				*errs = append(*errs, ValidationError{
					Field:   path,
					Message: fmt.Sprintf("must be one of [%s]", tag),
					Value:   val,
				})
			}
		}
	}

	// minItems / maxItems — slices.
	if fv.Kind() == reflect.Slice {
		length := fv.Len()
		if tag := f.Tag.Get("minItems"); tag != "" {
			if n, err := strconv.Atoi(tag); err == nil && length < n {
				*errs = append(*errs, ValidationError{
					Field:   path,
					Message: fmt.Sprintf("must have at least %d items", n),
					Value:   length,
				})
			}
		}
		if tag := f.Tag.Get("maxItems"); tag != "" {
			if n, err := strconv.Atoi(tag); err == nil && length > n {
				*errs = append(*errs, ValidationError{
					Field:   path,
					Message: fmt.Sprintf("must have at most %d items", n),
					Value:   length,
				})
			}
		}
	}
}

func isNumericKind(k reflect.Kind) bool {
	//exhaustive:ignore
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

func toFloat64(v reflect.Value) float64 {
	//exhaustive:ignore
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(v.Uint())
	default: // float32, float64
		return v.Float()
	}
}
