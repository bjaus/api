package api

import (
	"reflect"
	"strings"
)

// paramTags are the struct tags used for binding request parameters.
var paramTags = []string{"path", "query", "header", "cookie"}

// hasParamTags reports whether the given type has any fields with
// parameter binding tags (path, query, header, cookie).
func hasParamTags(t reflect.Type) bool {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return false
	}
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		for _, tag := range paramTags {
			if f.Tag.Get(tag) != "" {
				return true
			}
		}
	}
	return false
}

// hasRawRequest reports whether the given type embeds a RawRequest field.
func hasRawRequest(t reflect.Type) bool {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return false
	}
	for i := range t.NumField() {
		if t.Field(i).Type == reflect.TypeFor[RawRequest]() {
			return true
		}
	}
	return false
}

// hasBodyField reports whether the given type has an exported "Body" field.
func hasBodyField(t reflect.Type) bool {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return false
	}
	_, ok := t.FieldByName("Body")
	return ok
}

// hasFormTags reports whether the given type has any fields with
// a "form" binding tag.
func hasFormTags(t reflect.Type) bool {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return false
	}
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		if f.Tag.Get("form") != "" {
			return true
		}
	}
	return false
}

// tagOptions splits a struct tag value on comma and returns
// the name and remaining options.
func tagOptions(tag string) (string, string) {
	name, opts, _ := strings.Cut(tag, ",")
	return name, opts
}

// tagContains reports whether a comma-separated list of options
// contains a particular option.
func tagContains(opts string, name string) bool {
	for opts != "" {
		var opt string
		opt, opts, _ = strings.Cut(opts, ",")
		if opt == name {
			return true
		}
	}
	return false
}
