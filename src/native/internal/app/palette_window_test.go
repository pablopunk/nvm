package app

import (
	"sync"
	"testing"

	"github.com/pablopunk/nvm/native/internal/search"
)

// queuedWindow is the palette's window in app tests. Update queues its function the way the UI thread takes it
// later; drain runs the queue, so a test never depends on goroutine timing.
type queuedWindow struct {
	mu      sync.Mutex
	queued  []func()
	visible bool
}

func (w *queuedWindow) Update(fn func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.queued = append(w.queued, fn)
}
func (w *queuedWindow) Show()                { w.visible = true }
func (w *queuedWindow) Hide()                { w.visible = false }
func (w *queuedWindow) IsVisible() bool      { return w.visible }
func (w *queuedWindow) IsFocused() bool      { return w.visible }
func (w *queuedWindow) Focus()               {}
func (w *queuedWindow) Size() (int, int)     { return width, height }
func (w *queuedWindow) SetPosition(int, int) {}

// drain runs the queued functions in order, including the ones they queue in turn.
func (w *queuedWindow) drain() {
	for fn := w.next(); fn != nil; fn = w.next() {
		fn()
	}
}

// next pops the oldest queued function, or returns nil when none is queued. Update may be called from any goroutine.
func (w *queuedWindow) next() func() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.queued) == 0 {
		return nil
	}
	fn := w.queued[0]
	w.queued = w.queued[1:]
	return fn
}

// itemTitled returns the first row of the palette whose title is title, failing the test when there is none.
func itemTitled(t *testing.T, p *App, title string) search.Item {
	t.Helper()
	for _, it := range p.Results() {
		if it.Title == title {
			return it
		}
	}
	t.Fatalf("no row titled %q among %v", title, rowTitles(p))
	return search.Item{}
}

// actionTitled returns the action of it whose title is title, failing the test when there is none.
func actionTitled(t *testing.T, it search.Item, title string) search.Action {
	t.Helper()
	for _, a := range it.Actions {
		if a.Title == title {
			return a
		}
	}
	t.Fatalf("row %q has no action %q", it.Title, title)
	return search.Action{}
}

// rowTitles lists the titles of the rows on screen, for failure messages.
func rowTitles(p *App) []string {
	out := make([]string, 0, len(p.Results()))
	for _, it := range p.Results() {
		out = append(out, it.Title)
	}
	return out
}
