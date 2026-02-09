package api

import "io"

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
