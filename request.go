package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"time"
)

// maxMultipartMemory is the maximum memory used for multipart form parsing (32 MB).
const maxMultipartMemory = 32 << 20

// requestCategory describes how a request type should be decoded.
type requestCategory int

const (
	catVoid     requestCategory = iota // Void — no params, no body
	catBodyOnly                        // entire struct is the body (no param tags, no Body field)
	catParams                          // has param tags but no Body field
	catMixed                           // has Body field (params from tagged fields, body from Body)
	catForm                            // has form tags (multipart/form-data binding)
)

// classifyRequest determines how a request type should be decoded.
func classifyRequest(t reflect.Type) requestCategory {
	if t == reflect.TypeFor[Void]() {
		return catVoid
	}
	if hasFormTags(t) {
		return catForm
	}
	if hasBodyField(t) {
		return catMixed
	}
	if hasParamTags(t) || hasRawRequest(t) {
		return catParams
	}
	return catBodyOnly
}

// decodeRequest creates a new Req value and populates it from the HTTP request.
func decodeRequest[Req any](r *http.Request) (*Req, error) {
	req := new(Req)
	t := reflect.TypeFor[Req]()
	cat := classifyRequest(t)

	if cat == catVoid {
		return req, nil
	}

	// Always bind params — handles path/query/header/cookie AND RawRequest injection.
	if err := bindParams(req, r); err != nil {
		return nil, err
	}

	// Decode body.
	switch cat {
	case catBodyOnly:
		if err := decodeBody(r, req); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrBindBody, err)
		}
	case catMixed:
		bodyField := reflect.ValueOf(req).Elem().FieldByName("Body")
		bodyPtr := bodyField.Addr().Interface()
		if err := decodeBody(r, bodyPtr); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrBindBody, err)
		}
	case catForm:
		if err := bindFormFields(req, r); err != nil {
			return nil, err
		}
	}

	return req, nil
}

// bindParams binds path, query, header, and cookie values to struct fields.
func bindParams(target any, r *http.Request) error {
	v := reflect.ValueOf(target)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	t := v.Type()
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}

		// Skip the Body field — it's decoded separately.
		if f.Name == "Body" {
			continue
		}

		field := v.Field(i)

		if name := f.Tag.Get("path"); name != "" {
			val := r.PathValue(name)
			if val != "" {
				if err := setFieldValue(field, val); err != nil {
					return fmt.Errorf("%w: %s: %w", ErrBindPath, name, err)
				}
			}
		}

		if name := f.Tag.Get("query"); name != "" {
			val := r.URL.Query().Get(name)
			if val == "" {
				val = f.Tag.Get("default")
			}
			if val != "" {
				if err := setFieldValue(field, val); err != nil {
					return fmt.Errorf("%w: %s: %w", ErrBindQuery, name, err)
				}
			}
		}

		if name := f.Tag.Get("header"); name != "" {
			val := r.Header.Get(name)
			if val == "" {
				val = f.Tag.Get("default")
			}
			if val != "" {
				if err := setFieldValue(field, val); err != nil {
					return fmt.Errorf("%w: %s: %w", ErrBindHeader, name, err)
				}
			}
		}

		if name := f.Tag.Get("cookie"); name != "" {
			var val string
			if c, err := r.Cookie(name); err == nil {
				val = c.Value
			}
			if val == "" {
				val = f.Tag.Get("default")
			}
			if val != "" {
				if err := setFieldValue(field, val); err != nil {
					return fmt.Errorf("%w: %s: %w", ErrBindCookie, name, err)
				}
			}
		}

		// Embed RawRequest: inject *http.Request.
		if f.Type == reflect.TypeFor[RawRequest]() {
			field.Set(reflect.ValueOf(RawRequest{Request: r}))
		}
	}

	return nil
}

// bindFormFields binds multipart form fields and files to struct fields tagged with "form".
func bindFormFields(target any, r *http.Request) error {
	if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
		return fmt.Errorf("%w: %w", ErrBindForm, err)
	}

	v := reflect.ValueOf(target)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	t := v.Type()
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}

		name := f.Tag.Get("form")
		if name == "" {
			continue
		}

		field := v.Field(i)

		// FileUpload fields: use r.FormFile.
		if f.Type == reflect.TypeFor[FileUpload]() {
			file, header, err := r.FormFile(name)
			if errors.Is(err, http.ErrMissingFile) {
				continue // optional file — leave zero value
			}
			if err != nil {
				return fmt.Errorf("%w: %s: %w", ErrBindForm, name, err)
			}
			field.Set(reflect.ValueOf(FileUpload{
				Filename: header.Filename,
				Size:     header.Size,
				Header:   header,
				file:     file,
			}))
			continue
		}

		// []FileUpload fields: iterate all files for this field name.
		if f.Type == reflect.TypeFor[[]FileUpload]() {
			if r.MultipartForm == nil || len(r.MultipartForm.File[name]) == 0 {
				continue // no files — leave nil slice
			}
			headers := r.MultipartForm.File[name]
			uploads := make([]FileUpload, 0, len(headers))
			for _, header := range headers {
				file, err := header.Open()
				if err != nil {
					return fmt.Errorf("%w: %s: %w", ErrBindForm, name, err)
				}
				uploads = append(uploads, FileUpload{
					Filename: header.Filename,
					Size:     header.Size,
					Header:   header,
					file:     file,
				})
			}
			field.Set(reflect.ValueOf(uploads))
			continue
		}

		// Scalar fields: use r.FormValue.
		val := r.FormValue(name)
		if val != "" {
			if err := setFieldValue(field, val); err != nil {
				return fmt.Errorf("%w: %s: %w", ErrBindForm, name, err)
			}
		}
	}

	return nil
}

// setFieldValue sets a reflect.Value from a string, supporting common types.
func setFieldValue(field reflect.Value, value string) error {
	if field.Type() == reflect.TypeFor[time.Duration]() {
		d, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		field.Set(reflect.ValueOf(d))
		return nil
	}

	//exhaustive:ignore
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int64:
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(n)
	case reflect.Float64:
		n, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		field.SetFloat(n)
	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(b)
	default:
		return fmt.Errorf("unsupported type: %s", field.Type())
	}
	return nil
}

// decodeBody decodes the request body as JSON into target.
func decodeBody(r *http.Request, target any) error {
	if r.Body == nil || r.ContentLength == 0 {
		return nil
	}
	err := json.NewDecoder(r.Body).Decode(target)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}
