package fixtures

import (
	"io"
	"net/http"
	"strings"
	"sync"
)

// FailureMode selects how the fake backend answers. HTTP errors apply to every /api route; the two stream
// modes apply only to chat completions.
type FailureMode string

// Failure modes the fake backend supports.
const (
	// ModeHealthy answers every route normally.
	ModeHealthy FailureMode = "healthy"
	// ModeUnauthorized answers 401, the token is missing or expired.
	ModeUnauthorized FailureMode = "unauthorized"
	// ModeInsufficientCredits answers 402 with the credit balance the request would exceed.
	ModeInsufficientCredits FailureMode = "insufficient_credits"
	// ModeRateLimited answers 429 with a Retry-After header.
	ModeRateLimited FailureMode = "rate_limited"
	// ModeServerError answers 500.
	ModeServerError FailureMode = "server_error"
	// ModeTimeout accepts the request and never answers; the client must give up on its own deadline.
	ModeTimeout FailureMode = "timeout"
	// ModeMalformedSSE streams a truncated JSON frame and no [DONE] for chat completions.
	ModeMalformedSSE FailureMode = "malformed_sse"
	// ModeMidStreamError streams some text, then an error event, for chat completions.
	ModeMidStreamError FailureMode = "mid_stream_error"
)

// BackendRequest is one request the fake backend received.
type BackendRequest struct {
	Method string
	Path   string
	Body   string
}

// Backend is an http.Handler that imitates the Nevermind backend's /api/v1 routes and /api/me. Its failure
// mode can be switched between requests. Use it with fake.HTTP.Handle, or serve it from any listener.
//
//	backend := fixtures.NewBackend()
//	set.HTTP.Handle(backend)
//	backend.SetMode(fixtures.ModeInsufficientCredits)
type Backend struct {
	mu       sync.Mutex
	mode     FailureMode
	requests []BackendRequest
	hanging  chan struct{}
}

var _ http.Handler = (*Backend)(nil)

// NewBackend returns a healthy backend that has received no requests.
func NewBackend() *Backend {
	return &Backend{mode: ModeHealthy, hanging: make(chan struct{}, 16)}
}

// SetMode changes how the backend answers from the next request on.
func (b *Backend) SetMode(mode FailureMode) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.mode = mode
}

// Mode returns the current failure mode.
func (b *Backend) Mode() FailureMode {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.mode
}

// Requests returns the requests received so far, in order.
func (b *Backend) Requests() []BackendRequest {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]BackendRequest(nil), b.requests...)
}

// Hanging delivers one value each time a ModeTimeout request starts waiting, so a test can cancel the client
// only after the server has taken the request.
func (b *Backend) Hanging() <-chan struct{} {
	return b.hanging
}

// ServeHTTP answers one request according to the current failure mode.
func (b *Backend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body := readRequestBody(r)
	b.record(r, body)
	mode := b.Mode()
	if !isBackendRoute(r) {
		writeBody(w, http.StatusNotFound, "application/json", NotFoundBody)
		return
	}
	if b.applyHTTPFailure(w, r, mode) {
		return
	}
	switch {
	case r.Method == http.MethodPost && r.URL.Path == ChatCompletionsPath:
		serveChat(w, body, mode)
	case r.Method == http.MethodGet && r.URL.Path == ModelsPath:
		writeBody(w, http.StatusOK, "application/json", ModelsBody)
	case r.Method == http.MethodGet && r.URL.Path == MePath:
		writeBody(w, http.StatusOK, "application/json", MeBody)
	default:
		writeBody(w, http.StatusMethodNotAllowed, "application/json", MethodNotAllowedBody)
	}
}

// applyHTTPFailure writes the response for an HTTP-level failure mode and reports whether it did. Timeout
// blocks until the client goes away.
func (b *Backend) applyHTTPFailure(w http.ResponseWriter, r *http.Request, mode FailureMode) bool {
	switch mode {
	case ModeUnauthorized:
		writeBody(w, http.StatusUnauthorized, "text/plain; charset=utf-8", UnauthorizedBody)
	case ModeInsufficientCredits:
		writeBody(w, http.StatusPaymentRequired, "application/json", InsufficientCreditsBody)
	case ModeRateLimited:
		w.Header().Set("Retry-After", RetryAfterSeconds)
		writeBody(w, http.StatusTooManyRequests, "application/json", RateLimitedBody)
	case ModeServerError:
		writeBody(w, http.StatusInternalServerError, "application/json", ServerErrorBody)
	case ModeTimeout:
		b.hanging <- struct{}{}
		<-r.Context().Done()
	default:
		return false
	}
	return true
}

// record stores the request for later assertions.
func (b *Backend) record(r *http.Request, body string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.requests = append(b.requests, BackendRequest{Method: r.Method, Path: r.URL.Path, Body: body})
}

// isBackendRoute reports whether the request targets a route the fake implements at all.
func isBackendRoute(r *http.Request) bool {
	switch r.URL.Path {
	case ChatCompletionsPath, ModelsPath, MePath:
		return true
	default:
		return false
	}
}

// readRequestBody returns the body as a string; a body that cannot be read is treated as empty.
func readRequestBody(r *http.Request) string {
	if r.Body == nil {
		return ""
	}
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return ""
	}
	return string(data)
}

// writeBody sets the content type, writes the status and the exact body bytes.
func writeBody(w http.ResponseWriter, status int, contentType, body string) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

// serveChat answers a chat completion: a stream of the fixture reply, or one of the stream failure modes.
func serveChat(w http.ResponseWriter, body string, mode FailureMode) {
	if !strings.Contains(body, `"messages"`) {
		writeBody(w, http.StatusBadRequest, "application/json", InvalidRequestBody)
		return
	}
	frames := StreamFrames(FixtureReply, 24)
	switch mode {
	case ModeMalformedSSE:
		frames = MalformedStreamFrames(FixtureReply)
	case ModeMidStreamError:
		frames = MidStreamErrorFrames(FixtureReply)
	default:
		frames = append(frames, DoneFrame)
	}
	writeBody(w, http.StatusOK, "text/event-stream", strings.Join(frames, ""))
}
