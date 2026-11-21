package fake

import (
	"bytes"
	"io"
	"net/http"
)

// recorder is an http.ResponseWriter that buffers a handler's output into an *http.Response. It exists so
// fake HTTP can serve handlers without a listening socket.
type recorder struct {
	header      http.Header
	status      int
	wroteHeader bool
	body        bytes.Buffer
}

// newRecorder returns an empty recorder.
func newRecorder() *recorder {
	return &recorder{header: http.Header{}}
}

// Header returns the headers the handler sets before writing.
func (r *recorder) Header() http.Header { return r.header }

// WriteHeader records the status on the first call.
func (r *recorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.status = status
}

// Write buffers body bytes, implying a 200 status when none was written.
func (r *recorder) Write(p []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.body.Write(p)
}

// Flush is a no-op: the whole body is returned at once.
func (r *recorder) Flush() {}

// response converts the buffered output into a response for req. A handler that wrote nothing yields 200
// with an empty body, as net/http does.
func (r *recorder) response(req *http.Request) *http.Response {
	status := r.status
	if !r.wroteHeader {
		status = http.StatusOK
	}
	return &http.Response{
		Status:        http.StatusText(status),
		StatusCode:    status,
		Header:        r.header.Clone(),
		Body:          io.NopCloser(bytes.NewReader(r.body.Bytes())),
		ContentLength: int64(r.body.Len()),
		Request:       req,
	}
}
