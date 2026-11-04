package commands

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/fixtures"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/store"
)

// watchdog bounds a wait for queued work. A wait ends as soon as the work arrives; this never stands in for it.
const watchdog = 10 * time.Second

// staticToken is the bearer every test client carries.
type staticToken string

func (t staticToken) Get() (string, error) { return string(t), nil }

// recordingSurface keeps what the commands showed, in order.
type recordingSurface struct {
	statuses  []Status
	hidden    int
	presented []Result
	toasts    []string
	dismissed int
}

func (s *recordingSurface) ShowStatus(status Status) {
	s.statuses = append(s.statuses, status)
}

func (s *recordingSurface) HideStatus() { s.hidden++ }

func (s *recordingSurface) Present(result Result) {
	s.presented = append(s.presented, result)
}

func (s *recordingSurface) Toast(message string) { s.toasts = append(s.toasts, message) }

func (s *recordingSurface) DismissPalette() { s.dismissed++ }

// last returns the most recent status, failing the test when none was shown.
func (s *recordingSurface) last(t *testing.T) Status {
	t.Helper()
	if len(s.statuses) == 0 {
		t.Fatal("no status was shown")
	}
	return s.statuses[len(s.statuses)-1]
}

// harness is the commands service over fakes, a chat service on a fixture backend, and a UI queue the test runs.
type harness struct {
	t          *testing.T
	set        *fake.Set
	backend    *fixtures.Backend
	chats      *chat.Service
	service    *Service
	surface    *recordingSurface
	connected  bool
	dismissals []func()

	mu     sync.Mutex
	queued []func()
	notify chan struct{}
}

// newHarness builds the service with a signed-in account and a healthy fixture backend.
func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{t: t, set: fake.NewSet(), backend: fixtures.NewBackend(), connected: true, surface: &recordingSurface{}, notify: make(chan struct{}, 256)}
	h.set.HTTP.Handle(h.backend)
	h.build(h.set.HTTP)
	return h
}

// build opens the stores and the two services over transport.
func (h *harness) build(transport platform.HTTP) {
	h.t.Helper()
	logger, err := logx.New(h.set.FS, h.set.Clock, "/logs", logx.Options{FlushInterval: time.Second, Level: logx.Debug})
	if err != nil {
		h.t.Fatalf("logx: %v", err)
	}
	h.t.Cleanup(func() { _ = logger.Close() })
	env, err := backend.NewEnvironment(backend.EnvironmentProduction, "", false)
	if err != nil {
		h.t.Fatalf("environment: %v", err)
	}
	client, err := backend.New(transport, logger, env, staticToken("test-token"),
		backend.WithClock(h.set.Clock),
		backend.WithJitter(func(d time.Duration) time.Duration { return d }),
		backend.WithRetryPolicy(1, 0, 0))
	if err != nil {
		h.t.Fatalf("backend: %v", err)
	}
	st, err := store.Open(h.set.FS, h.set.Clock, "/chats")
	if err != nil {
		h.t.Fatalf("store: %v", err)
	}
	h.t.Cleanup(func() { _ = st.Close() })
	h.chats = chat.New(chat.Options{
		Store:     chat.OpenStore(st, h.set.Clock),
		Clock:     h.set.Clock,
		Client:    func() *backend.Client { return client },
		Connected: func() bool { return h.connected },
		Dispatch:  h.dispatch,
		OnSettled: func(id string, outcome chat.Outcome) { h.service.Settled(id, outcome) },
	})
	h.service = NewService(Options{
		Clock:        h.set.Clock,
		Clipboard:    h.set.Clipboard,
		SelectedText: h.set.SelectedText,
		Paster:       h.set.Paster,
		Focus:        h.set.Focus,
		Chats:        h.chats,
		Client:       func() *backend.Client { return client },
		Connected:    func() bool { return h.connected },
		Surface:      h.surface,
		Dispatch:     h.dispatch,
		Background:   func(_ string, fn func()) { go fn() },
		After:        func(_ time.Duration, fn func()) { h.dismissals = append(h.dismissals, fn) },
		RestoreDelay: 500 * time.Millisecond,
	})
}

// dispatch queues work for the test goroutine, the UI thread of the tests.
func (h *harness) dispatch(fn func()) {
	h.mu.Lock()
	h.queued = append(h.queued, fn)
	h.mu.Unlock()
	select {
	case h.notify <- struct{}{}:
	default:
	}
}

// await runs queued work until cond holds, failing the test when it never does.
func (h *harness) await(what string, cond func() bool) {
	h.t.Helper()
	deadline := time.After(watchdog)
	for {
		h.drain()
		if cond() {
			return
		}
		select {
		case <-h.notify:
		case <-deadline:
			h.t.Fatalf("never reached: %s", what)
		}
	}
}

// drain runs queued work in order until none is left.
func (h *harness) drain() {
	for {
		h.mu.Lock()
		if len(h.queued) == 0 {
			h.mu.Unlock()
			return
		}
		fn := h.queued[0]
		h.queued = h.queued[1:]
		h.mu.Unlock()
		fn()
	}
}

// fireAfter runs the functions the service scheduled with After, in order, on the test goroutine (the UI thread).
func (h *harness) fireAfter() {
	h.t.Helper()
	pending := h.dismissals
	h.dismissals = nil
	for _, fn := range pending {
		fn()
	}
}

// proofreadReplies serves a fixture backend whose chat replies are chosen by request number, so a test can make the
// first proofread answer badly and the retry answer well.
type proofreadReplies struct {
	backend *fixtures.Backend
	replies []string
	calls   int
}

func (p *proofreadReplies) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == fixtures.ChatCompletionsPath {
		index := min(p.calls, len(p.replies)-1)
		p.calls++
		p.backend.SetReply(p.replies[index])
	}
	p.backend.ServeHTTP(w, r)
}

// useReplies makes the chat replies follow replies, one per request.
func (h *harness) useReplies(replies ...string) {
	h.set.HTTP.Handle(&proofreadReplies{backend: h.backend, replies: replies})
}

// fixSource is the source window of a test: a window the keyboard was in when the palette opened.
var fixSource = platform.FocusTarget{Window: "0x1200007", PID: 4242}

// waitForPresented awaits a result page and returns it.
func (h *harness) waitForPresented() Result {
	h.t.Helper()
	h.await("a result is presented", func() bool { return len(h.surface.presented) > 0 })
	return h.surface.presented[0]
}

// awaitTimer advances the fake clock once a timer of d is pending, so a background wait on the clock ends.
func (h *harness) awaitTimer(d time.Duration) {
	h.set.Clock.AwaitTimerDueIn(d)
	h.set.Clock.Advance(d)
}
