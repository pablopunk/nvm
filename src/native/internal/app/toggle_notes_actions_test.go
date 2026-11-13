package app

import "testing"

// TestEveryRootCommandRowOffersShortcutAndAliasActions checks the default: a built-in command that sets nothing about
// customization, such as Toggle Floating Notes, still offers Set keyboard shortcut and Set alias.
func TestEveryRootCommandRowOffersShortcutAndAliasActions(t *testing.T) {
	p := goldenPalette(t)
	items := rootResultsForTable(p, "toggle floating notes")
	index := indexOfItem(items, "Toggle Floating Notes")
	if index < 0 {
		t.Fatalf("no Toggle Floating Notes row in %d results", len(items))
	}
	titles := make([]string, 0, len(items[index].Actions))
	for _, action := range items[index].Actions {
		titles = append(titles, action.Title)
	}
	if !containsTitle(titles, "Set keyboard shortcut") || !containsTitle(titles, "Set alias") {
		t.Fatalf("the Toggle Floating Notes menu = %q, want Set keyboard shortcut and Set alias", titles)
	}
}
