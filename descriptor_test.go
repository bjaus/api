package api_test

import (
	"io"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjaus/api"
)

type simpleJSONResp struct {
	Body struct {
		Name string `json:"name"`
	}
}

type withMetadataResp struct {
	Status int    `status:""`
	ETag   string `header:"ETag"`
	Body   struct {
		ID string `json:"id"`
	}
}

type embedHeaders struct {
	ETag         string `header:"ETag"`
	CacheControl string `header:"Cache-Control"`
}

type embeddedResp struct {
	embedHeaders
	Body struct {
		Name string `json:"name"`
	}
}

// ExportedCacheHeaders is an exported embedded type so we cover the
// reflect.VisibleFields anonymous-wrapper skip branch.
type ExportedCacheHeaders struct {
	ETag string `header:"ETag"`
}

type exportedEmbedResp struct {
	ExportedCacheHeaders
	Body struct {
		ID string `json:"id"`
	}
}

type cookieDescResp struct {
	Session api.Cookie `cookie:"session"`
	Body    struct{}
}

type streamResp struct {
	Type string `header:"Content-Type"`
	Body io.Reader
}

type eventsResp struct {
	Body <-chan api.Event
}

type emptyResp struct{}

type unexportedFields struct {
	secret string `header:"X-Secret"` //nolint:unused
	Body   struct{}
}

type noBodyJustHeaders struct {
	ETag string `header:"ETag"`
}

func TestBuildResponseDescriptor_simple(t *testing.T) {
	t.Parallel()

	d, err := api.BuildResponseDescriptor(reflect.TypeFor[simpleJSONResp]())
	require.NoError(t, err)
	assert.False(t, d.HasStatus())
	assert.Empty(t, d.HeaderNames())
	assert.Empty(t, d.CookieNames())
	assert.Equal(t, api.BodyKindCodec, d.BodyKind())
}

func TestBuildResponseDescriptor_withMetadata(t *testing.T) {
	t.Parallel()

	d, err := api.BuildResponseDescriptor(reflect.TypeFor[withMetadataResp]())
	require.NoError(t, err)
	assert.True(t, d.HasStatus())
	assert.Equal(t, []string{"ETag"}, d.HeaderNames())
	assert.Equal(t, api.BodyKindCodec, d.BodyKind())
}

func TestBuildResponseDescriptor_embeddedHeaders(t *testing.T) {
	t.Parallel()

	d, err := api.BuildResponseDescriptor(reflect.TypeFor[embeddedResp]())
	require.NoError(t, err)
	assert.Equal(t, []string{"ETag", "Cache-Control"}, d.HeaderNames())
	assert.Equal(t, api.BodyKindCodec, d.BodyKind())
}

func TestBuildResponseDescriptor_exportedEmbedded(t *testing.T) {
	t.Parallel()

	d, err := api.BuildResponseDescriptor(reflect.TypeFor[exportedEmbedResp]())
	require.NoError(t, err)
	assert.Equal(t, []string{"ETag"}, d.HeaderNames())
	assert.Equal(t, api.BodyKindCodec, d.BodyKind())
}

func TestBuildResponseDescriptor_cookie(t *testing.T) {
	t.Parallel()

	d, err := api.BuildResponseDescriptor(reflect.TypeFor[cookieDescResp]())
	require.NoError(t, err)
	assert.Equal(t, []string{"session"}, d.CookieNames())
}

func TestBuildResponseDescriptor_streamBody(t *testing.T) {
	t.Parallel()

	d, err := api.BuildResponseDescriptor(reflect.TypeFor[streamResp]())
	require.NoError(t, err)
	assert.Equal(t, []string{"Content-Type"}, d.HeaderNames())
	assert.Equal(t, api.BodyKindReader, d.BodyKind())
}

func TestBuildResponseDescriptor_eventsBody(t *testing.T) {
	t.Parallel()

	d, err := api.BuildResponseDescriptor(reflect.TypeFor[eventsResp]())
	require.NoError(t, err)
	assert.Equal(t, api.BodyKindChan, d.BodyKind())
}

func TestBuildResponseDescriptor_noBodyField(t *testing.T) {
	t.Parallel()

	d, err := api.BuildResponseDescriptor(reflect.TypeFor[noBodyJustHeaders]())
	require.NoError(t, err)
	assert.Equal(t, []string{"ETag"}, d.HeaderNames())
	assert.Equal(t, api.BodyKindNone, d.BodyKind())
}

