package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjaus/api"
)

func TestForm_string_and_int_fields(t *testing.T) {
	t.Parallel()

	type Req struct {
		Title string `form:"title"`
		Count int    `form:"count"`
	}
	type Resp struct {
		Title string `json:"title"`
		Count int    `json:"count"`
	}

	r := api.New()
	api.Post(r, "/items", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{Title: req.Title, Count: req.Count}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	require.NoError(t, w.WriteField("title", "My Item"))
	require.NoError(t, w.WriteField("count", "42"))
	require.NoError(t, w.Close())

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/items", &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body Resp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "My Item", body.Title)
	assert.Equal(t, 42, body.Count)
}

func TestForm_bool_field(t *testing.T) {
	t.Parallel()

	type Req struct {
		Active bool `form:"active"`
	}
	type Resp struct {
		Active bool `json:"active"`
	}

	r := api.New()
	api.Post(r, "/toggle", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{Active: req.Active}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	require.NoError(t, w.WriteField("active", "true"))
	require.NoError(t, w.Close())

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/toggle", &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body Resp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.True(t, body.Active)
}

func TestForm_file_upload(t *testing.T) {
	t.Parallel()

	type Req struct {
		Title string         `form:"title"`
		File  api.FileUpload `form:"file"`
	}
	type Resp struct {
		Title    string `json:"title"`
		Filename string `json:"filename"`
		Size     int64  `json:"size"`
		Content  string `json:"content"`
	}

	r := api.New()
	api.Post(r, "/upload", func(_ context.Context, req *Req) (*Resp, error) {
		rc, err := req.File.Open()
		if err != nil {
			return nil, err
		}
		defer func() { _ = rc.Close() }() //nolint:errcheck
		data, err := io.ReadAll(rc)
		if err != nil {
			return nil, err
		}
		return &Resp{
			Title:    req.Title,
			Filename: req.File.Filename,
			Size:     req.File.Size,
			Content:  string(data),
		}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	require.NoError(t, w.WriteField("title", "My Upload"))
	fw, err := w.CreateFormFile("file", "hello.txt")
	require.NoError(t, err)
	_, err = fw.Write([]byte("hello world"))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/upload", &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body Resp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "My Upload", body.Title)
	assert.Equal(t, "hello.txt", body.Filename)
	assert.Equal(t, int64(11), body.Size)
	assert.Equal(t, "hello world", body.Content)
}

func TestForm_mixed_path_and_form(t *testing.T) {
	t.Parallel()

	type Req struct {
		ID    string `path:"id"`
		Title string `form:"title"`
	}
	type Resp struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}

	r := api.New()
	api.Post(r, "/items/{id}/upload", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{ID: req.ID, Title: req.Title}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	require.NoError(t, w.WriteField("title", "Updated Title"))
	require.NoError(t, w.Close())

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/items/abc123/upload", &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body Resp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "abc123", body.ID)
	assert.Equal(t, "Updated Title", body.Title)
}

func TestForm_missing_optional_file(t *testing.T) {
	t.Parallel()

	type Req struct {
		Title string         `form:"title"`
		File  api.FileUpload `form:"file"`
	}
	type Resp struct {
		Title    string `json:"title"`
		HasFile  bool   `json:"has_file"`
		Filename string `json:"filename"`
	}

	r := api.New()
	api.Post(r, "/upload", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{
			Title:    req.Title,
			HasFile:  req.File.Filename != "",
			Filename: req.File.Filename,
		}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	// Send form without the file field.
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	require.NoError(t, w.WriteField("title", "No File"))
	require.NoError(t, w.Close())

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/upload", &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body Resp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "No File", body.Title)
	assert.False(t, body.HasFile)
	assert.Empty(t, body.Filename)
}

func TestForm_invalid_scalar_type(t *testing.T) {
	t.Parallel()

	type Req struct {
		Count int `form:"count"`
	}

	r := api.New()
	api.Post(r, "/items", func(_ context.Context, _ *Req) (*api.Void, error) {
		return &api.Void{}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	require.NoError(t, w.WriteField("count", "not-a-number"))
	require.NoError(t, w.Close())

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/items", &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestForm_non_multipart_request(t *testing.T) {
	t.Parallel()

	type Req struct {
		Title string `form:"title"`
	}

	r := api.New()
	api.Post(r, "/items", func(_ context.Context, _ *Req) (*api.Void, error) {
		return &api.Void{}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	// Send JSON body instead of multipart.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/items",
		strings.NewReader(`{"title":"test"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestForm_constraint_validation(t *testing.T) {
	t.Parallel()

	type Req struct {
		Title string `form:"title" minLength:"3" maxLength:"20"`
	}

	r := api.New()
	api.Post(r, "/items", func(_ context.Context, _ *Req) (*api.Void, error) {
		return &api.Void{}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	tests := map[string]struct {
		title      string
		wantStatus int
	}{
		"valid": {
			title:      "Good Title",
			wantStatus: http.StatusNoContent,
		},
		"too short": {
			title:      "AB",
			wantStatus: http.StatusBadRequest,
		},
		"too long": {
			title:      "This Title Is Way Too Long For The Constraint",
			wantStatus: http.StatusBadRequest,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			w := multipart.NewWriter(&buf)
			require.NoError(t, w.WriteField("title", tc.title))
			require.NoError(t, w.Close())

			req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/items", &buf)
			require.NoError(t, err)
			req.Header.Set("Content-Type", w.FormDataContentType())

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			assert.Equal(t, tc.wantStatus, resp.StatusCode)
		})
	}
}

func TestHasFormTags(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input any
		want  bool
	}{
		"with form tag": {
			input: struct {
				Title string `form:"title"`
			}{},
			want: true,
		},
		"without form tag": {
			input: struct {
				Title string `json:"title"`
			}{},
			want: false,
		},
		"with mixed tags": {
			input: struct {
				ID    string `path:"id"`
				Title string `form:"title"`
			}{},
			want: true,
		},
		"unexported form field": {
			input: struct {
				title string `form:"title"`
			}{},
			want: false,
		},
		"pointer type": {
			input: (*struct {
				Title string `form:"title"`
			})(nil),
			want: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := api.HasFormTags(reflect.TypeOf(tc.input))
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestHasFormTags_non_struct(t *testing.T) {
	t.Parallel()

	assert.False(t, api.HasFormTags(reflect.TypeFor[string]()))
	assert.False(t, api.HasFormTags(reflect.TypeFor[int]()))
}

func TestForm_isParamField_excludes_form_fields(t *testing.T) {
	t.Parallel()

	// When a struct has form fields, they should NOT appear in the JSON body schema.
	type FormReq struct {
		Title string `form:"title"`
		Tags  string `form:"tags"`
	}

	schema := api.StructToSchema(reflect.TypeFor[FormReq]())
	assert.Empty(t, schema.Properties, "form-tagged fields should be excluded from JSON body schema")
}

func TestForm_openapi_multipart_content_type(t *testing.T) {
	t.Parallel()

	type Req struct {
		Title string         `form:"title" doc:"Item title" required:"true"`
		File  api.FileUpload `form:"file" doc:"Upload file"`
	}
	type Resp struct {
		OK bool `json:"ok"`
	}

	r := api.New()
	api.Post(r, "/upload", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{OK: true}, nil
	})

	spec := r.Spec()

	postOp, ok := spec.Paths["/upload"]["post"]
	require.True(t, ok)
	require.NotNil(t, postOp.RequestBody)
	require.True(t, postOp.RequestBody.Required)

	// Should have multipart/form-data content type.
	media, ok := postOp.RequestBody.Content["multipart/form-data"]
	require.True(t, ok, "expected multipart/form-data content type")
	require.NotNil(t, media.Schema)

	// Check schema properties.
	assert.Equal(t, "object", media.Schema.Type)
	require.Contains(t, media.Schema.Properties, "title")
	require.Contains(t, media.Schema.Properties, "file")

	titleProp := media.Schema.Properties["title"]
	assert.Equal(t, "string", titleProp.Type)
	assert.Equal(t, "Item title", titleProp.Description)

	fileProp := media.Schema.Properties["file"]
	assert.Equal(t, "string", fileProp.Type)
	assert.Equal(t, "binary", fileProp.Format)
	assert.Equal(t, "Upload file", fileProp.Description)

	// Required fields.
	assert.Contains(t, media.Schema.Required, "title")
}

func TestForm_openapi_no_json_content_type(t *testing.T) {
	t.Parallel()

	type Req struct {
		Title string `form:"title"`
	}

	r := api.New()
	api.Post(r, "/items", func(_ context.Context, _ *Req) (*api.Void, error) {
		return &api.Void{}, nil
	})

	spec := r.Spec()

	postOp := spec.Paths["/items"]["post"]
	require.NotNil(t, postOp.RequestBody)

	// Should NOT have application/json.
	_, hasJSON := postOp.RequestBody.Content["application/json"]
	assert.False(t, hasJSON, "form request should not have application/json content type")

	// Should have multipart/form-data.
	_, hasForm := postOp.RequestBody.Content["multipart/form-data"]
	assert.True(t, hasForm, "form request should have multipart/form-data content type")
}

func TestForm_openapi_with_constraints(t *testing.T) {
	t.Parallel()

	type Req struct {
		Title string `form:"title" minLength:"3" maxLength:"100"`
		Count int    `form:"count" minimum:"1" maximum:"999"`
	}

	r := api.New()
	api.Post(r, "/items", func(_ context.Context, _ *Req) (*api.Void, error) {
		return &api.Void{}, nil
	})

	spec := r.Spec()

	media := spec.Paths["/items"]["post"].RequestBody.Content["multipart/form-data"]
	require.NotNil(t, media.Schema)

	titleProp := media.Schema.Properties["title"]
	require.NotNil(t, titleProp.MinLength)
	assert.Equal(t, 3, *titleProp.MinLength)
	require.NotNil(t, titleProp.MaxLength)
	assert.Equal(t, 100, *titleProp.MaxLength)

	countProp := media.Schema.Properties["count"]
	require.NotNil(t, countProp.Minimum)
	assert.Equal(t, 1.0, *countProp.Minimum)
	require.NotNil(t, countProp.Maximum)
	assert.Equal(t, 999.0, *countProp.Maximum)
}

func TestForm_openapi_mixed_path_and_form(t *testing.T) {
	t.Parallel()

	type Req struct {
		ID    string         `path:"id"`
		Title string         `form:"title"`
		File  api.FileUpload `form:"file"`
	}
	type Resp struct {
		OK bool `json:"ok"`
	}

	r := api.New()
	api.Post(r, "/items/{id}/upload", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{OK: true}, nil
	})

	spec := r.Spec()

	postOp := spec.Paths["/items/{id}/upload"]["post"]

	// Should have path parameter.
	require.Len(t, postOp.Parameters, 1)
	assert.Equal(t, "id", postOp.Parameters[0].Name)
	assert.Equal(t, "path", postOp.Parameters[0].In)

	// Should have multipart/form-data body.
	require.NotNil(t, postOp.RequestBody)
	media, ok := postOp.RequestBody.Content["multipart/form-data"]
	require.True(t, ok)

	// Form body should only contain form fields, not path params.
	assert.NotContains(t, media.Schema.Properties, "id")
	assert.Contains(t, media.Schema.Properties, "title")
	assert.Contains(t, media.Schema.Properties, "file")
}

func TestForm_float64_field(t *testing.T) {
	t.Parallel()

	type Req struct {
		Price float64 `form:"price"`
	}
	type Resp struct {
		Price float64 `json:"price"`
	}

	r := api.New()
	api.Post(r, "/items", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{Price: req.Price}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	require.NoError(t, w.WriteField("price", "19.99"))
	require.NoError(t, w.Close())

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/items", &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body Resp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.InDelta(t, 19.99, body.Price, 0.001)
}

func TestForm_unexported_fields_skipped(t *testing.T) {
	t.Parallel()

	type Req struct {
		Title  string `form:"title"`
		hidden string `form:"hidden"`
	}
	type Resp struct {
		Title string `json:"title"`
	}

	r := api.New()
	api.Post(r, "/items", func(_ context.Context, req *Req) (*Resp, error) {
		_ = req.hidden // should be zero value
		return &Resp{Title: req.Title}, nil
	})

	// Runtime binding: unexported field should be skipped.
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	require.NoError(t, w.WriteField("title", "Test"))
	require.NoError(t, w.WriteField("hidden", "should-be-ignored"))
	require.NoError(t, w.Close())

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/items", &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// OpenAPI spec: unexported field should not appear.
	spec := r.Spec()
	media := spec.Paths["/items"]["post"].RequestBody.Content["multipart/form-data"]
	assert.Contains(t, media.Schema.Properties, "title")
	assert.NotContains(t, media.Schema.Properties, "hidden")
}

func TestForm_empty_form_value_skipped(t *testing.T) {
	t.Parallel()

	type Req struct {
		Title string `form:"title"`
		Count int    `form:"count"`
	}
	type Resp struct {
		Title string `json:"title"`
		Count int    `json:"count"`
	}

	r := api.New()
	api.Post(r, "/items", func(_ context.Context, req *Req) (*Resp, error) {
		return &Resp{Title: req.Title, Count: req.Count}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	// Send only title, count should be zero value.
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	require.NoError(t, w.WriteField("title", "Test"))
	require.NoError(t, w.Close())

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/items", &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body Resp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "Test", body.Title)
	assert.Equal(t, 0, body.Count)
}
