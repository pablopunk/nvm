package backend

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/fixtures"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// canaryToken is the bearer token every test uses. The logs test checks that it never reaches a log file, a
// ring entry or an error message.
const canaryToken = "nvm-canary-token-7f3a9c2e51"

// staticTokens is a TokenSource that returns a fixed token or error.
type staticTokens struct {
	token string
	err   error
}

// Get returns the configured token.
func (s staticTokens) Get() (string, error) {
	return s.token, s.err
}

// noJitter returns the delay unchanged, so backoff timing is exact in tests.
func noJitter(d time.Duration) time.Duration {
	return d
}

// harness bundles a client with the fakes it runs on, so tests can script the network and read the logs.
type harness struct {
	client *Client
	clock  *fake.Clock
	logger *logx.Logger
	files  *fake.FS
	http   *fake.HTTP
}

// newHarness builds a production-origin client on fakes. Options are applied after the defaults, so a test can
// override them.
func newHarness(t *testing.T, opts ...Option) harness {
	t.Helper()
	clock := fake.NewClock()
	files := fake.NewFS()
	logger, err := logx.New(files, clock, "logs", logx.Options{FlushInterval: time.Second, Level: logx.Debug})
	if err != nil {
		t.Fatalf("logx.New: %v", err)
	}
	t.Cleanup(func() { _ = logger.Close() })
	httpPort := fake.NewHTTP()
	env, err := NewEnvironment(EnvironmentProduction, "", false)
	if err != nil {
		t.Fatalf("NewEnvironment: %v", err)
	}
	defaults := []Option{WithClock(clock), WithJitter(noJitter), WithClientVersion("1.2.3")}
	client, err := New(httpPort, logger, env, staticTokens{token: canaryToken}, append(defaults, opts...)...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return harness{client: client, clock: clock, logger: logger, files: files, http: httpPort}
}

// withBackend serves the fixture backend for every request that has no scripted answer.
func (h harness) withBackend(t *testing.T) *fixtures.Backend {
	t.Helper()
	backend := fixtures.NewBackend()
	h.http.Handle(backend)
	return backend
}

// useSynchronousWaits makes every wait the client makes advance the fake clock inline, in the calling goroutine.
// The call then finishes before the test continues, and the returned slice lists each wait in order. Use it
// wherever the test needs to know which waits happened, not only when they happened.
func (h harness) useSynchronousWaits() *[]time.Duration {
	waits := &[]time.Duration{}
	h.client.pause = func(ctx context.Context, d time.Duration) error {
		if err := ctx.Err(); err != nil {
			return classifyTransport(err, false, ctx)
		}
		*waits = append(*waits, d)
		h.clock.Advance(d)
		return nil
	}
	return waits
}

// readerHTTP is a platform.HTTP whose response body is produced by open. It lets a test deliver a stream in
// arbitrary read sizes, cut it off, or block it.
type readerHTTP struct {
	status int
	open   func() io.ReadCloser
}

// Do returns the configured response; the request is not inspected.
func (r readerHTTP) Do(req *platform.Request) (*platform.Response, error) {
	return &platform.Response{
		StatusCode: r.status,
		Status:     http.StatusText(r.status),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       r.open(),
		Request:    req,
	}, nil
}

// streamClient returns a client whose only response is a stream of body, read in the given pieces.
func streamClient(t *testing.T, body io.ReadCloser) *Client {
	t.Helper()
	return newClientOver(t, readerHTTP{status: http.StatusOK, open: func() io.ReadCloser { return body }})
}

// newClientOver builds a client over any platform.HTTP port.
func newClientOver(t *testing.T, port platform.HTTP) *Client {
	t.Helper()
	clock := fake.NewClock()
	logger, err := logx.New(nil, clock, "", logx.Options{})
	if err != nil {
		t.Fatalf("logx.New: %v", err)
	}
	t.Cleanup(func() { _ = logger.Close() })
	env, err := NewEnvironment(EnvironmentProduction, "", false)
	if err != nil {
		t.Fatalf("NewEnvironment: %v", err)
	}
	client, err := New(port, logger, env, staticTokens{token: canaryToken}, WithClock(clock), WithJitter(noJitter))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return client
}

// epochPlus returns a timestamp relative to the fake clock epoch, in the form the backend sends.
func epochPlus(d time.Duration) string {
	return fake.NewClock().Now().Add(d).Format(time.RFC3339Nano)
}
