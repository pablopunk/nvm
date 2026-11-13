package palette

import (
	"fmt"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/search"
)

// historyHarness opens the palette on a root of the given rows with the history wired as the app wires it.
func historyHarness(t *testing.T, h *QueryHistory, rows ...string) (*harness, *[]string) {
	t.Helper()
	var ran []string
	root := rowsView("Nevermind", &ran, rows...)
	hr := newHarness(t, root, true, func(o *Options) { o.History = h })
	return hr, &ran
}

func TestNewQueryHistoryDropsBlanksAndRepeatsAndKeepsTheNewest(t *testing.T) {
	saved := []string{" alpha ", "", "beta", "alpha", "  "}
	if got, want := NewQueryHistory(saved, nil).Entries(), []string{"beta", "alpha"}; !slices.Equal(got, want) {
		t.Fatalf("entries = %v, want %v (the last occurrence of a repeat wins)", got, want)
	}

	var many []string
	for i := 0; i < QueryHistoryLimit+5; i++ {
		many = append(many, fmt.Sprintf("q%03d", i))
	}
	got := NewQueryHistory(many, nil).Entries()
	if len(got) != QueryHistoryLimit || got[0] != "q005" || got[len(got)-1] != fmt.Sprintf("q%03d", QueryHistoryLimit+4) {
		t.Fatalf("entries after the limit = %d long, first %q, last %q", len(got), got[0], got[len(got)-1])
	}
}

func TestRememberMovesARepeatToTheEndAndPersists(t *testing.T) {
	var persisted [][]string
	h := NewQueryHistory(nil, func(entries []string) { persisted = append(persisted, entries) })
	h.Remember("alpha")
	h.Remember(" beta ")
	h.Remember("alpha")
	if got, want := h.Entries(), []string{"beta", "alpha"}; !slices.Equal(got, want) {
		t.Fatalf("entries = %v, want %v", got, want)
	}
	if len(persisted) != 3 || !slices.Equal(persisted[2], []string{"beta", "alpha"}) {
		t.Fatalf("persist calls = %v, want one per remembered query", persisted)
	}
	h.Remember("   ")
	if len(persisted) != 3 {
		t.Fatal("a blank query is not remembered and not persisted")
	}
}

func TestRememberKeepsTheLimitAtOneHundred(t *testing.T) {
	h := NewQueryHistory(nil, nil)
	for i := 0; i < QueryHistoryLimit+1; i++ {
		h.Remember(fmt.Sprintf("q%03d", i))
	}
	got := h.Entries()
	if len(got) != QueryHistoryLimit || got[0] != "q001" {
		t.Fatalf("after %d queries: %d entries, first %q", QueryHistoryLimit+1, len(got), got[0])
	}
}

func TestOlderAndNewerWalkAndStopAtTheEnds(t *testing.T) {
	h := NewQueryHistory([]string{"alpha", "beta", "gamma"}, nil)
	if _, ok := h.Newer(); ok {
		t.Fatal("Newer without a walk should report nothing")
	}
	steps := []struct{ want string }{{"gamma"}, {"beta"}, {"alpha"}, {"alpha"}}
	for i, step := range steps {
		got, ok := h.Older("typed")
		if !ok || got != step.want {
			t.Fatalf("Older step %d = %q, %v; want %q", i, got, ok, step.want)
		}
	}
	for _, want := range []string{"beta", "gamma", "typed"} {
		got, ok := h.Newer()
		if !ok || got != want {
			t.Fatalf("Newer = %q, %v; want %q", got, ok, want)
		}
	}
	if h.Navigating() {
		t.Fatal("walking past the newest query should end the walk and return the draft")
	}
}

func TestOlderWithoutEntriesReportsNothing(t *testing.T) {
	h := NewQueryHistory(nil, nil)
	if _, ok := h.Older("typed"); ok || h.Navigating() {
		t.Fatal("an empty history recalls nothing")
	}
}

