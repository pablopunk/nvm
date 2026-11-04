package palette

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// decideHarness opens the palette on a root page and pushes a host question on top of it, the way the app does.
func decideHarness(t *testing.T) (*harness, *[]bool) {
	t.Helper()
	h := newHarness(t, rowsView("Root", nil, "Alpha"), true)
	var answers []bool
	h.p.Push(Decide("Clear fixture history", "Requested by Desktop fixture. This cannot be undone.", func(confirmed bool) {
		answers = append(answers, confirmed)
	}))
	h.frame()
	return h, &answers
}

func TestDecideConfirmAnswersBeforeThePaletteHides(t *testing.T) {
	h, answers := decideHarness(t)
	h.key(0, ui.KeyEnter)
	if len(*answers) != 1 || !(*answers)[0] {
		t.Fatalf("answers %v, want one confirmation", *answers)
	}
	if h.win.visible {
		t.Fatal("the palette stayed open after the confirmation")
	}
}

func TestDecideCancelRowGoesBackAndRefuses(t *testing.T) {
	h, answers := decideHarness(t)
	h.key(0, ui.KeyDown)
	h.key(0, ui.KeyEnter)
	if len(*answers) != 1 || (*answers)[0] {
		t.Fatalf("answers %v, want one refusal", *answers)
	}
	if len(h.p.stack) != 1 {
		t.Fatalf("the question is still on the stack (%d pages)", len(h.p.stack))
	}
}

func TestDecideEscapeRefusesAtOnceAndLeavesThePage(t *testing.T) {
	h, answers := decideHarness(t)
	h.key(0, ui.KeyEscape)
	if len(*answers) != 1 || (*answers)[0] {
		t.Fatalf("answers %v after Escape, want one refusal at once", *answers)
	}
	if len(h.p.stack) != 1 {
		t.Fatalf("Escape left the question on the stack (%d pages)", len(h.p.stack))
	}
}

func TestDecideHidingTheWindowRefusesOnceAndNeverAgain(t *testing.T) {
	h, answers := decideHarness(t)
	h.p.Hide()
	if len(*answers) != 1 || (*answers)[0] {
		t.Fatalf("answers %v after the palette hid, want one refusal", *answers)
	}
	h.p.Show()
	h.frame()
	h.key(0, ui.KeyEscape)
	if len(*answers) != 1 {
		t.Fatalf("a second answer arrived after the page was left: %v", *answers)
	}
}
