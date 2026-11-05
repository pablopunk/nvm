package aichat

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/design/golden"
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

// rig is a palette over a chat service on a fake store and clock. The service's dispatched work waits in a queue,
// so every change happens on the test goroutine, as on the UI thread.
type rig struct {
	// feedback is what the actions said to the HUD, in order.
	feedback []palette.Feedback
	t        *testing.T
	clock    *fake.Clock
	store    *chat.Store
	service  *chat.Service
	palette  *palette.Palette
	root     *palette.View
	signed   bool

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

// open shows view above the root with an empty query, as a push does: the palette gives the keyboard to the page.
func (r *rig) open(view *palette.View) {
	r.palette.SetStack(r.root)
	r.palette.Push(view)
}

// user builds a transcript entry for the goldens and tests.
func user(text string) chat.Message {
	return chat.Message{Author: chat.AuthorUser, Text: text, Status: chat.StatusComplete, At: goldenNow}
}

// pageGoldens renders a chat page at the palette's size. Each golden is compared in light and dark, at 1x and 2x.
var pageGoldens = golden.Options{Group: "aichat", Width: goldenWidth, Height: goldenHeight}

// testPicture is a PNG of the given size and shade, read as a picture the composer attaches.
func testPicture(t *testing.T, name string, w, h int, shade uint8) chat.Image {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: shade, G: uint8(x * 255 / w), B: uint8(y * 255 / h), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	picture, err := chat.NewImage(name, buf.Bytes())
	if err != nil {
		t.Fatalf("NewImage: %v", err)
	}
	return picture
}
