package fixtures

import (
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
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

// Backend is an http.Handler that imitates the Nevermind backend's /api/v1 routes, /api/me and the device sign-in.
// Its failure mode can be switched between requests. Use it with fake.HTTP.Handle, or serve it from any listener.
//
//	backend := fixtures.NewBackend()
//	set.HTTP.Handle(backend)
//	backend.SetMode(fixtures.ModeInsufficientCredits)
type Backend struct {
	mu          sync.Mutex
	mode        FailureMode
	requests    []BackendRequest
	hanging     chan struct{}
	activeModel string
	reply       string
	chunkDelay  time.Duration
	device      *deviceSignIns
	clock       platform.Clock

	completedStreams int
	abandonedStreams int
}

var _ http.Handler = (*Backend)(nil)

// NewBackend returns a healthy backend on the fixture clock, which starts at the fixture epoch, so its timestamps are
// stable in golden files and tests. A rig that drives a real app uses NewBackendWithClock with a real clock.
func NewBackend() *Backend {
	return NewBackendWithClock(fake.NewClock())
}

// NewBackendWithClock returns a healthy backend that reads time from clock, for the expiry of sign-in codes.
func NewBackendWithClock(clock platform.Clock) *Backend {
	return &Backend{
		mode:        ModeHealthy,
		hanging:     make(chan struct{}, 16),
		activeModel: ActiveModelBody,
		reply:       FixtureReply,
		device:      newDeviceSignIns(),
		clock:       clock,
	}
}

// SetActiveModel replaces the descriptor the active-model route answers with, for tests that need another route
// shape or a spend-cap notice.
func (b *Backend) SetActiveModel(body string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.activeModel = body
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
	b.serveRoute(w, r, body, mode)
}

// serveRoute answers a route the fake implements, for the method the backend accepts on it.
func (b *Backend) serveRoute(w http.ResponseWriter, r *http.Request, body string, mode FailureMode) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == ChatCompletionsPath:
		b.serveChat(w, r, body, mode)
	case r.Method == http.MethodGet && r.URL.Path == ModelsPath:
		writeBody(w, http.StatusOK, "application/json", ModelsBody)
	case r.Method == http.MethodGet && r.URL.Path == ActiveModelPath:
		writeBody(w, http.StatusOK, "application/json", b.currentActiveModel())
	case r.Method == http.MethodGet && r.URL.Path == MePath:
		writeBody(w, http.StatusOK, "application/json", MeBody)
	case r.Method == http.MethodPost && r.URL.Path == DeviceInitiatePath:
		b.serveDeviceInitiate(w, r)
	case r.Method == http.MethodPost && r.URL.Path == DeviceExchangePath:
		b.serveDeviceExchange(w, body)
	case r.Method == http.MethodPost && r.URL.Path == DeviceApprovePath:
		b.serveDeviceApprove(w, body)
	case r.Method == http.MethodPost && r.URL.Path == TranscriptionsPath:
		writeBody(w, http.StatusOK, "application/json", TranscriptionBody)
	case r.URL.Path == TranscriptionPreparationPath && (r.Method == http.MethodPost || r.Method == http.MethodDelete):
		writeBody(w, http.StatusOK, "application/json", OperationBody)
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

// currentActiveModel returns the descriptor the active-model route answers with.
func (b *Backend) currentActiveModel() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.activeModel
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
	case ChatCompletionsPath, ModelsPath, ActiveModelPath, MePath, DeviceInitiatePath, DeviceExchangePath,
		DeviceApprovePath, TranscriptionsPath, TranscriptionPreparationPath:
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
