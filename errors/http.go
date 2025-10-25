package errors

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// HTTPError mirrors the JavaScript HTTPException pattern by storing response.
type HTTPError struct {
	Message  string
	Response *http.Response
	Body     []byte
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("%s (status %d)", e.Message, e.Response.StatusCode)
}

// NewHTTPError reads the response body once and stores it.
func NewHTTPError(message string, resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return &HTTPError{
		Message:  message,
		Response: resp,
		Body:     body,
	}
}

// JSON returns parsed body if it is JSON, otherwise an error.
func (e *HTTPError) JSON(v any) error {
	return json.Unmarshal(e.Body, v)
}

// NewJSONResponse helps create synthetic responses for errors.
func NewJSONResponse(status int, body any) *http.Response {
	payload, _ := json.Marshal(body)
	return &http.Response{
		StatusCode:    status,
		Status:        fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Body:          io.NopCloser(bytes.NewReader(payload)),
		ContentLength: int64(len(payload)),
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}
}
