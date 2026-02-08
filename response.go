package api

import (
	"encoding/json"
	"net/http"
)

// encodeResponse writes the response to the http.ResponseWriter.
// It handles Void (204), Stream, SSEStream, StatusCoder, and JSON.
func encodeResponse(w http.ResponseWriter, resp any, defaultStatus int) {
	if resp == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Stream response — caller controls content type and body.
	if s, ok := resp.(*Stream); ok {
		writeStream(w, s)
		return
	}

	// SSE stream — long-lived event stream.
	if s, ok := resp.(*SSEStream); ok {
		writeSSEStream(w, s)
		return
	}

	status := defaultStatus
	if status == 0 {
		status = http.StatusOK
	}

	// Let the response override the status dynamically.
	if sc, ok := resp.(StatusCoder); ok {
		status = sc.StatusCode()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	//nolint:errcheck,errchkjson,gosec // best-effort after WriteHeader
	json.NewEncoder(w).Encode(resp)
}

// writeErrorResponse writes an error as a JSON response.
func writeErrorResponse(w http.ResponseWriter, err error) {
	status := ErrorStatus(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	//nolint:errcheck,errchkjson,gosec // best-effort after WriteHeader
	json.NewEncoder(w).Encode(map[string]string{
		"error": err.Error(),
	})
}