func TestBuildResponseDescriptor_emptyStruct(t *testing.T) {
	t.Parallel()

	d, err := api.BuildResponseDescriptor(reflect.TypeFor[emptyResp]())
	require.NoError(t, err)
	assert.Empty(t, d.HeaderNames())
	assert.Empty(t, d.CookieNames())
	assert.Equal(t, api.BodyKindNone, d.BodyKind())
}

func TestBuildResponseDescriptor_unwrapsPointer(t *testing.T) {
	t.Parallel()

	d, err := api.BuildResponseDescriptor(reflect.TypeFor[*withMetadataResp]())
	require.NoError(t, err)
	assert.True(t, d.HasStatus())
}

func TestBuildResponseDescriptor_skipsUnexportedFields(t *testing.T) {
	t.Parallel()

	d, err := api.BuildResponseDescriptor(reflect.TypeFor[unexportedFields]())
	require.NoError(t, err)
	assert.Empty(t, d.HeaderNames(), "unexported fields must be skipped even if tagged")
}

// --- Error cases ---

func TestBuildResponseDescriptor_errors(t *testing.T) {
	t.Parallel()

	t.Run("non-struct type", func(t *testing.T) {
		t.Parallel()
		_, err := api.BuildResponseDescriptor(reflect.TypeFor[int]())
		require.Error(t, err)
	})

	t.Run("multiple status fields", func(t *testing.T) {
		t.Parallel()
		type bad struct {
			A int `status:""`
			B int `status:""`
		}
		_, err := api.BuildResponseDescriptor(reflect.TypeFor[bad]())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "multiple status")
	})

	t.Run("empty header tag", func(t *testing.T) {
		t.Parallel()
		type bad struct {
			X string `header:""`
		}
		_, err := api.BuildResponseDescriptor(reflect.TypeFor[bad]())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty header tag")
	})

	t.Run("duplicate header names", func(t *testing.T) {
		t.Parallel()
		type bad struct {
			A string `header:"X-Foo"`
			B string `header:"X-Foo"`
		}
		_, err := api.BuildResponseDescriptor(reflect.TypeFor[bad]())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate header")
	})

	t.Run("empty cookie tag", func(t *testing.T) {
		t.Parallel()
		type bad struct {
			X api.Cookie `cookie:""`
		}
		_, err := api.BuildResponseDescriptor(reflect.TypeFor[bad]())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty cookie tag")
	})

	t.Run("duplicate cookie names", func(t *testing.T) {
		t.Parallel()
		type bad struct {
			A api.Cookie `cookie:"session"`
			B api.Cookie `cookie:"session"`
		}
		_, err := api.BuildResponseDescriptor(reflect.TypeFor[bad]())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate cookie")
	})
}

// --- Body kind classification ---

func TestClassifyBodyKind(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		typ  reflect.Type
		want api.BodyKind
	}{
		"struct body":                   {reflect.TypeFor[struct{ X int }](), api.BodyKindCodec},
		"pointer struct body":           {reflect.TypeFor[*struct{ X int }](), api.BodyKindCodec},
		"slice body":                    {reflect.TypeFor[[]int](), api.BodyKindCodec},
		"map body":                      {reflect.TypeFor[map[string]int](), api.BodyKindCodec},
		"string body":                   {reflect.TypeFor[string](), api.BodyKindCodec},
		"io.Reader body":                {reflect.TypeFor[io.Reader](), api.BodyKindReader},
		"receive chan of Event":         {reflect.TypeFor[<-chan api.Event](), api.BodyKindChan},
		"bidirectional chan of Event":   {reflect.TypeFor[chan api.Event](), api.BodyKindChan},
		"send-only chan of Event":       {reflect.TypeFor[chan<- api.Event](), api.BodyKindCodec}, // not recv-capable
		"chan of other type":            {reflect.TypeFor[chan int](), api.BodyKindCodec},
		"concrete type implementing io": {reflect.TypeFor[*placeholderReader](), api.BodyKindCodec}, // only interface type counts
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			d, err := api.BuildResponseDescriptor(reflect.StructOf([]reflect.StructField{
				{Name: "Body", Type: tc.typ},
			}))
			require.NoError(t, err)
			assert.Equal(t, tc.want, d.BodyKind())
		})
	}
}

type placeholderReader struct{}

func (placeholderReader) Read(p []byte) (int, error) { return 0, io.EOF }
