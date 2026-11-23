package app

import (
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/search"
)

// awaitFixtureCommand installs extension id, types query while its commands are still being learned, and returns the
// row titled title once the palette lists it. A typed query keeps a fixed set of rows: background command discovery
// never adds to it while the query stays the same, and reopening the palette searches the query again. The empty query
// lists the root's catalog, which fills in as the commands are learned.
func awaitFixtureCommand(t *testing.T, id, query, title string) (*App, *manager.Manager, search.Item) {
	t.Helper()
	p := goldenPalette(t)
	w := uiWindow{stubWindow: &stubWindow{}, jobs: make(chan func(), 1024), thread: new(atomic.Uint64)}
	p.uiQueue = w
	p.Attach(w)
	m := newManagerWithFakes(t, managerFakes{onChange: p.extensionsStateChanged, commandsChange: p.extensionCommandsChanged}, nil, nil, nil, nil)
	p.exts.mgr.Store(m)
	if err := m.Install(id, fixtureFiles(t, id), true); err != nil {
		t.Fatal(err)
	}
	waitForState(t, m, id, supervisor.StateRunning)
	w.Show()
	p.SetQuery(query)
	shown := rowTitles(p)
	deadline := time.NewTimer(fixtureWaitTimeout)
	defer deadline.Stop()
	for {
		if found, ok := fixtureRowOnScreen(t, p, query, title); ok {
			return p, m, found
		}
		select {
		case fn := <-w.jobs:
			fn()
		case <-time.After(10 * time.Millisecond):
		case <-deadline.C:
			t.Fatalf("query %q did not list learned command %q after reopening: %v", query, title, rowTitles(p))
		}
		if p.Query() != query {
			t.Fatalf("background command discovery replaced query %q with %q", query, p.Query())
		}
		if got := rowTitles(p); query != "" && !slices.Equal(got, shown) {
			t.Fatalf("background command discovery changed the rows of %q from %v to %v", query, shown, got)
		}
		p.Show()
		p.SetQuery(query)
		shown = rowTitles(p)
	}
}

// fixtureRowOnScreen returns the one row titled title among the rows of the query on screen.
func fixtureRowOnScreen(t *testing.T, p *App, query, title string) (search.Item, bool) {
	t.Helper()
	var found search.Item
	count := 0
	for _, row := range p.Results() {
		if row.Title == title {
			found = row
			count++
		}
	}
	if count > 1 {
		t.Fatalf("query %q duplicated %q: %v", query, title, rowTitles(p))
	}
	return found, count == 1
}
