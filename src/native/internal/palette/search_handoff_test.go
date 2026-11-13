package palette

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/search"
)

func TestEmptyPartialSearchKeepsRowsWithoutRunningStaleActions(t *testing.T) {
	provider := search.NewRowsProvider("held", func(ctx context.Context, _ string) []search.Item {
		<-ctx.Done()
		return nil
	})
	root := &View{Title: "Root", Providers: func() []search.Provider { return []search.Provider{provider} }}
	h := newHarness(t, root, false)
	defer root.coord.Cancel()
	ran := 0
	old := search.Item{ID: "old", Title: "Old result", Actions: []search.Action{{Title: "Run old", Mods: Mod, Key: ui.KeyR, Dismiss: search.DismissNever, Run: func(search.Host) { ran++ }}}}
	h.p.showRows(root, "", []search.Item{old}, true, false)
	h.p.openActionMenu(0)
	h.p.SetQuery("new")
	if h.p.actionsOpen {
		t.Fatal("the previous query's action menu remained executable")
	}
	h.p.applySnapshot(root, search.Snapshot{Gen: root.coord.Gen(), Query: "new", Carried: []search.Item{old}})
	h.frame()
	if rows := h.p.Results(); len(rows) != 1 || rows[0].ID != "old" || !h.p.loadingMore() {
		t.Fatalf("empty partial discarded useful rows or loading feedback: rows=%v pending=%v", h.titles(), h.p.loadingMore())
	}
	if len(h.p.menuActions()) != 0 {
		t.Fatal("stale row offered its action menu for the new query")
	}
	h.key(Mod, ui.KeyR)
	h.key(0, ui.KeyEnter)
	if ran != 0 || h.p.defaultEnter == nil {
		t.Fatalf("pending Enter must wait without running an old shortcut: ran=%d intent=%+v", ran, h.p.defaultEnter)
	}
	fresh := search.Item{ID: "new", Title: "New result", Actions: []search.Action{{Title: "Run new", Dismiss: search.DismissNever, Run: func(search.Host) { ran += 10 }}}}
	h.p.applySnapshot(root, search.Snapshot{Gen: root.coord.Gen(), Query: "new", Items: []search.Item{fresh}, Settled: true})
	h.frame()
	if ran != 10 || h.p.Results()[0].ID != "new" {
		t.Fatalf("matching settled result did not resolve Enter once: ran=%d rows=%v", ran, h.titles())
	}
	h.frame()
	if ran != 10 {
		t.Fatal("pending Enter replayed")
	}
}

func TestSettledEmptySearchReplacesRetainedRows(t *testing.T) {
	h := newHarness(t, rowsView("Root", nil, "Old result"), true)
	h.p.query = "no match"
	h.p.showRows(h.p.cur(), "no match", nil, false, false)
	if len(h.p.Results()) != 1 {
		t.Fatal("partial empty result removed the old list")
	}
	h.p.showRows(h.p.cur(), "no match", nil, true, false)
	if len(h.p.Results()) != 0 || h.p.shownQuery != "no match" || !h.p.Settled() {
		t.Fatal("settled empty result did not replace the retained list")
	}
	other := rowsView("Other", nil, "Other result")
	h.p.showRows(h.p.cur(), "no match", []search.Item{{ID: "old", Title: "Old result"}}, true, false)
	h.p.showRows(other, "", nil, false, false)
	if h.p.shownView != other || len(h.p.Results()) != 0 {
		t.Fatal("a new view retained another view's rows")
	}
}

// J15 typed "alphazzz" after "alpha" while one provider never finishes: the rows of "alpha" whose provider answered
// must go, and only the rows of the provider still working stay, so typing does not blank the list.
func TestSupersededQueryKeepsOnlyRowsOfProvidersStillWorking(t *testing.T) {
	h := newHarness(t, rowsView("Root", nil, "alpha"), true)
	v := h.p.cur()
	h.p.showRows(v, "alpha", []search.Item{{ID: "alpha", Title: "alpha"}, {ID: "slow", Title: "slow row"}}, true, false)
	h.p.query = "alphazzz"
	h.p.coordinator(v).Cancel()
	gen := v.coord.Gen()
	h.p.applySnapshot(v, search.Snapshot{Gen: gen, Query: "alphazzz", Pending: true, Carried: []search.Item{{ID: "slow", Title: "slow row"}}})
	if rows := h.titles(); len(rows) != 1 || rows[0] != "slow row" {
		t.Fatalf("expected only the row of the provider still working: %v", rows)
	}
	h.p.applySnapshot(v, search.Snapshot{Gen: gen, Query: "alphazzz", Pending: true})
	if len(h.p.Results()) != 0 || !h.p.loadingMore() {
		t.Fatalf("rows of the superseded query remained: rows=%v", h.titles())
	}
}
