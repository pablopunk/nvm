package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/search"
)

// countRowsOfKind counts the rows of one kind.
func countRowsOfKind(rows []search.Item, kind string) int {
	n := 0
	for _, it := range rows {
		if it.Kind == kind {
			n++
		}
	}
	return n
}

// TestRootListHidesCopiesWhenItsSettingIsOff pins "Show Clipboard Items in Root List": off hides the copies at the
// empty root, and leaves the root search alone.
func TestRootListHidesCopiesWhenItsSettingIsOff(t *testing.T) {
	p := goldenPalette(t)
	p.S.ClipRootOff = true
	if n := countRowsOfKind(rootResultsForTable(p, ""), search.KindClipboard); n != 0 {
		t.Fatalf("the empty root shows %d copies with the root list off", n)
	}
	if n := countRowsOfKind(rootResultsForTable(p, "meeting"), search.KindClipboard); n == 0 {
		t.Fatal("the root search lost its copies when only the root list was switched off")
	}
}

// TestRootSearchHidesCopiesWhenItsSettingIsOff pins "Show Clipboard Items in Root Search": off hides the copies a
// query finds, and leaves the root list alone.
func TestRootSearchHidesCopiesWhenItsSettingIsOff(t *testing.T) {
	p := goldenPalette(t)
	p.S.ClipRootSearchOff = true
	if n := countRowsOfKind(rootResultsForTable(p, "meeting"), search.KindClipboard); n != 0 {
		t.Fatalf("a root search shows %d copies with the root search off", n)
	}
	if n := countRowsOfKind(rootResultsForTable(p, ""), search.KindClipboard); n == 0 {
		t.Fatal("the root list lost its copies when only the root search was switched off")
	}
}

// TestRootSearchShowsAtMostFiveCopiesFromTwoLetters pins the 1.x search: five copies at most, and a query of two
// letters already finds them.
func TestRootSearchShowsAtMostFiveCopiesFromTwoLetters(t *testing.T) {
	n := countRowsOfKind(rootResultsForTable(goldenPalette(t), "ht"), search.KindClipboard)
	if n == 0 || n > rootSearchClipboardLimit {
		t.Fatalf("a two-letter root search shows %d copies, want 1 to %d", n, rootSearchClipboardLimit)
	}
}

// refreshRecorder is the host a settings toggle runs against in tests: it counts the refreshes the toggle asks for.
type refreshRecorder struct {
	search.Host
	refreshes int
}

// Refresh records that the toggle asked the palette to search again.
func (h *refreshRecorder) Refresh() { h.refreshes++ }

// TestSettingsTogglesShowClipboardInTheRoot runs the two settings rows and checks that each one flips its switch and
// its accessory.
func TestSettingsTogglesShowClipboardInTheRoot(t *testing.T) {
	p := goldenPalette(t)
	for _, title := range []string{"Show Clipboard Items in Root List", "Show Clipboard Items in Root Search"} {
		row := findRowTitled(t, p.settingsPage().Search(""), title)
		if row.Accessory != "On" {
			t.Fatalf("%q starts %s, want On by default", title, row.Accessory)
		}
		host := &refreshRecorder{}
		row.Actions[0].Run(host)
		if host.refreshes != 1 {
			t.Fatalf("%q refreshed the settings %d times, want once", title, host.refreshes)
		}
		if after := findRowTitled(t, p.settingsPage().Search(""), title); after.Accessory != "Off" {
			t.Fatalf("%q shows %s after the toggle, want Off", title, after.Accessory)
		}
	}
	if !p.S.ClipRootOff || !p.S.ClipRootSearchOff {
		t.Fatalf("the switches are root list %v and root search %v, want both off", p.S.ClipRootOff, p.S.ClipRootSearchOff)
	}
}
