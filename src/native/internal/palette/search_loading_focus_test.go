package palette

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/search"
)

func TestSearchLoadingChangesPreserveInputAndSelection(t *testing.T) {
	loading := false
	root := &View{
		Title:   "Root",
		Loading: func() bool { return loading },
		Search: func(string) []search.Item {
			return []search.Item{{ID: "settings", Title: "Settings"}}
		},
	}
	h := newHarness(t, root, true)
	for _, letter := range "settings" {
		loading = !loading
		h.typeText(string(letter))
		if !h.tt.Focused("Search") {
			t.Fatalf("loading=%v stranded search focus after %q", loading, letter)
		}
	}
	if h.p.Query() != "settings" {
		t.Fatalf("loading changes lost typed characters: %q", h.p.Query())
	}
	h.key(ui.Cmd, ui.KeyA)
	loading = !loading
	h.frame()
	h.typeText("account")
	if h.p.Query() != "account" {
		t.Fatalf("loading change lost the search selection: %q", h.p.Query())
	}
}

func TestSearchInputDoesNotMoveWhenLoadingChanges(t *testing.T) {
	loading := false
	root := &View{
		Title:   "Root",
		Loading: func() bool { return loading },
		Search: func(string) []search.Item {
			return []search.Item{{ID: "settings", Title: "Settings"}}
		},
	}
	h := newHarness(t, root, true)
	h.typeText("set")
	before, ok := h.tt.Find("Search")
	if !ok {
		t.Fatal("search input not found")
	}
	loading = true
	h.frame()
	during, ok := h.tt.Find("Search")
	if !ok || during != before {
		t.Fatalf("the input moved when loading started: before=%v during=%v", before, during)
	}
}
