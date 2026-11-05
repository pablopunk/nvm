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
