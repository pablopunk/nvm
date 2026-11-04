package app

import (
	"context"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// searchChangeGrid opens the search-results grid fixture on the palette p, with a clock and a UI queue the test drives.
// It returns the queue of UI callbacks and the clock.
func searchChangeGrid(t *testing.T, p *App) (chan func(), *fake.Clock) {
	t.Helper()
	m := viewManager(t)
	clock := fake.NewClock()
	posted := make(chan func(), 4)
	deps := syncViewDeps(m)
	deps.Clock = clock
	deps.OnUI = func(fn func()) { posted <- fn }
	result, failure, err := m.Run(context.Background(), viewFixtureExtension, api.Invocation{Ref: api.ActionRef{ID: "search-results"}})
	if err != nil || failure != nil || result.View == nil {
		t.Fatalf("fixture search-results: err %v failure %+v", err, failure)
	}
	page := extview.NewSession(deps, viewFixtureExtension, *result.View).Open(refreshingHost{App: p})
	goldenOpen(p, page)
	return posted, clock
}

// refreshingHost is the palette as the view's host: a refresh searches the palette's query again, as the window does.
type refreshingHost struct {
	*App
}

// Refresh shows the rows of the view again for the palette's query.
func (h refreshingHost) Refresh() { h.SetQuery(h.Query()) }

// settleSearchChange runs the UI callback the rested search posted, as the palette's thread would.
func settleSearchChange(t *testing.T, posted chan func()) {
	t.Helper()
	select {
	case fn := <-posted:
		fn()
	case <-time.After(10 * time.Second):
		t.Fatal("the search change never reached the UI thread")
	}
}

// TestFixtureSearchChangeAddsTheMeaningMatchesThroughTheChild types "yellow" into the search-results grid fixture,
// rests, and expects the child's second batch of fruit after the local matches, which the name search cannot find.
func TestFixtureSearchChangeAddsTheMeaningMatchesThroughTheChild(t *testing.T) {
	p := goldenPalette(t)
	posted, clock := searchChangeGrid(t, p)
	goldenQuery(p, "yellow")
	if rows := rowLabelsOf(p.Results()); len(rows) != 0 {
		t.Fatalf("local rows before the pause = %v, want none", rows)
	}
	clock.Advance(350 * time.Millisecond)
	settleSearchChange(t, posted)
	if titles := rowLabelsOf(p.Results()); len(titles) != 2 || titles[0] != "Banana" || titles[1] != "Lemon" {
		t.Fatalf("rows = %v, want the child's Banana and Lemon", titles)
	}
}

// TestFixtureSearchChangeGrid is the grid with the child's second batch on screen, as the golden shows it.
func TestFixtureSearchChangeGrid(t *testing.T) {
	p := goldenPalette(t)
	posted, clock := searchChangeGrid(t, p)
	goldenQuery(p, "yellow")
	clock.Advance(350 * time.Millisecond)
	settleSearchChange(t, posted)
	golden.AssertMatrix(t, "extension-search-grid", p.View, paletteGoldens)
}
