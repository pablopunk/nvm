package palette

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

// watchdog bounds a wait for an event. It never stands in for one: waits end as soon as the event happens.
const watchdog = 10 * time.Second

// fakeWindow is the palette's window in tests. Update queues its function, as the UI thread would take it
// later; the test runs the queue with drain, so no test depends on goroutine timing.
type fakeWindow struct {
	mu         sync.Mutex
	queued     []func()
	notify     chan struct{}
	visible    bool
	focused    bool
	focusCalls int
	shows      int
	hides      int
	x, y       int
}

func newFakeWindow() *fakeWindow {
	return &fakeWindow{notify: make(chan struct{}, 256)}
}

func (w *fakeWindow) Update(fn func()) {
	w.mu.Lock()
	w.queued = append(w.queued, fn)
	w.mu.Unlock()
	select {
	case w.notify <- struct{}{}:
	default:
	}
}

// awaitQueued blocks until an update is queued, failing the test after the watchdog. A signal the queue has already
// drained is not an update, so the queue is checked after each signal.
func (w *fakeWindow) awaitQueued(t *testing.T) {
	t.Helper()
	deadline := time.After(watchdog)
	for {
		w.mu.Lock()
		queued := len(w.queued)
		w.mu.Unlock()
		if queued > 0 {
			return
		}
		select {
		case <-w.notify:
		case <-deadline:
			t.Fatal("the window was never asked to update")
		}
	}
}

// drain runs the queued functions in order, including those the functions queue in turn.
func (w *fakeWindow) drain() {
	for {
		w.mu.Lock()
		if len(w.queued) == 0 {
			w.mu.Unlock()
			return
		}
		fn := w.queued[0]
		w.queued = w.queued[1:]
		w.mu.Unlock()
		fn()
	}
}

func (w *fakeWindow) Show()           { w.visible, w.focused = true, true; w.shows++ }
func (w *fakeWindow) Hide()           { w.visible, w.focused = false, false; w.hides++ }
func (w *fakeWindow) IsVisible() bool { return w.visible }
func (w *fakeWindow) IsFocused() bool { return w.focused }
func (w *fakeWindow) Focus()          { w.focused = true; w.focusCalls++ }

func (w *fakeWindow) Size() (int, int)     { return 720, 470 }
func (w *fakeWindow) SetPosition(x, y int) { w.x, w.y = x, y }

// takeKeyboard gives the keyboard to another window, as a click on it does, without the palette hearing about it.
func (w *fakeWindow) takeKeyboard() { w.focused = false }

// harness drives a palette the way the window does: keys and text go to the tester, and frames settle.
type harness struct {
	t     *testing.T
	p     *Palette
	win   *fakeWindow
	tt    *ui.Tester
	clock *fake.Clock
}

// newHarness opens the palette on root, shown, with a fake clock and a synchronous or asynchronous search.
func newHarness(t *testing.T, root *View, synchronous bool, opts ...func(*Options)) *harness {
	t.Helper()
	o := Options{Root: root, Clock: fake.NewClock(), Synchronous: synchronous}
	for _, apply := range opts {
		apply(&o)
	}
	clock, ok := o.Clock.(*fake.Clock)
	if !ok {
		t.Fatal("the harness needs a fake clock")
	}
	p := New(o)
	win := newFakeWindow()
	p.Attach(win)
	h := &harness{t: t, p: p, win: win, clock: clock}
	h.tt = ui.NewTester(p.View, 720, 470)
	p.Show()
	h.frame()
	return h
}

// frame renders until the view asks for no more frames, runs what the window queued, and renders again.
func (h *harness) frame() {
	h.tt.Frame()
	h.win.drain()
	h.tt.Frame()
}

// key presses a key with modifiers and settles the frames it causes.
func (h *harness) key(mods ui.Modifiers, key ui.Key) {
	h.tt.Key(mods, key)
	h.win.drain()
	h.tt.Frame()
}

// typeText types into the focused search field.
func (h *harness) typeText(s string) {
	h.tt.Type(s)
	h.frame()
}

// titles returns the titles of the rows on screen, in order.
func (h *harness) titles() []string {
	rows := h.p.Results()
	out := make([]string, len(rows))
	for i, it := range rows {
		out[i] = it.Title
	}
	return out
}

// chosenTitle returns the title of the selected row, or "" without rows.
func (h *harness) chosenTitle() string {
	if it := h.p.chosenItem(); it != nil {
		return it.Title
	}
	return ""
}

// awaitRows drains the window until the rows on screen satisfy done, waiting for each update in turn.
func (h *harness) awaitRows(done func([]string) bool) {
	h.t.Helper()
	for {
		h.win.drain()
		h.tt.Frame()
		if done(h.titles()) {
			return
		}
		h.win.awaitQueued(h.t)
	}
}

// rowsView returns a sub-view whose rows are the given titles; running a row's action records its title.
func rowsView(title string, ran *[]string, titles ...string) *View {
	return &View{
		Title:       title,
		Placeholder: "Search " + title,
		Search: func(q string) []search.Item {
			var out []search.Item
			for _, name := range titles {
				name := name
				out = append(out, search.Item{Kind: "Test", Title: name, Actions: []search.Action{
					{Title: "Open " + name, Run: func(search.Host) { *ran = append(*ran, name) }},
				}})
			}
			return out
		},
	}
}

// fixedProvider answers every query with the rows it is given.
func fixedProvider(id string, rows ...search.Item) search.Provider {
	return search.NewRowsProvider(id, func(_ context.Context, _ string) []search.Item {
		out := make([]search.Item, len(rows))
		copy(out, rows)
		return out
	})
}
