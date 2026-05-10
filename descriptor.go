package api

import (
	"fmt"
	"io"
	"reflect"
)

// responseDescriptor is a precomputed map of a response struct's tagged
// fields, built once at route registration. The encoder iterates the
// descriptor's slices per request instead of reparsing tags.
type responseDescriptor struct {
	status  *responseFieldDesc
	headers []responseHeaderDesc
	cookies []responseCookieDesc
	body    *responseBodyDesc
}

// responseFieldDesc locates a scalar field by its reflect.VisibleFields
// index path (handles embedded structs). typ and description are populated
// for spec generation; the runtime emitter consults only index and kind.
type responseFieldDesc struct {
	index       []int
	kind        reflect.Kind
	typ         reflect.Type
	description string
}

type responseHeaderDesc struct {
	responseFieldDesc
	name string
}

type responseCookieDesc struct {
	responseFieldDesc
	name string
}

// responseBodyDesc describes the Body field and how the encoder should emit
// the value found at that field's index path.
type responseBodyDesc struct {
	index []int
	kind  bodyKind
	// typ is the field's static type. Used for sanity checks and OpenAPI
	// schema generation; the encoder itself consults kind.
	typ reflect.Type
}

// bodyKind identifies how the framework emits the value stored in the
// response struct's Body field. When a response type has no Body field,
// desc.body is nil and no bodyKind is consulted.
type bodyKind int

const (
	bodyKindCodec  bodyKind = iota // encode via negotiated codec (JSON/XML/...)
	bodyKindReader                 // io.Copy raw bytes
	bodyKindChan                   // emit each channel value as an SSE event
)

var (
	readerInterfaceType = reflect.TypeFor[io.Reader]()
	eventType           = reflect.TypeFor[Event]()
)

// buildResponseDescriptor walks the response type once and produces a
// descriptor keyed by field index paths. Returns an error if the type is
// not a struct (after pointer unwrapping) or if two tagged fields collide
// on the same header/cookie name.
func buildResponseDescriptor(t reflect.Type) (*responseDescriptor, error) {
	t = derefType(t)
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("response type must be a struct, got %s", t.Kind())
	}

	desc := &responseDescriptor{}
	seenHeader := map[string]struct{}{}
	seenCookie := map[string]struct{}{}

	for _, f := range reflect.VisibleFields(t) {
		if !f.IsExported() {
			continue
		}
		// Skip anonymous struct fields themselves; their promoted leaf fields
		// appear separately in VisibleFields.
		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			continue
		}

		fd := responseFieldDesc{
			index:       f.Index,
			kind:        f.Type.Kind(),
			typ:         f.Type,
			description: f.Tag.Get("doc"),
		}

		if f.Name == "Body" {
			desc.body = &responseBodyDesc{
				index: f.Index,
				kind:  classifyBodyKind(f.Type),
				typ:   f.Type,
			}
			continue
		}

		if _, ok := f.Tag.Lookup("status"); ok {
			if desc.status != nil {
				return nil, fmt.Errorf("multiple status fields in response type %s", t)
			}
			desc.status = &fd
			continue
		}

		if name, ok := f.Tag.Lookup("header"); ok {
			if name == "" {
				return nil, fmt.Errorf("empty header tag on field %s in %s", f.Name, t)
			}
			if _, dup := seenHeader[name]; dup {
				return nil, fmt.Errorf("duplicate header %q in response type %s", name, t)
			}
			seenHeader[name] = struct{}{}
			desc.headers = append(desc.headers, responseHeaderDesc{
				responseFieldDesc: fd,
				name:              name,
			})
			continue
		}

		if name, ok := f.Tag.Lookup("cookie"); ok {
			if name == "" {
				return nil, fmt.Errorf("empty cookie tag on field %s in %s", f.Name, t)
			}
			if _, dup := seenCookie[name]; dup {
				return nil, fmt.Errorf("duplicate cookie %q in response type %s", name, t)
			}
			seenCookie[name] = struct{}{}
			desc.cookies = append(desc.cookies, responseCookieDesc{
				responseFieldDesc: fd,
				name:              name,
			})
			continue
		}
	}

	return desc, nil
}

// classifyBodyKind picks the emission path for a Body field based on its
// static type. The field's declared type wins: a field typed io.Reader
// streams even if the concrete value also satisfies some other interface.
func classifyBodyKind(t reflect.Type) bodyKind {
	if t.Kind() == reflect.Interface && t == readerInterfaceType {
		return bodyKindReader
	}
	if t.Kind() == reflect.Chan && t.Elem() == eventType {
		dir := t.ChanDir()
		if dir == reflect.RecvDir || dir == reflect.BothDir {
			return bodyKindChan
		}
	}
	return bodyKindCodec
}

// derefType unwraps *T to T. Non-pointer types are returned unchanged.
func derefType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}
