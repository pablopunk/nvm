package palette

import (
	"github.com/pablopunk/nvm/native/internal/search"
	"testing"
)

func TestDefaultChoiceFollowsRankingUntilTheUserChooses(t *testing.T) {
	view := &View{Title: "Root"}
	p := New(Options{Root: view})
	weaker := search.Item{ID: "window-settings", Title: "Window Management Settings"}
	best := search.Item{ID: "settings", Title: "Settings"}
	p.showRows(view, "settings", []search.Item{weaker}, false, true)
	p.showRows(view, "settings", []search.Item{best, weaker}, true, false)
	if p.Chosen() != 0 {
		t.Fatal("default choice stayed on a weaker late-ranked result")
	}
	p.Choose(1)
	p.Choose(0)
	p.showRows(view, "settings", []search.Item{weaker, best}, true, false)
	if p.Chosen() != 1 {
		t.Fatal("refresh lost the user's explicitly chosen first result")
	}
	p.showRows(view, "window", []search.Item{weaker, best}, true, false)
	if p.Chosen() != 0 || p.choicePinned {
		t.Fatal("new query inherited the old choice")
	}
}

func TestRenderedListDefaultChoiceFollowsNewTopResult(t *testing.T) {
	weaker := search.Item{ID: "weaker", Title: "Weaker"}
	best := search.Item{ID: "best", Title: "Best"}
	view := &View{Title: "Root", Search: func(string) []search.Item { return []search.Item{weaker} }}
	h := newHarness(t, view, true)
	h.p.showRows(view, "query", []search.Item{weaker}, false, false)
	h.frame()
	h.p.showRows(view, "query", []search.Item{best, weaker}, true, false)
	h.frame()
	if h.p.Chosen() != 0 || h.p.choicePinned {
		t.Fatalf("list auto-follow overruled ranked default: chosen=%d pinned=%v", h.p.Chosen(), h.p.choicePinned)
	}
	h.p.Choose(1)
	h.frame()
	h.p.showRows(view, "query", []search.Item{weaker, best}, true, false)
	h.frame()
	if h.p.Chosen() != 0 || !h.p.choicePinned {
		t.Fatal("explicit choice did not follow its item")
	}
	h.p.showRows(view, "new query", []search.Item{best, weaker}, true, false)
	h.frame()
	if h.p.Chosen() != 0 || h.p.choicePinned {
		t.Fatal("new query inherited the list's old default item")
	}
}

func TestPinnedChoiceDisappearingReturnsToRankedDefault(t *testing.T) {
	view := &View{Title: "Root"}
	p := New(Options{Root: view})
	p.showRows(view, "q", []search.Item{{ID: "first", Kind: "one"}, {ID: "chosen", Kind: "one"}}, false, true)
	p.Choose(1)
	p.showRows(view, "q", []search.Item{{ID: "first", Kind: "one"}, {ID: "chosen", Kind: "other"}}, true, false)
	if p.Chosen() != 0 || p.choicePinned {
		t.Fatal("removed selection must return to the ranked default")
	}
}

func TestEnterAdmitsLastPresentedRowAcrossLateRanking(t *testing.T) {
	var ran string
	action := func(name string) search.Action {
		return search.Action{Title: name, Dismiss: search.DismissNever, Run: func(search.Host) { ran = name }}
	}
	view := &View{Title: "Root"}
	p := New(Options{Root: view})
	p.query = "pu"
	old := search.Item{ID: "pull", Kind: "command", Publication: 1, Title: "Show Pull Requests", Actions: []search.Action{action("pull")}}
	best := search.Item{ID: "pulse", Kind: "app", Publication: 2, Title: "Pulse", Actions: []search.Action{action("pulse")}}
	p.showRows(view, "pu", []search.Item{old}, false, true)
	p.recordPresentedChoice()
	p.showRows(view, "pu", []search.Item{best, old}, true, false)
	p.onEnter()
	if ran != "pull" {
		t.Fatalf("Enter ran %q, want last presented row", ran)
	}
	p.choicePinned = false
	p.chosen = 0
	p.recordPresentedChoice()
	p.onEnter()
	if ran != "pulse" {
		t.Fatalf("after new frame Enter ran %q, want new top", ran)
	}
}

func TestRemovedPresentedRowCannotRunReplacement(t *testing.T) {
	view := &View{Title: "Root"}
	p := New(Options{Root: view})
	p.query = "pu"
	p.showRows(view, "pu", []search.Item{{ID: "old", Kind: "command", Title: "Old"}}, false, true)
	p.recordPresentedChoice()
	p.showRows(view, "pu", []search.Item{{ID: "new", Kind: "app", Title: "New"}}, true, false)
	if p.admitPresentedChoice() {
		t.Fatal("removed row admitted its replacement")
	}
}

func TestExplicitNavigationOverridesPresentedChoice(t *testing.T) {
	view := &View{Title: "Root"}
	p := New(Options{Root: view})
	p.query = "pu"
	p.showRows(view, "pu", []search.Item{{ID: "a"}, {ID: "b"}}, false, true)
	p.recordPresentedChoice()
	p.Choose(1)
	if !p.admitPresentedChoice() || p.Chosen() != 1 {
		t.Fatal("explicit navigation lost to the prior frame")
	}
}

func TestEnterUsesPresentedActionWhenRowMutatesInPlace(t *testing.T) {
	var ran string
	view := &View{Title: "Root"}
	p := New(Options{Root: view})
	p.query = "pu"
	makeRow := func(name string) search.Item {
		return search.Item{ID: "same", Kind: "command", Title: "Command", Actions: []search.Action{{Title: name, Dismiss: search.DismissNever, Run: func(search.Host) { ran = name }}}}
	}
	p.showRows(view, "pu", []search.Item{makeRow("shown")}, false, true)
	p.recordPresentedChoice()
	p.showRows(view, "pu", []search.Item{makeRow("updated")}, true, false)
	p.onEnter()
	if ran != "" {
		t.Fatalf("undrawn replacement ran action %q", ran)
	}
	p.recordPresentedChoice()
	p.onEnter()
	if ran != "updated" {
		t.Fatalf("new frame did not adopt updated action: %q", ran)
	}
}

