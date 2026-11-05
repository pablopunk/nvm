package builder

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/fixtures"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/store"
)

// turnFixture runs real builder turns: a chat service over a fake streaming backend, the workshop's tools, and a UI
// queue the test drains, as the UI thread does.
type turnFixture struct {
	t          *testing.T
	transport  *fixtures.StreamingTransport
	svc        *chat.Service
	workshop   *Workshop
	extensions *fakeExtensions
	clock      *fake.Clock
	mu         sync.Mutex
	queue      []func()
	notify     chan struct{}
}

type turnToken string

func (t turnToken) Get() (string, error) { return string(t), nil }

// newTurnFixture builds the fixture over a fresh fake file system and clock.
func newTurnFixture(t *testing.T) *turnFixture {
	t.Helper()
	f := &turnFixture{t: t, transport: fixtures.NewStreamingTransport(), clock: fake.NewClock(), notify: make(chan struct{}, 1), extensions: newFakeExtensions()}
	set := fake.NewSet()
	logger, err := logx.New(set.FS, f.clock, "/logs", logx.Options{FlushInterval: time.Second, Level: logx.Debug})
	if err != nil {
		t.Fatalf("logx: %v", err)
	}
	t.Cleanup(func() { _ = logger.Close() })
	env, err := backend.NewEnvironment(backend.EnvironmentProduction, "", false)
	if err != nil {
		t.Fatalf("environment: %v", err)
	}
	client, err := backend.New(f.transport, logger, env, turnToken("test-token"),
		backend.WithClock(f.clock), backend.WithJitter(func(d time.Duration) time.Duration { return d }), backend.WithRetryPolicy(1, 0, 0))
	if err != nil {
		t.Fatalf("backend: %v", err)
	}
	st, err := store.Open(set.FS, f.clock, "/chats")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	var workshop *Workshop
	f.svc = chat.New(chat.Options{
		Store:        chat.OpenStore(st, f.clock),
		Clock:        f.clock,
		Client:       func() *backend.Client { return client },
		Connected:    func() bool { return true },
		Dispatch:     f.dispatch,
		Logger:       logger,
		BuilderTools: func(id string) chat.Toolset { return workshop.Toolset(id) },
		OnSettled:    func(id string, outcome chat.Outcome) { workshop.Settled(id, outcome) },
		SystemPrompt: func(c chat.Chat) string { return workshop.SystemPrompt(c) },
	})
	workshop = New(f.svc, f.extensions, f.dispatch)
	f.workshop = workshop
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	go f.pump(stop)
	return f
}

// dispatch queues UI work for the pump, which runs it in order on its own goroutine: the UI thread of the fixture.
func (f *turnFixture) dispatch(fn func()) {
	f.mu.Lock()
	f.queue = append(f.queue, fn)
	f.mu.Unlock()
	select {
	case f.notify <- struct{}{}:
	default:
	}
}

// pump runs the queued UI work until the test ends.
func (f *turnFixture) pump(stop <-chan struct{}) {
	for {
		select {
		case <-f.notify:
			f.drain()
		case <-stop:
			return
		}
	}
}

// drain runs the queued UI work until none is left.
func (f *turnFixture) drain() {
	for {
		f.mu.Lock()
		if len(f.queue) == 0 {
			f.mu.Unlock()
			return
		}
		fn := f.queue[0]
		f.queue = f.queue[1:]
		f.mu.Unlock()
		fn()
	}
}

// onUI runs fn on the UI thread and waits for it.
func (f *turnFixture) onUI(fn func()) {
	done := make(chan struct{})
	f.dispatch(func() {
		fn()
		close(done)
	})
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		f.t.Fatal("the UI thread did not run the work")
	}
}

// awaitSettled waits until the chat is no longer answering, failing if the turn never settles.
func (f *turnFixture) awaitSettled(id string) {
	f.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		busy := false
		f.onUI(func() { busy = f.svc.Busy(id) })
		if !busy {
			return
		}
		if time.Now().After(deadline) {
			f.t.Fatal("the turn never settled")
		}
		time.Sleep(time.Millisecond)
	}
}

