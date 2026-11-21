package palette

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

// TestShortcutKeepsThePaletteOpenWhenTheLoadingPageIsSwappedForTheResult checks the path an extension command takes: it
// pushes a loading page, which replaces the opening surface and becomes the entry view, then pops it and pushes the result.
func TestShortcutKeepsThePaletteOpenWhenTheLoadingPageIsSwappedForTheResult(t *testing.T) {
	root := &View{Title: "root"}
	p := New(Options{Root: root, Clock: fake.NewClock(), Synchronous: true})
	w := &openingWindow{fakeWindow: newFakeWindow(), palette: p}
	p.Attach(w)
	loading := &View{Title: "Screenshots"}
	p.OpenAction("Screenshots", search.Action{Dismiss: search.DismissNever, Run: func(h search.Host) { h.Push(loading) }})
	if p.Top() != loading {
		t.Fatal("the loading page is not on top")
	}
	p.PopPage()
	result := &View{Title: "Screenshots"}
	p.Push(result)
	if !p.Visible() || p.Top() != result {
		t.Fatalf("the palette hid or lost the result: visible=%v", p.Visible())
	}
}
