package app

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/search"
)

func awaitFixtureCommand(t *testing.T, id, query, title string) (*App, *manager.Manager, search.Item) {
	t.Helper()
	p := goldenPalette(t)
	w := uiWindow{stubWindow: &stubWindow{}, jobs: make(chan func(), 1024)}
	p.uiQueue = w
	p.Attach(w)
	m := newManagerWithFakes(t, managerFakes{onChange: p.extensionsStateChanged}, nil, nil, nil, nil)
	p.exts.mgr.Store(m)
	if err := m.Install(id, fixtureFiles(t, id), true); err != nil {
		t.Fatal(err)
	}
	waitForState(t, m, id, supervisor.StateRunning)
	w.Show()
	p.SetQuery(query)
	deadline := time.NewTimer(fixtureWaitTimeout)
	defer deadline.Stop()
	for {
		if p.Query() != query {
			t.Fatalf("background command discovery replaced query %q with %q", query, p.Query())
		}
		var found search.Item
		count := 0
		for _, row := range p.Results() {
			if row.Title == title {
				found = row
				count++
			}
		}
		if count > 1 {
			t.Fatalf("background command discovery duplicated %q: %v", title, rowTitles(p))
		}
		if count == 1 {
			return p, m, found
		}
		select {
		case fn := <-w.jobs:
			fn()
		case <-deadline.C:
			t.Fatalf("query %q did not receive learned command %q without further input: %v", query, title, rowTitles(p))
		}
	}
}
