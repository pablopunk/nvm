package app

import (
	"context"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"github.com/pablopunk/nvm/native/internal/search"
)

// lateFixture is the extension whose search answers after the palette's hard budget (internal/ext/testdata/extensions/acme.late).
const lateFixture = "acme.late"

// lateSnapshotWait bounds how long a test waits for the palette to show what it expects.
const lateSnapshotWait = 10 * time.Second

const (
	fastRootTitle    = "Fixture fast root row"
	fastRowTitle     = "Fixture fast row"
	fastCommandTitle = "Fixture fast command"
	lateRowTitle     = "Fixture late row"
	lateCommandTitle = "Fixture late command"
)

// fastBuiltIn is an in-process built-in that answers at once: a command, a root row and a search row.
type fastBuiltIn struct{}

func (fastBuiltIn) Manifest() api.Manifest {
	return api.Manifest{APIVersion: "1.0", Identity: api.Identity{ID: "nevermind.fixture-fast", Title: "Fast fixture"}}
}

func (fastBuiltIn) Commands(api.Context) ([]api.Command, error) {
	return []api.Command{{ID: "fast", Title: fastCommandTitle}}, nil
}

func (fastBuiltIn) RootItems(api.Context) ([]api.Item, error) {
	return []api.Item{{ID: "fast-root", Title: fastRootTitle}}, nil
}

func (fastBuiltIn) SearchItems(api.Context, string) ([]api.Item, error) {
	return []api.Item{{ID: "fast-row", Title: fastRowTitle}}, nil
}

// TestSlowExtensionDoesNotHideBuiltInsOrCommands runs the palette's extensions source on an asynchronous coordinator,
// with a built-in that answers at once and an extension that answers after the hard budget. The built-in's row comes
// before the slow extension's row, every command shows at the root and for a query, and the slow row follows once it
// answers.
func TestSlowExtensionDoesNotHideBuiltInsOrCommands(t *testing.T) {
	m := newManagerWithFakes(t, managerFakes{}, []api.Extension{fastBuiltIn{}}, nil, nil, nil, lateFixture)
	waitForState(t, m, lateFixture, supervisor.StateRunning)
	p := goldenPalette(t)
	p.exts.mgr.Store(m)

	snaps, gen := startExtensionsQuery(p, "fixture")
	seen := waitForSnapshot(t, snaps, gen, func(s search.Snapshot) bool { return hasTitle(rowTitlesOf(s.Items), fastRowTitle) })
	for _, snap := range seen {
		if hasTitle(rowTitlesOf(snap.Items), lateRowTitle) {
			t.Fatalf("the slow extension's row came before the built-in's row: %v", rowTitlesOf(snap.Items))
		}
	}
	waitForSnapshot(t, snaps, gen, func(s search.Snapshot) bool { return hasTitle(rowTitlesOf(s.Items), lateRowTitle) })

	eventuallyShowTitles(t, p, "", fastRootTitle, fastCommandTitle, lateCommandTitle)
	eventuallyShowTitles(t, p, "fixture", fastRowTitle, fastCommandTitle, lateCommandTitle)
}

// startExtensionsQuery runs q through the extensions source of p on an asynchronous coordinator, as the palette runs it.
// Its snapshots arrive on the returned channel, and gen identifies the query's snapshots.
func startExtensionsQuery(p *App, q string) (<-chan search.Snapshot, int) {
	snaps := make(chan search.Snapshot, 256)
	c := search.NewCoordinator(search.Options{Clock: real.NewPorts().Clock, OnSnapshot: func(s search.Snapshot) { snaps <- s }})
	in := &rootInput{}
	commands := search.NewRowsProvider("extension-commands", func(_ context.Context, q string) []search.Item { return extensionCommandRows(p, in, q) })
	c.Query(q, []search.Provider{commands, extensionProvider(p, in)})
	return snaps, c.Gen()
}

// waitForSnapshot reads the snapshots of gen until one satisfies pred, and returns every snapshot of gen read so far.
func waitForSnapshot(t *testing.T, snaps <-chan search.Snapshot, gen int, pred func(search.Snapshot) bool) []search.Snapshot {
	t.Helper()
	var seen []search.Snapshot
	deadline := time.After(lateSnapshotWait)
	for {
		select {
		case snap := <-snaps:
			if snap.Gen != gen {
				continue
			}
			seen = append(seen, snap)
			if pred(snap) {
				return seen
			}
		case <-deadline:
			t.Fatalf("no snapshot satisfied the wait; %d snapshots, last rows %v", len(seen), lastRowTitles(seen))
		}
	}
}

// eventuallyShowTitles runs q until its settled rows carry every title. Commands are learned in the background, so the
// first query after a start may not list them yet, as the palette's next query does not miss them.
func eventuallyShowTitles(t *testing.T, p *App, q string, titles ...string) {
	t.Helper()
	deadline := time.Now().Add(lateSnapshotWait)
	for {
		snaps, gen := startExtensionsQuery(p, q)
		settled := waitForSnapshot(t, snaps, gen, func(s search.Snapshot) bool { return s.Settled })
		rows := rowTitlesOf(settled[len(settled)-1].Items)
		if containsAllTitles(rows, titles) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("query %q showed %v; want all of %v", q, rows, titles)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func containsAllTitles(rows, titles []string) bool {
	for _, title := range titles {
		if !hasTitle(rows, title) {
			return false
		}
	}
	return true
}

func lastRowTitles(snaps []search.Snapshot) []string {
	if len(snaps) == 0 {
		return nil
	}
	return rowTitlesOf(snaps[len(snaps)-1].Items)
}
