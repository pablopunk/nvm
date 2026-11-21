package appcommands

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestQuitIsFoundByItsTitleAndKeywords(t *testing.T) {
	for _, query := range []string{"quit", "exit"} {
		items, err := Extension{}.SearchItems(nil, query)
		if err != nil || len(items) != 1 {
			t.Fatalf("SearchItems(%q) = %+v, %v; want Quit Nevermind", query, items, err)
		}
		if p := items[0].Primary; p == nil || p.Kind != api.ActionSystem || p.Op != QuitOp {
			t.Errorf("primary = %+v, want the quit op", p)
		}
	}
}

func TestRestartIsSearchable(t *testing.T) {
	items, err := (Extension{}).SearchItems(nil, "restart nevermind")
	if err != nil || len(items) != 1 || items[0].Title != RestartTitle || items[0].Primary == nil || items[0].Primary.Op != RestartOp {
		t.Fatalf("restart search = %+v, %v", items, err)
	}
}

func TestRestartDoesNotMatchTerminalQuery(t *testing.T) {
	items, err := (Extension{}).SearchItems(nil, "term")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.Title == RestartTitle {
			t.Fatalf("terminal query listed %q", item.Title)
		}
	}
}

func TestEmptyQueryListsNothing(t *testing.T) {
	if items, _ := (Extension{}).SearchItems(nil, ""); len(items) != 0 {
		t.Fatalf("items = %+v, want none for the empty root", items)
	}
}

// TestRootListsQuitNevermindInRedAtItsOneXScore pins the one app command the empty root shows.
func TestRootListsQuitNevermindInRedAtItsOneXScore(t *testing.T) {
	items, err := Extension{}.RootItems(nil)
	if err != nil {
		t.Fatalf("RootItems: %v", err)
	}
	if len(items) != 1 || items[0].Title != "Quit Nevermind" || items[0].Score != rootQuitScore {
		t.Fatalf("root = %+v, want Quit Nevermind at %d", items, rootQuitScore)
	}
	if items[0].Appearance == nil || items[0].Appearance.Foreground != api.ColorRed {
		t.Fatalf("Quit Nevermind appearance = %+v, want red", items[0].Appearance)
	}
}
