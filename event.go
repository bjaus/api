package api

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"
)

// Event is a single server-sent event. Emit a stream of events by returning a
// response whose Body field is `<-chan Event`; the framework writes each
// event in the text/event-stream format and flushes after every send.
//
// Only populated fields are emitted. A zero-value Event produces a blank
// separator, which clients ignore.
type Event struct {
	// Name is the event type. Emitted as `event: <Name>`.
	Name string

	// Data is the event payload. Strings and []byte are written verbatim;
	// any other value is JSON-encoded.
	Data any

	// ID is the event identifier. Emitted as `id: <ID>`.
	ID string

	// Retry is the reconnection delay the client should use. Emitted as
	// `retry: <milliseconds>` when non-zero.
	Retry time.Duration
}

// writeEvent serializes a single Event in the text/event-stream wire format
// and terminates with a blank line. Fields set to their zero values are
// omitted. If Data is not a string or []byte, it is JSON-encoded.
func writeEvent(w io.Writer, e Event) error {
	if e.ID != "" {
		if _, err := fmt.Fprintf(w, "id: %s\n", e.ID); err != nil {
			return err
		}
	}
	if e.Name != "" {
		if _, err := fmt.Fprintf(w, "event: %s\n", e.Name); err != nil {
			return err
		}
	}
	if e.Retry > 0 {
		ms := strconv.FormatInt(e.Retry.Milliseconds(), 10)
		if _, err := fmt.Fprintf(w, "retry: %s\n", ms); err != nil {
			return err
		}
	}

	if err := writeEventData(w, e.Data); err != nil {
		return err
	}

	_, err := fmt.Fprint(w, "\n")
	return err
}

// writeEventData emits the data field, choosing a serialization appropriate
// to the payload type.
func writeEventData(w io.Writer, data any) error {
	switch v := data.(type) {
	case nil:
		return nil
	case string:
		_, err := fmt.Fprintf(w, "data: %s\n", v)
		return err
	case []byte:
		_, err := fmt.Fprintf(w, "data: %s\n", v)
		return err
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "data: %s\n", b)
		return err
	}
}
