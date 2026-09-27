// TestTransport: a fake Doer and a response builder, for testing YOUR OWN application without a
// real network call. Pass it via WithHTTPClient when constructing a Client in your own tests.
package duva

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
)

// RecordedRequest is one request TestTransport received, kept for assertions in your tests.
type RecordedRequest struct {
	Method string
	Path   string
	Query  url.Values
	Header http.Header
	Body   []byte
}

// TestTransport is a fake Doer: no network call. handler decides the response; every request is
// recorded in Requests, in order.
type TestTransport struct {
	Requests []RecordedRequest
	handler  func(*http.Request) (*http.Response, error)
}

// NewTestTransport builds a TestTransport that answers every request with handler.
func NewTestTransport(handler func(*http.Request) (*http.Response, error)) *TestTransport {
	return &TestTransport{handler: handler}
}

// Do implements Doer.
func (t *TestTransport) Do(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(body))
	}
	t.Requests = append(t.Requests, RecordedRequest{
		Method: req.Method,
		// EscapedPath, not Path: Path is always the DECODED form (net/url), which would hide a
		// wrong or missing percent-encoding (e.g. "@") that a real server would actually see.
		Path:   req.URL.EscapedPath(),
		Query:  req.URL.Query(),
		Header: req.Header.Clone(),
		Body:   body,
	})
	return t.handler(req)
}

// JSONResponse builds an *http.Response for a TestTransport handler: status, with body encoded
// as JSON.
func JSONResponse(status int, body any) *http.Response {
	data, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(data)),
	}
}
