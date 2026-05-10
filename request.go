package api

import (
	"errors"
	"fmt"
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

// decodeRequest creates a new Req value and populates it from the HTTP request,
// using the precomputed request descriptor to avoid per-request reflection.
func decodeRequest[Req any](r *http.Request, codecs *codecRegistry, desc *requestDescriptor) (*Req, error) {
	req := new(Req)

	if desc.category == catVoid {
		return req, nil
	}

	v := reflect.ValueOf(req).Elem()

	if err := bindParams(v, r, desc); err != nil {
		return nil, err
	}

	switch desc.category {
	case catBodyOnly:
		if err := decodeBody(r, req, codecs); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrBindBody, err)
		}
	case catMixed:
		bodyPtr := v.FieldByIndex(desc.body.index).Addr().Interface()
		if err := decodeBody(r, bodyPtr, codecs); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrBindBody, err)
		}
	case catForm:
		if err := bindFormFields(v, r, desc); err != nil {
			return nil, err
		}
	}

	return req, nil
}

// bindParams binds path/query/header/cookie values and injects RawRequest
// using the descriptor's cached field index paths.
func bindParams(v reflect.Value, r *http.Request, desc *requestDescriptor) error {
	if desc.rawRequest != nil {
		v.FieldByIndex(desc.rawRequest.index).Set(reflect.ValueOf(RawRequest{Request: r}))
	}

	for _, p := range desc.params {
		var val string
		switch p.in {
		case paramInPath:
			val = r.PathValue(p.name)
		case paramInQuery:
			val = r.URL.Query().Get(p.name)
			if val == "" {
				val = p.defaultValue
			}
		case paramInHeader:
			val = r.Header.Get(p.name)
			if val == "" {
				val = p.defaultValue
			}
		case paramInCookie:
			if c, err := r.Cookie(p.name); err == nil {
				val = c.Value
			}
			if val == "" {
				val = p.defaultValue
			}
		}
		if val == "" {
			continue
		}
		if err := setFieldValue(v.FieldByIndex(p.index), val); err != nil {
			return fmt.Errorf("%w: %s: %w", bindErrFor(p.in), p.name, err)
		}
	}

	return nil
}

// bindErrFor returns the sentinel bind error for a parameter source.
func bindErrFor(in paramIn) error {
	switch in {
	case paramInPath:
		return ErrBindPath
	case paramInQuery:
		return ErrBindQuery
	case paramInHeader:
		return ErrBindHeader
	case paramInCookie:
		return ErrBindCookie
	}
	return ErrBindPath
}

// bindFormFields binds multipart form fields and files using the
// descriptor's cached form field map.
func bindFormFields(v reflect.Value, r *http.Request, desc *requestDescriptor) error {
	if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
		return fmt.Errorf("%w: %w", ErrBindForm, err)
	}

	for _, ff := range desc.forms {
		field := v.FieldByIndex(ff.index)

		switch ff.kind {
		case formSingleFile:
			file, header, err := r.FormFile(ff.name)
			if errors.Is(err, http.ErrMissingFile) {
				continue
			}
			if err != nil {
				return fmt.Errorf("%w: %s: %w", ErrBindForm, ff.name, err)
			}
			field.Set(reflect.ValueOf(FileUpload{
				Filename: header.Filename,
				Size:     header.Size,
				Header:   header,
				file:     file,
			}))

		case formMultiFile:
			if r.MultipartForm == nil || len(r.MultipartForm.File[ff.name]) == 0 {
				continue
			}
			headers := r.MultipartForm.File[ff.name]
			uploads := make([]FileUpload, 0, len(headers))
			for _, header := range headers {
				file, err := header.Open()
				if err != nil {
					return fmt.Errorf("%w: %s: %w", ErrBindForm, ff.name, err)
				}
				uploads = append(uploads, FileUpload{
					Filename: header.Filename,
					Size:     header.Size,
					Header:   header,
					file:     file,
				})
			}
			field.Set(reflect.ValueOf(uploads))

		case formScalar:
			val := r.FormValue(ff.name)
			if val == "" {
				continue
			}
			if err := setFieldValue(field, val); err != nil {
				return fmt.Errorf("%w: %s: %w", ErrBindForm, ff.name, err)
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

// decodeBody decodes the request body using the codec matched by Content-Type.
func decodeBody(r *http.Request, target any, codecs *codecRegistry) error {
	if r.Body == nil || r.ContentLength == 0 {
		return nil
	}

	dec, ok := codecs.decoderFor(r.Header.Get("Content-Type"))
	if !ok {
		return fmt.Errorf("unsupported Content-Type: %s", r.Header.Get("Content-Type"))
	}

	return dec.Decode(r.Body, target)
}
