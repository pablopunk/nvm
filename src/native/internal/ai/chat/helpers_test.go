package chat

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/fixtures"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/store"
)

// watchdog bounds a wait for the UI queue. Waits end as soon as the work arrives; this never stands in for one.
const watchdog = 10 * time.Second

// queue is the UI thread of a test. The service dispatches onto it from its turn goroutines; the test goroutine
// runs the queue, so every state change happens on one goroutine, as it does in the app.
type queue struct {
	mu     sync.Mutex
	fns    []func()
	notify chan struct{}
}

func newQueue() *queue {
	return &queue{notify: make(chan struct{}, 256)}
}

// dispatch queues fn for the test goroutine.
func (q *queue) dispatch(fn func()) {
	q.mu.Lock()
	q.fns = append(q.fns, fn)
	q.mu.Unlock()
	select {
	case q.notify <- struct{}{}:
	default:
	}
}

// drain runs queued functions in order, including those they queue in turn.
func (q *queue) drain() {
	for {
		q.mu.Lock()
		if len(q.fns) == 0 {
			q.mu.Unlock()
			return
		}
		fn := q.fns[0]
		q.fns = q.fns[1:]
		q.mu.Unlock()
		fn()
	}
}

// staticToken is the bearer every test client carries.
type staticToken string

func (t staticToken) Get() (string, error) { return string(t), nil }

// fixture is one service over a fake file system, a fake clock and a fake backend transport.
type fixture struct {
	t         *testing.T
	set       *fake.Set
	backend   *fixtures.Backend
	store     *Store
	svc       *Service
	q         *queue
	connected bool
	tools     Toolset
	noted     []error
	settled   []Outcome
	logger    *logx.Logger
}

// newScriptedFixture serves a recorded backend, and offers tools to conversations.
func newScriptedFixture(t *testing.T, tools Toolset, backend http.Handler) *fixture {
	t.Helper()
	f := &fixture{t: t, set: fake.NewSet(), connected: true, tools: tools}
	f.set.HTTP.Handle(backend)
	f.build(f.set.HTTP)
	return f
}

// newFixture serves the fixture backend over the fake transport.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{t: t, set: fake.NewSet(), backend: fixtures.NewBackend(), connected: true}
	f.set.HTTP.Handle(f.backend)
	f.build(f.set.HTTP)
	return f
}

// build opens the store and the service over transport.
func (f *fixture) build(transport platform.HTTP) {
	f.t.Helper()
	logger, err := logx.New(f.set.FS, f.set.Clock, "/logs", logx.Options{FlushInterval: time.Second, Level: logx.Debug})
	if err != nil {
		f.t.Fatalf("logx: %v", err)
	}
	f.t.Cleanup(func() { _ = logger.Close() })
	f.logger = logger
	env, err := backend.NewEnvironment(backend.EnvironmentProduction, "", false)
	if err != nil {
		f.t.Fatalf("environment: %v", err)
	}
	client, err := backend.New(transport, logger, env, staticToken("test-token"),
		backend.WithClock(f.set.Clock),
		backend.WithJitter(func(d time.Duration) time.Duration { return d }),
		backend.WithRetryPolicy(1, 0, 0))
	if err != nil {
		f.t.Fatalf("backend: %v", err)
	}
	st, err := store.Open(f.set.FS, f.set.Clock, "/chats")
	if err != nil {
		f.t.Fatalf("store: %v", err)
	}
	f.t.Cleanup(func() { _ = st.Close() })
	f.q = newQueue()
	f.store = OpenStore(st, f.set.Clock)
	f.svc = New(Options{
		Store:     f.store,
		Clock:     f.set.Clock,
		Client:    func() *backend.Client { return client },
		Connected: func() bool { return f.connected },
		Dispatch:  f.q.dispatch,
		NoteError: func(err error) { f.noted = append(f.noted, err) },
		OnSettled: func(_ string, outcome Outcome) { f.settled = append(f.settled, outcome) },
		Tools:     f.tools,
		Logger:    logger,
	})
}

// newChat adds a conversation with the fast tier.
func (f *fixture) newChat() Chat {
	f.t.Helper()
	chat, err := f.svc.NewChat(KindConversation, TierFast)
	if err != nil {
		f.t.Fatalf("NewChat: %v", err)
	}
	return chat
}

// awaitTurn runs queued work until the chat is no longer answering.
func (f *fixture) awaitTurn(id string) {
	f.t.Helper()
	deadline := time.After(watchdog)
	for {
		f.q.drain()
		if !f.svc.Busy(id) {
			return
		}
		select {
		case <-f.q.notify:
		case <-deadline:
			f.t.Fatal("the turn never settled")
		}
	}
}

// awaitText runs queued work until the streaming reply of the chat holds want.
func (f *fixture) awaitText(id, want string) {
	f.t.Helper()
	deadline := time.After(watchdog)
	for {
		f.q.drain()
		if chat, _ := f.svc.Chat(id); lastAssistantText(chat) == want {
			return
		}
		select {
		case <-f.q.notify:
		case <-deadline:
			f.t.Fatalf("reply text never reached %q", want)
		}
	}
}

// messages returns the transcript of the chat, failing the test when it is gone.
func (f *fixture) messages(id string) []Message {
	f.t.Helper()
	chat, ok := f.svc.Chat(id)
	if !ok {
		f.t.Fatalf("chat %s is gone", id)
	}
	return chat.Messages
}

// lastAssistantText returns the text of the most recent assistant message.
func lastAssistantText(chat Chat) string {
	text, _ := LastReply(chat)
	return text
}
