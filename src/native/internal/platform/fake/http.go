package fake

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// ErrNoResponse is returned by HTTP.Do when neither a scripted response nor a handler is available, so a
// test that forgot to script a request fails loudly instead of hanging.
var ErrNoResponse = errors.New("fake http: no scripted response or handler for this request")

// Response is one scripted answer. When Err is set, Do returns it and no response; otherwise the status,
// header and body are returned as a real response would be.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
	Err    error
}

// RecordedRequest is a request the fake received, with its body read in full.
type RecordedRequest struct {
	Method string
	URL    string
	Header http.Header
	Body   []byte
}

// HTTP is a platform.HTTP that answers from a script of responses, or from an http.Handler when no response
// is queued. Every request is recorded. A cancelled request context fails the call before anything is
// queued or served, as a real client would.
//
//	h := fake.NewHTTP()
//	h.Script(fake.Response{Status: 200, Body: []byte(`{"ok":true}`)})
//	resp, err := ports.HTTP.Do(req)
type HTTP struct {
	mu       sync.Mutex
	script   []Response
	handler  http.Handler
	recorded []RecordedRequest
}

var _ platform.HTTP = (*HTTP)(nil)

// NewHTTP returns a fake with no script and no handler.
func NewHTTP() *HTTP {
	return &HTTP{}
}

// Script queues responses, used in order, one per request.
func (h *HTTP) Script(responses ...Response) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.script = append(h.script, responses...)
}

// Handle serves requests that have no queued response with handler, for example a fixture backend.
// Passing nil removes it.
func (h *HTTP) Handle(handler http.Handler) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.handler = handler
}

// Requests returns the recorded requests in arrival order.
func (h *HTTP) Requests() []RecordedRequest {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]RecordedRequest(nil), h.recorded...)
}

// Do records the request and answers it from the script, then from the handler.
func (h *HTTP) Do(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, errors.New("fake http: nil request")
	}
	body, err := readAndRestoreBody(req)
	if err != nil {
		return nil, err
	}
	h.record(req, body)
	if err := req.Context().Err(); err != nil {
		return nil, err
	}

	next, scripted, handler := h.nextAnswer()
	switch {
	case scripted:
		if next.Err != nil {
			return nil, next.Err
		}
		return responseFor(req, next), nil
	case handler != nil:
		recorder := newRecorder()
		handler.ServeHTTP(recorder, req)
		if err := req.Context().Err(); err != nil {
			return nil, err
		}
		return recorder.response(req), nil
	default:
		return nil, fmt.Errorf("%w: %s %s", ErrNoResponse, req.Method, req.URL)
	}
}

// nextAnswer pops the next scripted response, or reports the handler to use.
func (h *HTTP) nextAnswer() (Response, bool, http.Handler) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.script) > 0 {
		next := h.script[0]
		h.script = h.script[1:]
		return next, true, nil
	}
	return Response{}, false, h.handler
}

// record stores a copy of the request for later assertions.
func (h *HTTP) record(req *http.Request, body []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.recorded = append(h.recorded, RecordedRequest{
		Method: req.Method,
		URL:    req.URL.String(),
		Header: req.Header.Clone(),
		Body:   body,
	})
}

// readAndRestoreBody reads the request body and replaces it so the request can still be sent.
func readAndRestoreBody(req *http.Request) ([]byte, error) {
	if req.Body == nil || req.Body == http.NoBody {
		return nil, nil
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	_ = req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

// responseFor builds the *http.Response a scripted answer describes.
func responseFor(req *http.Request, scripted Response) *http.Response {
	status := scripted.Status
	if status == 0 {
		status = http.StatusOK
	}
	header := scripted.Header
	if header == nil {
		header = http.Header{}
	}
	return &http.Response{
		Status:        fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode:    status,
		Header:        header.Clone(),
		Body:          io.NopCloser(bytes.NewReader(scripted.Body)),
		ContentLength: int64(len(scripted.Body)),
		Request:       req,
	}
}
