package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bjaus/api"
)

type greetResp struct {
	XMLName xml.Name `json:"-" xml:"greeting"`
	Message string   `json:"message" xml:"message"`
}

type greetReq struct {
	Name string `json:"name" xml:"name"`
}

func newGreetRouter() *api.Router {
	r := api.New()
	api.Get(r, "/greet", func(_ context.Context, _ *api.Void) (*greetResp, error) {
		return &greetResp{Message: "hello"}, nil
	})
	api.Post(r, "/greet", func(_ context.Context, req *greetReq) (*greetResp, error) {
		return &greetResp{Message: "hello " + req.Name}, nil
	})
	return r
}

func TestNegotiate_json_response_default(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(newGreetRouter())
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/greet", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var body greetResp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "hello", body.Message)
}

func TestNegotiate_xml_response_via_accept(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(newGreetRouter())
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/greet", nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "application/xml")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/xml", resp.Header.Get("Content-Type"))

	var body greetResp
	require.NoError(t, xml.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "hello", body.Message)
}

func TestNegotiate_wildcard_accept_returns_json(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(newGreetRouter())
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/greet", nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "*/*")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
}

func TestNegotiate_quality_values_pick_json(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(newGreetRouter())
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/greet", nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "application/xml;q=0.9, application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
}

func TestNegotiate_unsupported_accept_returns_406(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(newGreetRouter())
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/greet", nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/csv")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusNotAcceptable, resp.StatusCode)
}

func TestNegotiate_json_request_body_default(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(newGreetRouter())
	t.Cleanup(srv.Close)

	body, err := json.Marshal(greetReq{Name: "world"})
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/greet", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var respBody greetResp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&respBody))
	assert.Equal(t, "hello world", respBody.Message)
}

func TestNegotiate_xml_request_body(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(newGreetRouter())
	t.Cleanup(srv.Close)

	body, err := xml.Marshal(greetReq{Name: "xml-world"})
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/greet", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/xml")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var respBody greetResp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&respBody))
	assert.Equal(t, "hello xml-world", respBody.Message)
}

func TestNegotiate_unknown_content_type_returns_400(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(newGreetRouter())
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/greet", bytes.NewReader([]byte("data")))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "text/csv")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestNegotiate_openapi_lists_all_content_types(t *testing.T) {
	t.Parallel()

	r := newGreetRouter()
	spec := r.Spec()

	getOp := spec.Paths["/greet"]["get"]
	respObj := getOp.Responses["200"]
	assert.Contains(t, respObj.Content, "application/json")
	assert.Contains(t, respObj.Content, "application/xml")

	postOp := spec.Paths["/greet"]["post"]
	require.NotNil(t, postOp.RequestBody)
	assert.Contains(t, postOp.RequestBody.Content, "application/json")
	assert.Contains(t, postOp.RequestBody.Content, "application/xml")
}

// testEncoder is a custom encoder for testing WithEncoder.
type testEncoder struct{}

func (testEncoder) ContentType() string             { return "text/plain" }
func (testEncoder) Encode(w io.Writer, v any) error { _, err := io.WriteString(w, "plain"); return err }

func TestNegotiate_custom_encoder_via_WithEncoder(t *testing.T) {
	t.Parallel()

	r := api.New(api.WithEncoder(testEncoder{}))
	api.Get(r, "/test", func(_ context.Context, _ *api.Void) (*greetResp, error) {
		return &greetResp{Message: "custom"}, nil
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/test", nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/plain")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/plain", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "plain", string(body))
}

func TestNegotiate_custom_encoder_in_openapi_spec(t *testing.T) {
	t.Parallel()

	r := api.New(api.WithEncoder(testEncoder{}))
	api.Get(r, "/test", func(_ context.Context, _ *api.Void) (*greetResp, error) {
		return &greetResp{Message: "custom"}, nil
	})

	spec := r.Spec()
	respObj := spec.Paths["/test"]["get"].Responses["200"]
	assert.Contains(t, respObj.Content, "application/json")
	assert.Contains(t, respObj.Content, "application/xml")
	assert.Contains(t, respObj.Content, "text/plain")
}

func TestNegotiate_xml_request_and_xml_response(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(newGreetRouter())
	t.Cleanup(srv.Close)

	body, err := xml.Marshal(greetReq{Name: "both"})
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/greet", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("Accept", "application/xml")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/xml", resp.Header.Get("Content-Type"))

	var respBody greetResp
	require.NoError(t, xml.NewDecoder(resp.Body).Decode(&respBody))
	assert.Equal(t, "hello both", respBody.Message)
}