func TestRestoreReturnsTheTypedTextAndEndsTheWalk(t *testing.T) {
	h := NewQueryHistory([]string{"alpha"}, nil)
	h.Older("typed")
	got, ok := h.Restore()
	if !ok || got != "typed" || h.Navigating() {
		t.Fatalf("Restore = %q, %v, navigating %v; want typed, true, false", got, ok, h.Navigating())
	}
	if _, ok := h.Restore(); ok {
		t.Fatal("Restore without a walk reports nothing")
	}
}

func TestResetEndsTheWalkAndForgetsTheDraft(t *testing.T) {
	h := NewQueryHistory([]string{"alpha", "beta"}, nil)
	h.Older("typed")
	h.Reset()
	if h.Navigating() {
		t.Fatal("Reset should end the walk")
	}
	h.Older("new text")
	if got, _ := h.Restore(); got != "new text" {
		t.Fatalf("after a reset the next walk's draft = %q, want the text on the field", got)
	}
}

func TestArrowUpRecallsTheNewestQueryAndArrowDownWalksBack(t *testing.T) {
	history := NewQueryHistory([]string{"alpha", "beta", "gamma"}, nil)
	h, _ := historyHarness(t, history, "Alpha", "Beta")

	h.key(0, ui.KeyUp)
	if got := h.p.Query(); got != "gamma" {
		t.Fatalf("ArrowUp in an empty field recalled %q, want gamma", got)
	}
	h.key(0, ui.KeyUp)
	h.key(0, ui.KeyUp)
	h.key(0, ui.KeyUp)
	if got := h.p.Query(); got != "alpha" {
		t.Fatalf("ArrowUp past the oldest query recalled %q, want alpha (clamped)", got)
	}
	h.key(0, ui.KeyDown)
	if got := h.p.Query(); got != "beta" {
		t.Fatalf("ArrowDown recalled %q, want beta", got)
	}
	h.key(0, ui.KeyDown)
	h.key(0, ui.KeyDown)
	if got := h.p.Query(); got != "" || history.Navigating() {
		t.Fatalf("ArrowDown past the newest query should restore the empty draft: query %q, navigating %v", got, history.Navigating())
	}
}

func TestArrowUpKeepsTheTypedTextAsTheDraft(t *testing.T) {
	history := NewQueryHistory([]string{"alpha", "beta"}, nil)
	h, _ := historyHarness(t, history, "Alpha", "Beta")
	h.typeText("al")
	h.key(0, ui.KeyUp)
	if got := h.p.Query(); got != "beta" {
		t.Fatalf("ArrowUp after typing al recalled %q, want beta", got)
	}
	h.key(0, ui.KeyDown)
	if got := h.p.Query(); got != "al" {
		t.Fatalf("ArrowDown back should restore the typed text, got %q", got)
	}
}

func TestEscapeRestoresTheTypedTextAndStaysOpen(t *testing.T) {
	history := NewQueryHistory([]string{"alpha", "beta"}, nil)
	h, _ := historyHarness(t, history, "Alpha", "Beta")
	h.typeText("x")
	h.key(0, ui.KeyUp)
	if h.p.Query() != "beta" {
		t.Fatalf("ArrowUp recalled %q, want beta", h.p.Query())
	}
	h.key(0, ui.KeyEscape)
	if got := h.p.Query(); got != "x" || history.Navigating() || !h.win.visible {
		t.Fatalf("Escape during a walk: query %q, navigating %v, visible %v; want x, false, true", got, history.Navigating(), h.win.visible)
	}
	h.key(0, ui.KeyEscape)
	if h.win.visible {
		t.Fatal("a second Escape at the root should hide the palette, with or without text")
	}
}

func TestTypingEndsTheWalkAndTheTypedTextIsTheNewDraft(t *testing.T) {
	history := NewQueryHistory([]string{"alpha", "beta"}, nil)
	h, _ := historyHarness(t, history, "Alpha", "Beta")
	h.key(0, ui.KeyUp)
	if h.p.Query() != "beta" {
		t.Fatalf("ArrowUp recalled %q, want beta", h.p.Query())
	}
	h.typeText("z")
	if history.Navigating() {
		t.Fatal("typing should end the walk")
	}
	before := h.p.Query()
	h.key(0, ui.KeyDown)
	if got := h.p.Query(); got != before {
		t.Fatalf("ArrowDown after typing should not recall: query %q, was %q", got, before)
	}
}

