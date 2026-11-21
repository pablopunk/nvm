package app

import (
	"context"
	"sync"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/services"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// fakePresenter is a palette that records what a question did to it. Its methods are safe for the goroutines a
// question uses.
type fakePresenter struct {
	mu      sync.Mutex
	visible bool
	shows   int
	hides   int
	pops    int
	page    *palette.View
	pushed  chan *palette.View
}

func newFakePresenter(visible bool) *fakePresenter {
	return &fakePresenter{visible: visible, pushed: make(chan *palette.View, 1)}
}

func (f *fakePresenter) Visible() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.visible
}

func (f *fakePresenter) Show() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.visible = true
	f.shows++
}

func (f *fakePresenter) Hide() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.visible = false
	f.hides++
}

func (f *fakePresenter) Push(v any) {
	f.mu.Lock()
	f.page = v.(*palette.View)
	f.mu.Unlock()
	f.pushed <- f.page
}

func (f *fakePresenter) PopIf(v *palette.View) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.page != v {
		return false
	}
	f.pops++
	f.page = nil
	return true
}

// presenterHost is the search.Host a question's rows run with: Hide and Pop act on the fake palette.
type presenterHost struct {
	search.Host
	pres *fakePresenter
}

func (h presenterHost) Hide() { h.pres.Hide() }
func (h presenterHost) Pop()  {}

// runFirstRow runs the first row's first action, which is the Confirm row of a question.
func runFirstRow(t *testing.T, view *palette.View, pres *fakePresenter) {
	t.Helper()
	items := view.Search("")
	if len(items) == 0 || len(items[0].Actions) == 0 {
		t.Fatal("the question has no confirm row")
	}
	items[0].Actions[0].Run(presenterHost{pres: pres})
}

// runCancelRow runs the second row, the Cancel row.
func runCancelRow(t *testing.T, view *palette.View, pres *fakePresenter) {
	t.Helper()
	items := view.Search("")
	if len(items) < 2 || len(items[1].Actions) == 0 {
		t.Fatal("the question has no cancel row")
	}
	items[1].Actions[0].Run(presenterHost{pres: pres})
}

var question = services.Question{Extension: "Desktop fixture", Action: "Clear clipboard history"}

// synchronousUI runs a palette change at once, as the UI thread does for a window that is not shown.
func synchronousUI(fn func()) { fn() }

func TestHiddenPaletteShowsTheQuestionAndHidesAgainAfterTheAnswer(t *testing.T) {
	pres := newFakePresenter(false)
	type outcome struct {
		confirmed bool
		err       error
	}
	done := make(chan outcome, 1)
	go func() {
		confirmed, err := awaitQuestion(context.Background(), synchronousUI, pres, question)
		done <- outcome{confirmed, err}
	}()
	view := <-pres.pushed
	runFirstRow(t, view, pres)
	result := <-done
	if result.err != nil || !result.confirmed {
		t.Fatalf("answer = %v (%v), want confirmed", result.confirmed, result.err)
	}
	pres.mu.Lock()
	defer pres.mu.Unlock()
	if pres.shows != 1 {
		t.Fatalf("the hidden palette was shown %d times, want once", pres.shows)
	}
	if pres.visible {
		t.Fatal("the palette stayed visible after the question it was shown for")
	}
}

func TestVisiblePaletteKeepsItsWindowAndTakesTheQuestionBack(t *testing.T) {
	pres := newFakePresenter(true)
	done := make(chan bool, 1)
	go func() {
		confirmed, _ := awaitQuestion(context.Background(), synchronousUI, pres, question)
		done <- confirmed
	}()
	view := <-pres.pushed
	runCancelRow(t, view, pres)
	if confirmed := <-done; confirmed {
		t.Fatal("a cancelled question was confirmed")
	}
	pres.mu.Lock()
	defer pres.mu.Unlock()
	if pres.shows != 0 || pres.hides != 0 {
		t.Fatalf("a visible palette was shown %d or hidden %d times, want neither", pres.shows, pres.hides)
	}
	if !pres.visible {
		t.Fatal("the visible palette was hidden by the question")
	}
}

func TestEndedContextTakesTheQuestionOffAndRefuses(t *testing.T) {
	pres := newFakePresenter(false)
	ctx, stop := context.WithCancel(context.Background())
	stop()
	confirmed, err := awaitQuestion(ctx, synchronousUI, pres, question)
	if confirmed || err != context.Canceled {
		t.Fatalf("answer = %v (%v), want a refusal from the ended context", confirmed, err)
	}
	pres.mu.Lock()
	defer pres.mu.Unlock()
	if pres.pops != 1 {
		t.Fatalf("the question was taken off %d times, want once", pres.pops)
	}
	if pres.visible {
		t.Fatal("the palette this question showed stayed visible")
	}
}
