package backend

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/fixtures"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// pipeHTTP answers every request with a stream the test writes to, or, with silentHeaders, never answers until the
// request is cancelled.
type pipeHTTP struct {
	silentHeaders bool
	body          *io.PipeReader
}

// Do returns the piped stream, or waits for the request to end.
func (p pipeHTTP) Do(req *platform.Request) (*platform.Response, error) {
	if p.silentHeaders {
		<-req.Context().Done()
		return nil, req.Context().Err()
	}
	return &platform.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       p.body,
		Request:    req,
	}, nil
}

// stallRig is a client on a fake clock and an info-level logger, over a port the test controls.
type stallRig struct {
	client *Client
	clock  *fake.Clock
	logger *logx.Logger
}

func newStallRig(t *testing.T, port platform.HTTP) stallRig {
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
	return stallRig{client: client, clock: clock, logger: logger}
}

// nextEvents reads the stream on its own goroutine, so the test can move the clock while a read waits.
func nextEvents(stream *Stream) <-chan Event {
	events := make(chan Event, 64)
	go func() {
		defer close(events)
		for {
			event, err := stream.Next()
			if err != nil {
				return
			}
			events <- event
		}
	}()
	return events
}

// TestSilentStreamEndsAsATimeout opens a reply that never sends a byte: once the idle window passes, the stream ends
// with a timeout instead of waiting forever, and the log says so at warn.
func TestSilentStreamEndsAsATimeout(t *testing.T) {
	reader, writer := io.Pipe()
	defer func() { _ = writer.Close() }()
	rig := newStallRig(t, pipeHTTP{body: reader})
	stream, err := rig.client.ChatStream(context.Background(), ChatRequest{Messages: userMessages("hi")})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	defer func() { _ = stream.Close() }()
	events := nextEvents(stream)
	rig.clock.AwaitTimerDueIn(StreamIdleTimeout)
	rig.clock.Advance(StreamIdleTimeout)
	event := <-events
	if event.Type != EventError || kindOf(event.Err) != KindTimeout {
		t.Fatalf("event = %+v, want a timeout error", event)
	}
	assertLogged(t, rig.logger, logx.Warn, "backend stream failed", "timeout")
}

// TestSilentHeadersEndAsATimeout covers a backend that accepts the request and never answers it.
func TestSilentHeadersEndAsATimeout(t *testing.T) {
	rig := newStallRig(t, pipeHTTP{silentHeaders: true})
	result := make(chan error, 1)
	go func() {
		_, err := rig.client.ChatStream(context.Background(), ChatRequest{Messages: userMessages("hi")})
		result <- err
	}()
	rig.clock.AwaitTimerDueIn(StreamIdleTimeout)
	rig.clock.Advance(StreamIdleTimeout)
	if err := <-result; kindOf(err) != KindTimeout {
		t.Fatalf("ChatStream error = %v, want a timeout", err)
	}
	assertLogged(t, rig.logger, logx.Warn, "backend request failed", "timeout")
}

// TestBytesKeepASlowStreamAlive sends only keep-alive comments for longer than the idle window in total, then the
// reply. Each byte restarts the window, so the stream completes.
func TestBytesKeepASlowStreamAlive(t *testing.T) {
	reader, writer := io.Pipe()
	rig := newStallRig(t, pipeHTTP{body: reader})
	stream, err := rig.client.ChatStream(context.Background(), ChatRequest{Messages: userMessages("hi")})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	defer func() { _ = stream.Close() }()
	events := nextEvents(stream)
	pause := StreamIdleTimeout * 3 / 4
	for range 3 {
		rig.clock.AwaitTimerDueIn(StreamIdleTimeout)
		rig.clock.Advance(pause)
		writeString(t, writer, ": keep-alive\n\n")
	}
	rig.clock.AwaitTimerDueIn(StreamIdleTimeout)
	writeString(t, writer, strings.Join(append(fixtures.StreamFrames("Hello.", 24), fixtures.DoneFrame), ""))
	var text strings.Builder
	for event := range events {
		switch event.Type {
		case EventContent:
			text.WriteString(event.Text)
		case EventError:
			t.Fatalf("stream failed: %v", event.Err)
		}
	}
	if text.String() != "Hello." {
		t.Fatalf("text = %q, want the whole reply", text.String())
	}
	assertLogged(t, rig.logger, logx.Info, "backend stream ended", "done")
}

// TestRequestOutcomesReachTheInfoLog checks an answered call is logged at the app's default level, with the route
// and status and without the token.
func TestRequestOutcomesReachTheInfoLog(t *testing.T) {
	port := fake.NewHTTP()
	port.Handle(fixtures.NewBackend())
	rig := newStallRig(t, port)
	if _, err := rig.client.ActiveModel(context.Background(), ActiveModelQuery{Role: RoleSmart}); err != nil {
		t.Fatalf("ActiveModel: %v", err)
	}
	entry := assertLogged(t, rig.logger, logx.Info, "backend request", "")
	if entry.Fields["path"] != pathActiveModel || fmt.Sprint(entry.Fields["status"]) != "200" {
		t.Fatalf("fields = %v, want the route and its status", entry.Fields)
	}
}

// assertLogged finds the entry with msg at level, whose outcome or kind is detail when detail is not empty.
func assertLogged(t *testing.T, logger *logx.Logger, level logx.Level, msg, detail string) logx.Entry {
	t.Helper()
	for _, entry := range logger.Recent(0) {
		if entry.Level != level || entry.Message != msg {
			continue
		}
		if detail == "" || entry.Fields["kind"] == detail || entry.Fields["outcome"] == detail {
			return entry
		}
	}
	t.Fatalf("no %s entry %q (%s) in %+v", level, msg, detail, logger.Recent(0))
	return logx.Entry{}
}

// writeString writes s to the stream, failing the test when the reader is gone.
func writeString(t *testing.T, writer io.Writer, s string) {
	t.Helper()
	if _, err := io.WriteString(writer, s); err != nil {
		t.Fatalf("write: %v", err)
	}
}
