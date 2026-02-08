package api

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

// FileUpload holds a parsed file from a multipart form upload.
type FileUpload struct {
	Filename string
	Size     int64
	Header   *multipart.FileHeader
	file     multipart.File
}

// Open returns a reader for the uploaded file contents.
func (f *FileUpload) Open() (io.ReadCloser, error) {
	if f.file != nil {
		return f.file, nil
	}
	if f.Header == nil {
		return nil, fmt.Errorf("no file header")
	}
	file, err := f.Header.Open()
	if err != nil {
		return nil, err
	}
	f.file = file
	return file, nil
}

// ParseFileUpload extracts a file upload from a multipart form.
func ParseFileUpload(r *http.Request, fieldName string) (*FileUpload, error) {
	file, header, err := r.FormFile(fieldName)
	if err != nil {
		return nil, fmt.Errorf("form file %q: %w", fieldName, err)
	}
	return &FileUpload{
		Filename: header.Filename,
		Size:     header.Size,
		Header:   header,
		file:     file,
	}, nil
}
