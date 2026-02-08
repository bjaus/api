package api_test

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjaus/api"
)

func TestParseFileUpload(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("avatar", "photo.png")
	require.NoError(t, err)
	_, err = fw.Write([]byte("fake png data"))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/upload", &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", w.FormDataContentType())

	upload, err := api.ParseFileUpload(req, "avatar")
	require.NoError(t, err)

	assert.Equal(t, "photo.png", upload.Filename)
	assert.Greater(t, upload.Size, int64(0))

	rc, err := upload.Open()
	require.NoError(t, err)
	defer func() { require.NoError(t, rc.Close()) }()

	data := make([]byte, 100)
	n, err := rc.Read(data)
	require.NoError(t, err)
	assert.Equal(t, "fake png data", string(data[:n]))
}

func TestParseFileUpload_missing_field(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	require.NoError(t, w.Close())

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/upload", &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", w.FormDataContentType())

	_, err = api.ParseFileUpload(req, "avatar")
	assert.Error(t, err)
}
