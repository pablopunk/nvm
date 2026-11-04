package aichat

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/backend/auth"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/store"
)

// goldenWidth and goldenHeight are the palette's size (PLAN.md section 2).
const (
	goldenWidth  = 720
	goldenHeight = 470
)

// watchdog bounds a wait for queued work. Waits end as soon as the work arrives.
const watchdog = 10 * time.Second

// goldenNow is the fixed time the goldens and tests read; every message and chat is dated against it.
var goldenNow = time.Date(2026, time.March, 2, 12, 0, 0, 0, time.UTC)

// testWindow is the palette's window in tests. It runs updates at once and never shows anything.
type testWindow struct{}

func (testWindow) Update(fn func())     { fn() }
func (testWindow) Show()                {}
func (testWindow) Hide()                {}
func (testWindow) IsVisible() bool      { return true }
func (testWindow) IsFocused() bool      { return true }
func (testWindow) Focus()               {}
func (testWindow) Size() (int, int)     { return goldenWidth, goldenHeight }
func (testWindow) SetPosition(x, y int) {}

// rig is a palette over a chat service on a fake store and clock. The service's dispatched work waits in a queue
// that the test runs with settle, so every change happens on the test goroutine, as on the UI thread.
type rig struct {
	// feedback is what the actions said to the HUD, in order.
	feedback []palette.Feedback
	t        *testing.T
	clock    *fake.Clock
	store    *chat.Store
	service  *chat.Service
	palette  *palette.Palette
	root     *palette.View
	copied   []string
	signed   bool
	sign     auth.SignInState
	seq      int

	signInStarts int
	startErr     error
	// previewRows is the extension rows the preview reads, when a test sets it.
	previewRows func(extension string) []PreviewRow

	mu     sync.Mutex
	queued []func()
	notify chan struct{}
}

// newRig builds the rig. client is the backend client of the account, or nil when there is none.
func newRig(t *testing.T, client *backend.Client) *rig {
	t.Helper()
	return newRigWithTools(t, client, chat.Toolset{})
}

// newRigWithTools builds the rig with tools offered to conversations.
func newRigWithTools(t *testing.T, client *backend.Client, tools chat.Toolset) *rig {
	t.Helper()
	clock := fake.NewClock()
	st, err := store.Open(fake.NewFS(), clock, "/chats")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	r := &rig{t: t, clock: clock, signed: true, notify: make(chan struct{}, 256)}
	r.store = chat.OpenStore(st, clock)
	r.service = chat.New(chat.Options{
		Store:     r.store,
		Clock:     clock,
		Client:    func() *backend.Client { return client },
		Connected: func() bool { return r.signed },
		Dispatch:  r.dispatch,
		Tools:     tools,
	})
	r.root = &palette.View{Title: "Nevermind", Search: func(string) []search.Item { return nil }}
	r.palette = palette.New(palette.Options{Root: r.root, Clock: clock, Synchronous: true, Feedback: func(f palette.Feedback) { r.feedback = append(r.feedback, f) }})
	r.palette.Attach(testWindow{})
	return r
}

// dispatch queues work for the test goroutine.
func (r *rig) dispatch(fn func()) {
	r.mu.Lock()
	r.queued = append(r.queued, fn)
	r.mu.Unlock()
	select {
	case r.notify <- struct{}{}:
	default:
	}
}

// settle runs the queued work, in order, until none is left.
func (r *rig) settle() {
	for {
		r.mu.Lock()
		if len(r.queued) == 0 {
			r.mu.Unlock()
			return
		}
		fn := r.queued[0]
		r.queued = r.queued[1:]
		r.mu.Unlock()
		fn()
	}
}

// awaitIdle settles the chat's turn, failing the test if it never ends.
func (r *rig) awaitIdle(id string) {
	r.t.Helper()
	deadline := time.After(watchdog)
	for {
		r.settle()
		if !r.service.Busy(id) {
			return
		}
		select {
		case <-r.notify:
		case <-deadline:
			r.t.Fatal("the turn never settled")
		}
	}
}

