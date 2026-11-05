package palette

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

type opacityWindow struct {
	*fakeWindow
	opacity float64
}

func (w *opacityWindow) SetOpacity(value float64) { w.opacity = value }

func TestOpeningRevealsOnlyTheFreshFrameAndRejectsOldReveals(t *testing.T) {
	p := New(Options{Root: &View{Title: "root"}, Clock: fake.NewClock(), Synchronous: true})
	w := &opacityWindow{fakeWindow: newFakeWindow(), opacity: 1}
	p.Attach(w)
	tester := ui.NewTester(p.View, 720, 470)
	p.OpenAction("Clipboard History", search.Action{Dismiss: search.DismissNever, Run: func(h search.Host) { h.Push(&View{Title: "Clipboard History"}) }})
	if w.opacity != 0 || !w.IsVisible() {
		t.Fatal("previous backing frame was exposed")
	}
	tester.Frame()
	p.Hide()
	w.drain()
	if w.opacity != 0 {
		t.Fatal("a dismissed opening revealed its obsolete frame")
	}
	p.Show()
	if w.opacity != 0 {
		t.Fatal("new opening exposed the previous frame")
	}
	tester.Frame()
	w.drain()
	if w.opacity != 1 || !w.IsVisible() {
		t.Fatal("fresh rendered frame was not revealed")
	}
}

func TestResummoningBeforeRevealStillRevealsTheNewFrame(t *testing.T) {
	p := New(Options{Root: &View{Title: "root"}, Clock: fake.NewClock(), Synchronous: true})
	w := &opacityWindow{fakeWindow: newFakeWindow(), opacity: 1}
	p.Attach(w)
	tester := ui.NewTester(p.View, 720, 470)
	p.Show()
	tester.Frame()
	p.Open(&View{Title: "Clipboard History"})
	w.drain()
	if w.opacity != 0 {
		t.Fatal("old opening revealed a newer destination before its frame")
	}
	tester.Frame()
	w.drain()
	if w.opacity != 1 {
		t.Fatal("resummoning stranded the window transparent")
	}
}