func TestArrowUpMovesTheListWhenARowAboveIsChosen(t *testing.T) {
	history := NewQueryHistory([]string{"alpha"}, nil)
	h, _ := historyHarness(t, history, "Alpha", "Beta")
	h.key(0, ui.KeyDown)
	if h.p.chosen != 1 {
		t.Fatalf("ArrowDown should move the list to row 1, chosen %d", h.p.chosen)
	}
	h.key(0, ui.KeyUp)
	if h.p.Query() != "" || h.p.chosen != 0 || history.Navigating() {
		t.Fatalf("ArrowUp off the top row moves the list, not the history: query %q chosen %d", h.p.Query(), h.p.chosen)
	}
}

func TestEnterAtTheRootRemembersTheQueryAndRecordsTheRow(t *testing.T) {
	history := NewQueryHistory([]string{"alpha"}, nil)
	learner := &recordingLearner{}
	var ran []string
	root := &View{Title: "Nevermind", Search: func(q string) []search.Item {
		return []search.Item{{Title: "Beta", LearnKey: "app:beta", Actions: []search.Action{
			{Title: "Open", Run: func(search.Host) { ran = append(ran, "open") }},
		}}, {Title: "Gamma", Actions: []search.Action{
			{Title: "Open", Run: func(search.Host) { ran = append(ran, "open-gamma") }},
		}}}
	}}
	h := newHarness(t, root, true, func(o *Options) { o.History = history; o.Learner = learner })
	h.typeText("bet")
	h.key(0, ui.KeyEnter)
	if got := history.Entries(); !slices.Equal(got, []string{"alpha", "bet"}) {
		t.Fatalf("entries = %v, want the submitted query last", got)
	}
	if !slices.Equal(learner.calls, []string{"app:beta|bet"}) || !slices.Equal(ran, []string{"open"}) {
		t.Fatalf("learner calls %v, ran %v", learner.calls, ran)
	}
}

// recordingLearner notes each Record call as "key|query" and boosts nothing.
type recordingLearner struct{ calls []string }

func (l *recordingLearner) Boost(string, string) int { return 0 }

func (l *recordingLearner) Record(key, query string) { l.calls = append(l.calls, key+"|"+query) }

func TestSubViewsDoNotWalkTheRootHistory(t *testing.T) {
	history := NewQueryHistory([]string{"alpha"}, nil)
	var ran []string
	h, _ := historyHarness(t, history, "Alpha")
	sub := rowsView("Sub", &ran, "One", "Two")
	h.p.Push(sub)
	h.key(0, ui.KeyUp)
	if h.p.Query() != "" || history.Navigating() {
		t.Fatalf("ArrowUp in a sub-view must not recall the root history: query %q", h.p.Query())
	}
}

func TestAPaletteWithoutHistoryKeepsItsArrowKeys(t *testing.T) {
	var ran []string
	h := newHarness(t, rowsView("Nevermind", &ran, "Alpha", "Beta"), true)
	h.key(0, ui.KeyDown)
	if h.p.chosen != 1 {
		t.Fatalf("without history ArrowDown moves the list, chosen %d", h.p.chosen)
	}
}

func TestShowEndsAWalkLeftOpen(t *testing.T) {
	history := NewQueryHistory([]string{"alpha"}, nil)
	h, _ := historyHarness(t, history, "Alpha")
	h.key(0, ui.KeyUp)
	h.p.Show()
	h.frame()
	if history.Navigating() || h.p.Query() != "" {
		t.Fatalf("summoning the palette should end a walk: navigating %v, query %q", history.Navigating(), h.p.Query())
	}
}

func (l *recordingLearner) LastUsed(string) int64 { return 0 }
