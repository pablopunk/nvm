package palette

import (
	"reflect"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

// appBeforeSummon is the window that had the keyboard before the palette was summoned.
var appBeforeSummon = platform.FocusTarget{Window: "41943045", PID: 812}

// withFocus gives the palette a focus port.
func withFocus(f platform.Focus) func(*Options) {
	return func(o *Options) { o.Focus = f }
}

// launchingView has one row whose action dismisses the palette, as an action that opens an app does.
func launchingView(ran *[]string) *View {
	return &View{Title: "Root", Search: func(string) []search.Item {
		return []search.Item{{Kind: "Test", Title: "Launch", Actions: []search.Action{{
			Title: "Launch", Dismiss: search.DismissAlways,
			Run: func(search.Host) { *ran = append(*ran, "Launch") },
		}}}}
	}}
}

// TestEscapeAtRootGivesTheKeyboardBack checks the Escape hide: the window that had the keyboard before the summon
// gets it back.
func TestEscapeAtRootGivesTheKeyboardBack(t *testing.T) {
	focus := fake.NewFocus()
	focus.SetActive(appBeforeSummon)
	var ran []string
	h := newHarness(t, rowsView("Root", &ran, "Alpha"), true, withFocus(focus))
	h.key(0, ui.KeyEscape)
	if got := focus.Restored(); !reflect.DeepEqual(got, []platform.FocusTarget{appBeforeSummon}) {
		t.Fatalf("restored %v, want the window that had the keyboard before the summon", got)
	}
}

// TestToggleGivesTheKeyboardBack checks the summon press that hides the palette.
func TestToggleGivesTheKeyboardBack(t *testing.T) {
	focus := fake.NewFocus()
	focus.SetActive(appBeforeSummon)
	var ran []string
	h := newHarness(t, rowsView("Root", &ran, "Alpha"), true, withFocus(focus))
	h.p.Toggle()
	if got := focus.Restored(); !reflect.DeepEqual(got, []platform.FocusTarget{appBeforeSummon}) {
		t.Fatalf("restored %v after the toggle", got)
	}
}

// TestSummonBringsAVisibleUnfocusedPaletteForward checks the summon press on a palette another window took the keyboard
// from: it stays shown, captures the window that has the keyboard now, and the next press hides it and gives it back.
func TestSummonBringsAVisibleUnfocusedPaletteForward(t *testing.T) {
	focus := fake.NewFocus()
	focus.SetActive(appBeforeSummon)
	var ran []string
	h := newHarness(t, rowsView("Root", &ran, "Alpha"), true, withFocus(focus))
	otherApp := platform.FocusTarget{Window: "7", PID: 99}
	h.win.takeKeyboard()
	focus.SetActive(otherApp)

	h.p.Toggle()
	if !h.win.IsVisible() || h.win.hides != 0 {
		t.Fatalf("the summon press hid a visible palette the keyboard had left: visible=%v hides=%d", h.win.IsVisible(), h.win.hides)
	}
	if got, ok := h.p.Summoned(); !ok || got != otherApp {
		t.Fatalf("summoned %v (%v), want the window that has the keyboard now", got, ok)
	}

	h.p.Toggle()
	if h.win.IsVisible() {
		t.Fatal("the next summon press should hide the focused palette")
	}
	if got := focus.Restored(); !reflect.DeepEqual(got, []platform.FocusTarget{otherApp}) {
		t.Fatalf("restored %v, want the window captured by the last show", got)
	}
}

// TestClickAwayLeavesTheKeyboardWithTheWindowClicked checks a focus loss that hides the palette after the grace:
// the user picked another window, so the palette does not pull the keyboard back to the one from before the summon.
func TestClickAwayLeavesTheKeyboardWithTheWindowClicked(t *testing.T) {
	focus := fake.NewFocus()
	focus.SetActive(appBeforeSummon)
	var ran []string
	h := newHarness(t, rowsView("Root", &ran, "Alpha"), true, withFocus(focus))
	blurOnce(h)
	passGrace(t, h)
	if h.win.IsVisible() {
		t.Fatal("the palette is still shown after a click away")
	}
	if got := focus.Restored(); len(got) != 0 {
		t.Fatalf("restored %v after a click away; the clicked window keeps the keyboard", got)
	}
}

// TestActionThatDismissesKeepsTheKeyboardWhereItPutsIt checks that an action which opens an app is not undone by
// the palette sending the keyboard back.
func TestActionThatDismissesKeepsTheKeyboardWhereItPutsIt(t *testing.T) {
	focus := fake.NewFocus()
	focus.SetActive(appBeforeSummon)
	var ran []string
	h := newHarness(t, launchingView(&ran), true, withFocus(focus))
	h.key(0, ui.KeyEnter)
	if !reflect.DeepEqual(ran, []string{"Launch"}) {
		t.Fatalf("the action ran %v, want once", ran)
	}
	if got := focus.Restored(); len(got) != 0 {
		t.Fatalf("restored %v after an action that dismisses; the action owns the keyboard", got)
	}
}

// TestNothingIsRestoredWhenNoWindowHadTheKeyboard checks a summon from the desktop: there is no window to return to.
func TestNothingIsRestoredWhenNoWindowHadTheKeyboard(t *testing.T) {
	focus := fake.NewFocus()
	var ran []string
	h := newHarness(t, rowsView("Root", &ran, "Alpha"), true, withFocus(focus))
	h.key(0, ui.KeyEscape)
	if got := focus.Restored(); len(got) != 0 {
		t.Fatalf("restored %v with no window captured", got)
	}
}
