package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Stream is a response type for binary or streaming responses.
// Return *Stream from a handler to bypass JSON encoding.
type Stream struct {
	ContentType string
	Status      int
	Body        io.Reader
}

// SSEStream is a response type for server-sent events.
// The handler writes events to the channel; the framework flushes them.
type SSEStream struct {
	Events <-chan SSEEvent
}

// SSEEvent is a single server-sent event.
type SSEEvent struct {
	// Event is the event type (optional). Maps to the "event:" field.
	Event string
	// Data is the event payload. If it's a struct/map, it will be JSON-encoded.
	Data any
	// ID is the event ID (optional). Maps to the "id:" field.
	ID string
}

// writeStream writes a Stream response.
func writeStream(w http.ResponseWriter, s *Stream) {
	if s.ContentType != "" {
		w.Header().Set("Content-Type", s.ContentType)
	}
	status := s.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	if s.Body != nil {
		//nolint:errcheck,gosec // best-effort streaming copy
		io.Copy(w, s.Body)
	}
}

// writeSSEStream writes an SSEStream response.
func writeSSEStream(w http.ResponseWriter, s *SSEStream) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	for event := range s.Events {
		writeSSEEvent(w, event)
		flusher.Flush()
	}
}

func writeSSEEvent(w io.Writer, event SSEEvent) {
	if event.ID != "" {
		writeSSEField(w, "id", event.ID)
	}
	if event.Event != "" {
		writeSSEField(w, "event", event.Event)
	}

	switch v := event.Data.(type) {
	case string:
		writeSSEField(w, "data", v)
	case []byte:
		writeSSEField(w, "data", string(v))
	default:
		data, err := json.Marshal(v)
		if err != nil {
			writeSSEField(w, "data", err.Error())
		} else {
			writeSSEField(w, "data", string(data))
		}
	}

	//nolint:errcheck // best-effort SSE write
	fmt.Fprint(w, "\n")
}

func writeSSEField(w io.Writer, name, value string) {
	//nolint:errcheck // best-effort SSE write
	fmt.Fprintf(w, "%s: %s\n", name, value)
}
