package api

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"mime"
	"strconv"
	"strings"
)

// Encoder encodes response values to a wire format.
type Encoder interface {
	ContentType() string
	Encode(w io.Writer, v any) error
}

// Decoder decodes request bodies from a wire format.
type Decoder interface {
	ContentType() string
	Decode(r io.Reader, v any) error
}

// jsonCodec implements both Encoder and Decoder for JSON.
type jsonCodec struct{}

func (jsonCodec) ContentType() string { return "application/json" }

func (jsonCodec) Encode(w io.Writer, v any) error {
	return json.NewEncoder(w).Encode(v)
}

func (jsonCodec) Decode(r io.Reader, v any) error {
	err := json.NewDecoder(r).Decode(v)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

// xmlCodec implements both Encoder and Decoder for XML.
type xmlCodec struct{}

func (xmlCodec) ContentType() string { return "application/xml" }

func (xmlCodec) Encode(w io.Writer, v any) error {
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	return xml.NewEncoder(w).Encode(v)
}

func (xmlCodec) Decode(r io.Reader, v any) error {
	err := xml.NewDecoder(r).Decode(v)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

// codecRegistry holds all registered encoders and decoders.
// Index 0 is always JSON (the default).
type codecRegistry struct {
	encoders []Encoder
	decoders []Decoder
}

// newCodecRegistry builds a registry with JSON first, XML second, then any
// user-registered encoders and decoders.
func newCodecRegistry(userEncoders []Encoder, userDecoders []Decoder) *codecRegistry {
	cr := &codecRegistry{
		encoders: make([]Encoder, 0, 2+len(userEncoders)),
		decoders: make([]Decoder, 0, 2+len(userDecoders)),
	}
	cr.encoders = append(cr.encoders, jsonCodec{}, xmlCodec{})
	cr.encoders = append(cr.encoders, userEncoders...)
	cr.decoders = append(cr.decoders, jsonCodec{}, xmlCodec{})
	cr.decoders = append(cr.decoders, userDecoders...)
	return cr
}

// negotiate picks an encoder based on the Accept header value.
// Returns (JSON, true) for empty or */* accept values.
// Returns (nil, false) if an explicit Accept has no match.
func (cr *codecRegistry) negotiate(accept string) (Encoder, bool) {
	if accept == "" {
		return cr.encoders[0], true
	}

	type candidate struct {
		encoder Encoder
		quality float64
	}

	var best candidate
	best.quality = -1

	for part := range strings.SplitSeq(accept, ",") {
		mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(part))
		if err != nil {
			continue
		}

		q := 1.0
		if qs, ok := params["q"]; ok {
			if parsed, err := strconv.ParseFloat(qs, 64); err == nil {
				q = parsed
			}
		}

		if q <= best.quality {
			continue
		}

		if mediaType == "*/*" {
			best = candidate{encoder: cr.encoders[0], quality: q}
			continue
		}

		for _, enc := range cr.encoders {
			if enc.ContentType() == mediaType {
				best = candidate{encoder: enc, quality: q}
				break
			}
		}
	}

	if best.encoder == nil {
		return nil, false
	}
	return best.encoder, true
}

// decoderFor returns the decoder matching the given Content-Type.
// Returns (JSON decoder, true) for empty content type.
// Returns (nil, false) if the content type is present but unrecognized.
func (cr *codecRegistry) decoderFor(contentType string) (Decoder, bool) {
	if contentType == "" {
		return cr.decoders[0], true
	}

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, false
	}

	for _, dec := range cr.decoders {
		if dec.ContentType() == mediaType {
			return dec, true
		}
	}
	return nil, false
}

// contentTypes returns all encoder content types (for OpenAPI).
func (cr *codecRegistry) contentTypes() []string {
	cts := make([]string, len(cr.encoders))
	for i, enc := range cr.encoders {
		cts[i] = enc.ContentType()
	}
	return cts
}
