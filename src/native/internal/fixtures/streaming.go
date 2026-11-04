package fixtures

import (
	"io"
	"net/http"
	"strings"
	"sync"
)

// StreamingTransport is a platform.HTTP for tests that stop or regenerate a reply while it is still arriving. It
// answers the active-model route with ActiveModelBody and every chat request with a body the test writes frame by
// frame: each chat request appears on Replies, and the test writes to the writer it receives. Chat request bodies
// are recorded in order.
type StreamingTransport struct {
	replies  chan *io.PipeWriter
	mu       sync.Mutex
	requests []string
}

// NewStreamingTransport returns a transport with no requests yet.
func NewStreamingTransport() *StreamingTransport {
	return &StreamingTransport{replies: make(chan *io.PipeWriter, 16)}
}

// Replies delivers the writer of each chat reply, in request order. A test writes the reply's frames to it.
func (s *StreamingTransport) Replies() <-chan *io.PipeWriter {
	return s.replies
}

// Body returns the body of the i-th chat request the transport received.
func (s *StreamingTransport) Body(i int) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return []byte(s.requests[i])
}

// Do answers one request: the descriptor for the active-model route, or a pending chat reply.
func (s *StreamingTransport) Do(req *http.Request) (*http.Response, error) {
	if strings.HasSuffix(req.URL.Path, ActiveModelPath) {
		return jsonResponse(req, ActiveModelBody), nil
	}
	body, _ := io.ReadAll(req.Body)
	s.mu.Lock()
	s.requests = append(s.requests, string(body))
	s.mu.Unlock()
	reader, writer := io.Pipe()
	s.replies <- writer
	return &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"text/event-stream"}},
		Body:       reader,
		Request:    req,
	}, nil
}

// jsonResponse answers req with a 200 JSON body.
func jsonResponse(req *http.Request, body string) *http.Response {
	return &http.Response{
		Status:        "200 OK",
		StatusCode:    http.StatusOK,
		Header:        http.Header{"Content-Type": {"application/json"}},
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}
}
