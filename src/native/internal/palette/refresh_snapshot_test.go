package palette

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/pablopunk/nvm/native/internal/search"
)

func TestPassiveRefreshKeepsTheWarmRootWhileProvidersRevalidate(t *testing.T) {
	for _, kind := range []string{"host notification", "view refresh", "search refresh"} {
		t.Run(kind, func(t *testing.T) {
			release := make(chan struct{})
			defer close(release)
			var calls atomic.Int32
			apps := search.NewRowsProvider("apps", func(ctx context.Context, _ string) []search.Item {
				if calls.Add(1) > 1 {
					select {
					case <-release:
					case <-ctx.Done():
						return nil
					}
				}
				return []search.Item{{ID: "app", Kind: search.KindApplications, Title: "Cached app", Score: 100}}
			})
			commandRelease := make(chan struct{})
			var commandCalls atomic.Int32
			commands := search.NewRowsProvider("commands", func(ctx context.Context, _ string) []search.Item {
				if commandCalls.Add(1) > 1 {
					select {
					case <-commandRelease:
					case <-ctx.Done():
						return nil
					}
				}
				return []search.Item{{ID: "settings", Kind: search.KindCommands, Title: "Settings", Score: 1}}
			})
			root := &View{Title: "root", Flat: true, Providers: func() []search.Provider { return []search.Provider{apps, commands} }}
			h := newHarness(t, root, false)
			h.awaitRows(func(titles []string) bool { return len(titles) == 2 })
			switch kind {
			case "host notification":
				h.p.ReloadRows()
			case "view refresh":
				h.p.Refresh()
				h.win.drain()
			case "search refresh":
				h.p.RefreshSearch("")
				h.clock.AwaitTimerDueIn(searchRefreshDelay)
				h.clock.Advance(searchRefreshDelay)
				h.win.awaitQueued(t)
				h.win.drain()
			}
			if len(h.p.Results()) != 2 {
				t.Fatal("refresh discarded the cached root")
			}
			close(commandRelease)
			h.win.awaitQueued(t)
			h.frame()
			if rows := h.p.Results(); len(rows) != 2 || rows[0].ID != "app" {
				t.Fatalf("partial refresh replaced the apps with commands: %v", h.titles())
			}
			h.p.coordinator(root).Cancel()
		})
	}
}

// TestFixedRootRefreshUpdatesShownRowsButNotTheirSet refreshes a typed root query after its catalog changed. The row on
// screen takes its new content at once; the row the catalog added waits for the next search of the query.
func TestFixedRootRefreshUpdatesShownRowsButNotTheirSet(t *testing.T) {
	var updated atomic.Bool
	provider := search.NewRowsProvider("apps", func(_ context.Context, q string) []search.Item {
		if q == "" {
			return nil
		}
		if updated.Load() {
			return []search.Item{{ID: "app", Kind: search.KindApplications, Title: "New app"}, {ID: "added", Kind: search.KindApplications, Title: "Added app"}}
		}
		return []search.Item{{ID: "app", Kind: search.KindApplications, Title: "Old app"}}
	})
	root := &View{Title: "root", Flat: true, SingleSnapshot: true, Providers: func() []search.Provider { return []search.Provider{provider} }}
	h := newHarness(t, root, false)
	h.p.SetQuery("lo")
	h.awaitRows(func(titles []string) bool { return len(titles) == 1 && titles[0] == "Old app" })
	updated.Store(true)
	h.p.ForgetCachedResults(func(v *View) bool { return v == root })
	h.p.ReloadRows()
	h.awaitRows(func(titles []string) bool { return len(titles) == 1 && titles[0] == "New app" })
	h.p.SetQuery("other")
	h.awaitRows(func(titles []string) bool { return len(titles) == 2 })
	h.p.SetQuery("lo")
	h.awaitRows(func(titles []string) bool { return len(titles) == 2 })
}
