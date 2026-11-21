package backend

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pablopunk/nvm/native/internal/fixtures"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// loopbackHTTP sends requests over a real TCP connection with the standard client, so the test covers net/http's
// body and cancellation behaviour that the fake ports cannot.
type loopbackHTTP struct {
	client *http.Client
}

// Do sends the request with the standard client.
func (l loopbackHTTP) Do(req *platform.Request) (*platform.Response, error) {
	return l.client.Do(req)
}

// TestSilentLoopbackReplyEndsAsATimeout is the L2 rung for a reply that goes quiet on a real connection: the server
// sends the headers and one content frame, then nothing until the client leaves. The idle window, driven by the fake
// clock, ends the stream as a timeout and the log records it at warn.
func TestSilentLoopbackReplyEndsAsATimeout(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, fixtures.StreamFrames("Hi", 8)[0])
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer close(release)

	clock := fake.NewClock()
	logger, err := logx.New(nil, clock, "", logx.Options{})
	if err != nil {
		t.Fatalf("logx.New: %v", err)
	}
	defer func() { _ = logger.Close() }()
	env, err := NewEnvironment(EnvironmentDevelopment, server.URL, false)
	if err != nil {
		t.Fatalf("NewEnvironment: %v", err)
	}
	client, err := New(loopbackHTTP{client: &http.Client{}}, logger, env, staticTokens{token: canaryToken}, WithClock(clock), WithJitter(noJitter))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	stream, err := client.ChatStream(context.Background(), ChatRequest{Messages: userMessages("hi")})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	defer func() { _ = stream.Close() }()

	events := nextEvents(stream)
	clock.AwaitTimerDueIn(StreamIdleTimeout)
	clock.Advance(StreamIdleTimeout)
	var failure Event
	for event := range events {
		if event.Type == EventError {
			failure = event
		}
	}
	if failure.Type != EventError || kindOf(failure.Err) != KindTimeout {
		t.Fatalf("final event = %+v, want a timeout error after the content frame", failure)
	}
	assertLogged(t, logger, logx.Warn, "backend stream failed", "timeout")
}