// nextReply returns the writer of the next model step the service opened.
func (f *turnFixture) nextReply() io.WriteCloser {
	f.t.Helper()
	select {
	case writer := <-f.transport.Replies():
		return writer
	case <-time.After(10 * time.Second):
		f.t.Fatal("no model step was requested")
	}
	return nil
}

// toolCallReply streams one model step that calls a tool and ends the stream.
func toolCallReply(writer io.Writer, id, name string, args any) {
	encoded, _ := json.Marshal(args)
	frames := []string{
		fmt.Sprintf(`data: {"object":"chat.completion.chunk","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":%q,"type":"function","function":{"name":%q,"arguments":%q}}]}}]}`+"\n\n", id, name, string(encoded)),
		`data: {"object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}` + "\n\n",
		fixtures.DoneFrame,
	}
	for _, frame := range frames {
		_, _ = io.WriteString(writer, frame)
	}
}

// turnRequestBudgetBytes bounds each request a builder turn sends with the terse reference. The one request a turn used
// to send, with the full reference, was 46827 bytes; the chat and builder API merged onto the built-ins grew the terse
// reference past that, so the budget is the smallest KiB multiple that passes.
const turnRequestBudgetBytes = 47 << 10

// TestBuilderTurnWritesWithThePreloadedReferenceAndBindsTheChat runs one builder turn end to end on the fake backend:
// the model writes the extension through write_extension from the terse reference it was given, and then answers in a
// sentence. The extension is installed enabled, the chat is bound to it, and the transcript keeps no source. The turn
// takes two model requests, each under the turn request budget.
func TestBuilderTurnWritesWithThePreloadedReferenceAndBindsTheChat(t *testing.T) {
	f := newTurnFixture(t)
	var id string
	f.onUI(func() {
		record, err := f.svc.NewChat(chat.KindBuilder, chat.TierSmart)
		if err != nil {
			t.Errorf("NewChat: %v", err)
		}
		id = record.ID
		if err := f.svc.Send(id, "a clock in the palette"); err != nil {
			t.Errorf("Send: %v", err)
		}
	})

	source := "package main\n\nfunc SearchItems(ctx api.Context, query string) ([]api.Item, error) { return nil, nil }\n"
	first := f.nextReply()
	toolCallReply(first, "call-1", writeToolName, map[string]any{
		"manifest": `{"apiVersion":"1.0","id":"user.clock","title":"Clock"}`,
		"source":   source,
	})
	_ = first.Close()

	second := f.nextReply()
	_, _ = io.WriteString(second, fixtures.StreamFrames("Installed the clock. It is not run yet.", 24)[0])
	_, _ = io.WriteString(second, fixtures.DoneFrame)
	_ = second.Close()
	f.awaitSettled(id)

	f.onUI(func() {
		stored, _ := f.svc.Chat(id)
		if stored.Extension != "user.clock" {
			t.Errorf("chat bound to %q, want user.clock", stored.Extension)
		}
		for _, message := range stored.Messages {
			if message.Author != chat.AuthorTool && strings.Contains(message.Text, "func SearchItems") {
				t.Errorf("the transcript shows the source: %q", message.Text)
			}
		}
	})
	if strings.Join(f.extensions.installs, ",") != "user.clock" {
		t.Fatalf("installs = %v, want user.clock installed once", f.extensions.installs)
	}
	writeBytes, answerBytes := len(f.transport.Body(0)), len(f.transport.Body(1))
	t.Logf("builder turn: 2 model steps; request bytes %d, %d; preloaded reference %d bytes; instructions %d bytes",
		writeBytes, answerBytes, len(apiReferenceCompact), len(instructionsText))
	for step, bytes := range []int{writeBytes, answerBytes} {
		if bytes >= turnRequestBudgetBytes {
			t.Fatalf("request %d is %d bytes, over the %d-byte turn request budget", step, bytes, turnRequestBudgetBytes)
		}
	}
}
