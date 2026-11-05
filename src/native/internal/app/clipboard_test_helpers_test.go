package app

import (
	"github.com/pablopunk/nvm/native/internal/search"
	"testing"
)

// Limits of the root's copy rows: the empty root shows up to rootClipboardLimit, and a search shows up to
// rootSearchClipboardLimit.
const (
	rootClipboardLimit       = 10
	rootSearchClipboardLimit = 5
)

// seededClipPalette is the palette over a fixture holding the golden entries, opened on the Clipboard History list.
func seededClipPalette(t *testing.T) (*App, *testWindow, *clipFixture) {
	t.Helper()
	f := newClipFixture(t, clipDefaultLimit)
	seedGoldenClipboard(t, f)
	p := goldenPalette(t)
	p.clip = f.engine
	win := &testWindow{}
	p.Attach(win)
	goldenOpen(p, clipboardBuiltInView(t, f.engine))
	return p, win, f
}

type testWindow struct {
	visible bool
	hides   int
}

// Update runs fn at once; a test is its own UI thread.
func (w *testWindow) Update(fn func()) { fn() }

// Show marks the window visible.
func (w *testWindow) Show() { w.visible = true }

// Hide marks the window hidden and counts it.
func (w *testWindow) Hide() {
	w.visible = false
	w.hides++
}

// IsVisible reports the visibility the test set.
func (w *testWindow) IsVisible() bool { return w.visible }
func (w *testWindow) IsFocused() bool { return w.visible }
func (w *testWindow) Focus()          {}

// Size is the palette's size.
func (w *testWindow) Size() (int, int) { return width, height }

// SetPosition is a no-op: a test window has no screen.
func (w *testWindow) SetPosition(int, int) {}

// clipPalette returns the golden palette showing the clipboard history of f, with a window that can hide.

func actionTitles(actions []search.Action) []string {
	titles := make([]string, 0, len(actions))
	for _, a := range actions {
		titles = append(titles, a.Title)
	}
	return titles
}
