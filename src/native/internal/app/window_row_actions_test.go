package app

import "testing"

// TestWindowCommandRowsOfferShortcutAndAliasActions checks the Command-K menu of a window layout row: as the 1.x action
// does, it offers Set keyboard shortcut and Set alias after its own action.
func TestWindowCommandRowsOfferShortcutAndAliasActions(t *testing.T) {
	p := goldenPalette(t)
	p.attachWindows()
	items := rootResultsForTable(p, "left half")
	row := items[indexOfItem(items, "Left Half")]
	titles := make([]string, 0, len(row.Actions))
	for _, action := range row.Actions {
		titles = append(titles, action.Title)
	}
	if !containsTitle(titles, "Set keyboard shortcut") || !containsTitle(titles, "Set alias") {
		t.Fatalf("the Left Half menu = %q, want Set keyboard shortcut and Set alias", titles)
	}
}
