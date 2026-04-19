package api_test

import (
	"bytes"
	"errors"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjaus/api"
)

func TestWriteEvent(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		event api.Event
		want  string
	}{
		"zero value emits only terminator": {
			event: api.Event{},
			want:  "\n",
		},
		"name only": {
			event: api.Event{Name: "tick"},
			want:  "event: tick\n\n",
		},
		"id only": {
			event: api.Event{ID: "42"},
			want:  "id: 42\n\n",
		},
		"string data": {
			event: api.Event{Data: "hello"},
			want:  "data: hello\n\n",
		},
		"bytes data": {
			event: api.Event{Data: []byte("hello")},
			want:  "data: hello\n\n",
		},
		"struct data as json": {
			event: api.Event{Data: map[string]int{"count": 5}},
			want:  "data: {\"count\":5}\n\n",
		},
		"retry emitted in ms": {
			event: api.Event{Retry: 2500 * time.Millisecond},
			want:  "retry: 2500\n\n",
		},
		"all fields populated": {
			event: api.Event{
				ID:    "9",
				Name:  "update",
				Retry: time.Second,
				Data:  "payload",
			},
			want: "id: 9\nevent: update\nretry: 1000\ndata: payload\n\n",
		},
		"nil data suppresses data line": {
			event: api.Event{Name: "ping", Data: nil},
			want:  "event: ping\n\n",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			require.NoError(t, api.WriteEvent(&buf, tc.event))
			assert.Equal(t, tc.want, buf.String())
		})
	}
}

func TestWriteEvent_jsonMarshalError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := api.WriteEvent(&buf, api.Event{Data: math.NaN()})
	require.Error(t, err)
}

var errWriteFailed = errors.New("write failed")

type failWriter struct{ failAt int }

func (w *failWriter) Write(p []byte) (int, error) {
	w.failAt--
	if w.failAt <= 0 {
		return 0, errWriteFailed
	}
	return len(p), nil
}

func TestWriteEvent_writerErrors(t *testing.T) {
	t.Parallel()

	ev := api.Event{ID: "1", Name: "x", Retry: time.Second, Data: "d"}

	for failAt := 1; failAt <= 5; failAt++ {
		failAt := failAt
		t.Run("fail_at_"+strconv.Itoa(failAt), func(t *testing.T) {
			t.Parallel()
			fw := &failWriter{failAt: failAt}
			err := api.WriteEvent(fw, ev)
			require.ErrorIs(t, err, errWriteFailed)
		})
	}
}
