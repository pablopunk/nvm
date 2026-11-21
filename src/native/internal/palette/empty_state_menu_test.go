package palette

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/search"
)

// TestEmptyStateActionMenuListsThePagesActions opens the action menu of a page that has no rows, as an empty Snippets
// list does. With no row drawn chosen, the menu lists the page's own actions, such as Add Snippet.
func TestEmptyStateActionMenuListsThePagesActions(t *testing.T) {
	page := &View{
		Title:      "Snippets",
		Search:     func(string) []search.Item { return nil },
		EmptyTitle: "No snippets yet",
		Actions:    func() []search.Action { return []search.Action{{Title: "Add Snippet", Run: func(search.Host) {}}} },
	}
	h := newHarness(t, page, true)
	h.p.SetQuery("")
	h.tt.Frame()
	h.p.OpenActionMenu(0)
	if len(h.p.menuSnapshot) != 1 || h.p.menuSnapshot[0].Title != "Add Snippet" {
		t.Fatalf("menu = %+v, want the page's Add Snippet", h.p.menuSnapshot)
	}
}