// deps is the page dependencies over the rig.
func (r *rig) deps() Deps {
	return Deps{
		Service:  r.service,
		Reply:    drawReply,
		Copy:     func(text string) { r.copied = append(r.copied, text) },
		SignedIn: func() bool { return r.signed },
		SignIn:   func() auth.SignInState { return r.sign },
		StartSignIn: func() error {
			r.signInStarts++
			return r.startErr
		},
		Now:         func() time.Time { return goldenNow },
		PreviewRows: r.extensionRows,
	}
}

// extensionRows is the preview source of the rig: nil when the test sets none, so no chat offers a preview.
func (r *rig) extensionRows(extension string) []PreviewRow {
	if r.previewRows == nil {
		return nil
	}
	return r.previewRows(extension)
}

// chatWith creates a conversation with a transcript, dated against goldenNow. With no transcript it is an opened chat.
func (r *rig) chatWith(title string, tier chat.Tier, messages ...chat.Message) chat.Chat {
	r.t.Helper()
	return r.seedChat(chat.KindConversation, tier, title, "", messages)
}

// seedChat records a chat of kind with a transcript and the given title and extension, dated against goldenNow, and
// reloads the service. A chat with no transcript is only opened, since the service records a chat with its first
// message.
func (r *rig) seedChat(kind chat.Kind, tier chat.Tier, title, extension string, messages []chat.Message) chat.Chat {
	r.t.Helper()
	if len(messages) == 0 {
		return r.openChat(kind, tier, extension)
	}
	r.seq++
	record := chat.Chat{
		ID: fmt.Sprintf("chat-seed-%d", r.seq), Kind: kind, Tier: tier, Title: title, Extension: extension,
		Messages: messages, CreatedAt: goldenNow, UpdatedAt: goldenNow,
	}
	if err := r.store.Insert(record); err != nil {
		r.t.Fatalf("seed chat: %v", err)
	}
	r.service.Reload()
	record, _ = r.service.Chat(record.ID)
	return record
}

// openChat opens a chat of kind on tier, bound to extension when it is not empty. Nothing is recorded until a message.
func (r *rig) openChat(kind chat.Kind, tier chat.Tier, extension string) chat.Chat {
	r.t.Helper()
	record, err := r.service.NewChat(kind, tier)
	if err != nil {
		r.t.Fatalf("NewChat: %v", err)
	}
	if extension != "" {
		if err := r.service.Bind(record.ID, extension); err != nil {
			r.t.Fatalf("Bind: %v", err)
		}
	}
	record, _ = r.service.Chat(record.ID)
	return record
}

// open shows view above the root with an empty query, as a push does: the palette gives the keyboard to the page.
func (r *rig) open(view *palette.View) {
	r.palette.SetStack(r.root)
	r.palette.Push(view)
}

// user, assistant, streaming, stopped and notice build transcript entries for the goldens and tests.
func user(text string) chat.Message {
	return chat.Message{Author: chat.AuthorUser, Text: text, Status: chat.StatusComplete, At: goldenNow}
}

func assistant(text string) chat.Message {
	return chat.Message{Author: chat.AuthorAssistant, Text: text, Status: chat.StatusComplete, At: goldenNow}
}

func streaming(text string) chat.Message {
	return chat.Message{Author: chat.AuthorAssistant, Text: text, Status: chat.StatusStreaming, At: goldenNow}
}

func stopped(text string) chat.Message {
	return chat.Message{Author: chat.AuthorAssistant, Text: text, Status: chat.StatusStopped, At: goldenNow}
}

func notice(n chat.Notice) chat.Message {
	return chat.Message{Author: chat.AuthorNotice, Text: n.Message, Status: chat.StatusComplete, Notice: &n, At: goldenNow}
}

// drawReply is the reply renderer the tests use: the same Markdown the app draws.
func drawReply(c *ui.Context, src string, streaming bool) {
	options := &markdown.Options{}
	if streaming {
		markdown.Stream(c, src, options)
		return
	}
	markdown.View(c, src, options)
}

// said reports whether the HUD was told text, in a message of its own or in a feedback's title.
func (r *rig) said(text string) bool {
	for _, f := range r.feedback {
		if strings.Contains(f.Message, text) {
			return true
		}
	}
	return false
}