func TestActionMenuOwnerIncludesKind(t *testing.T) {
	view := &View{Title: "Root"}
	p := New(Options{Root: view})
	p.showRows(view, "q", []search.Item{{ID: "same", Kind: "app"}}, true, false)
	before := p.menuOwnerKey()
	p.showRows(view, "q", []search.Item{{ID: "same", Kind: "command"}}, true, false)
	if p.menuOwnerKey() == before {
		t.Fatal("different row kinds share an action menu owner")
	}
}

func TestExplicitNavigationCancelsPendingEnter(t *testing.T) {
	view := &View{Title: "Root"}
	p := New(Options{Root: view})
	p.showRows(view, "q", []search.Item{{ID: "a"}, {ID: "b"}}, false, true)
	p.defaultEnter = &defaultEnterIntent{view: view, query: "q"}
	p.Choose(1)
	if p.defaultEnter != nil {
		t.Fatal("navigation retained a pending Enter")
	}
}

func TestReplacementCannotInheritPresentedOrPinnedAction(t *testing.T) {
	for _, tc := range []struct {
		name             string
		old, replacement search.Item
	}{
		{"same ID new publication", search.Item{Provider: "extensions", Source: "one", ID: "same", Kind: "command", Publication: 1}, search.Item{Provider: "extensions", Source: "one", ID: "same", Kind: "command", Publication: 2}},
		{"different extension", search.Item{Provider: "extensions", Source: "one", ID: "same", Kind: "command", Publication: 1}, search.Item{Provider: "extensions", Source: "two", ID: "same", Kind: "command", Publication: 2}},
		{"different provider", search.Item{Provider: "one", ID: "same", Kind: "command", Publication: 1}, search.Item{Provider: "two", ID: "same", Kind: "command", Publication: 2}},
		{"missing ID and same title", search.Item{Provider: "one", Title: "same", Publication: 1}, search.Item{Provider: "one", Title: "same", Publication: 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var ran string
			tc.old.Actions = []search.Action{{Title: "old", Dismiss: search.DismissNever, Run: func(search.Host) { ran = "old" }}}
			tc.replacement.Actions = []search.Action{{Title: "new", Dismiss: search.DismissNever, Run: func(search.Host) { ran = "new" }}}
			v := &View{Title: "Root"}
			p := New(Options{Root: v})
			p.query = "q"
			p.showRows(v, "q", []search.Item{tc.old}, false, true)
			p.Choose(0)
			p.recordPresentedChoice()
			p.showRows(v, "q", []search.Item{tc.replacement}, true, false)
			p.onEnter()
			if ran != "" {
				t.Fatalf("before draw ran %q", ran)
			}
			p.recordPresentedChoice()
			p.onEnter()
			if ran != "new" {
				t.Fatalf("after draw ran %q", ran)
			}
		})
	}
}

func TestDuplicateIDsDoNotPreservePinnedChoice(t *testing.T) {
	v := &View{Title: "Root"}
	p := New(Options{Root: v})
	p.query = "q"
	old := search.Item{Provider: "provider", Source: "extension", Kind: "command", ID: "same", Publication: 1}
	p.showRows(v, "q", []search.Item{old}, false, true)
	p.Choose(0)
	p.recordPresentedChoice()
	p.showRows(v, "q", []search.Item{{Provider: "provider", Source: "extension", Kind: "command", ID: "same", Publication: 2}, {Provider: "provider", Source: "extension", Kind: "command", ID: "same", Publication: 3}}, true, false)
	if p.choicePinned || p.admitPresentedChoice() {
		t.Fatal("ambiguous replacement retained pinned action")
	}
}

func TestRemovedPresentedPublicationCannotBeRevivedBeforeFrame(t *testing.T) {
	v := &View{Title: "Root"}
	p := New(Options{Root: v})
	p.query = "q"
	old := search.Item{Provider: "provider", ID: "same", Publication: 1}
	p.showRows(v, "q", []search.Item{old}, false, true)
	p.recordPresentedChoice()
	p.showRows(v, "q", []search.Item{{Provider: "provider", ID: "other", Publication: 2}}, false, true)
	p.showRows(v, "q", []search.Item{old}, true, false)
	if p.admitPresentedChoice() {
		t.Fatal("removed publication was revived before a frame")
	}
	p.recordPresentedChoice()
	if !p.admitPresentedChoice() {
		t.Fatal("drawn row was not admitted")
	}
}

func TestRemovedPinnedChoiceDoesNotFallBackToPriorPresentedRow(t *testing.T) {
	var ran bool
	v := &View{Title: "Root"}
	p := New(Options{Root: v})
	p.query = "q"
	first := search.Item{ID: "first", Publication: 1, Actions: []search.Action{{Title: "First", Dismiss: search.DismissNever, Run: func(search.Host) { ran = true }}}}
	chosen := search.Item{ID: "chosen", Publication: 2}
	p.showRows(v, "q", []search.Item{first, chosen}, false, true)
	p.recordPresentedChoice()
	p.Choose(1)
	p.showRows(v, "q", []search.Item{first}, true, false)
	p.onEnter()
	if ran {
		t.Fatal("removed pinned row fell back to previously presented action")
	}
	p.recordPresentedChoice()
	p.onEnter()
	if !ran {
		t.Fatal("newly drawn default did not become actionable")
	}
}
