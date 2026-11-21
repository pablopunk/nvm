package app

import (
	"context"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// slowSearchFixture is the dev fixture of a grid and a list whose onSearchChange answers late
// (internal/ext/testdata/extensions/acme.slowsearch).
const slowSearchFixture = "acme.slowsearch"

// typingPause is the palette's pause before an onSearchChange call starts: the session's own debounce, restated.
const typingPause = 350 * time.Millisecond

// loadingSearch is a fixture view whose onSearchChange call the test holds, so the batch stays loading for a golden.
type loadingSearch struct {
	clock   *fake.Clock
	session *extview.Session
	ui      chan func()
}

// openSlowSearch shows the fixture view id above the root with an empty query, as the palette does when the user runs
// its command. Its handler's calls are held: nothing answers them.
func openSlowSearch(t *testing.T, p *App, id string) *loadingSearch {
	t.Helper()
	m := newFixtureManager(t, slowSearchFixture)
	waitForState(t, m, slowSearchFixture, supervisor.StateRunning)
	result, failure, err := m.Run(context.Background(), slowSearchFixture, api.Invocation{Ref: api.ActionRef{ID: id}})
	if err != nil || failure != nil || result.View == nil {
		t.Fatalf("open %s: err %v failure %+v", id, err, failure)
	}
	ls := &loadingSearch{clock: fake.NewClock(), ui: make(chan func(), 8)}
	deps := extview.Deps{
		Invoke: func(string, api.Invocation, func(api.Result, *api.Failure)) {},
		OnUI:   func(fn func()) { ls.ui <- fn },
		Clock:  ls.clock,
	}
	ls.session = extview.NewSession(deps, slowSearchFixture, *result.View)
	ls.session.Open(&viewHost{})
	goldenOpen(p, ls.session.Palette())
	return ls
}

// typeAndRest types text into the view and lets the typing pause pass, so the handler is called and holds the batch.
func (ls *loadingSearch) typeAndRest(p *App, text string) {
	goldenQuery(p, text)
	ls.clock.AwaitTimers(1)
	ls.clock.Advance(typingPause)
	(<-ls.ui)()
}

// TestSlowSearchGridShowsItsSecondBatchLoadingBehindTheRows is the grid while onSearchChange runs for text that matches
// some rows: the placeholder cells follow the rows.
func TestSlowSearchGridShowsItsSecondBatchLoadingBehindTheRows(t *testing.T) {
	p := goldenPalette(t)
	openSlowSearch(t, p, "grid").typeAndRest(p, "a")
	golden.AssertMatrix(t, "slow-grid-loading", p.View, viewsGoldens)
}

// TestSlowSearchGridShowsOnlyPlaceholdersWithNoLocalMatch is the grid while the handler runs for text no row matches.
func TestSlowSearchGridShowsOnlyPlaceholdersWithNoLocalMatch(t *testing.T) {
	p := goldenPalette(t)
	openSlowSearch(t, p, "grid").typeAndRest(p, "zz")
	golden.AssertMatrix(t, "slow-grid-loading-empty", p.View, viewsGoldens)
}

// TestSlowSearchListShowsItsSecondBatchLoadingBehindTheRows is the list while onSearchChange runs for text that matches
// some rows: the loading row follows them.
func TestSlowSearchListShowsItsSecondBatchLoadingBehindTheRows(t *testing.T) {
	p := goldenPalette(t)
	openSlowSearch(t, p, "list").typeAndRest(p, "a")
	golden.AssertMatrix(t, "slow-list-loading", p.View, viewsGoldens)
}

// TestSlowSearchListShowsOnlyTheLoadingRowWithNoLocalMatch is the list while the handler runs for text no row matches.
func TestSlowSearchListShowsOnlyTheLoadingRowWithNoLocalMatch(t *testing.T) {
	p := goldenPalette(t)
	openSlowSearch(t, p, "list").typeAndRest(p, "zz")
	golden.AssertMatrix(t, "slow-list-loading-empty", p.View, viewsGoldens)
}
