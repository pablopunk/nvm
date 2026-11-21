package extview

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// guideRowsForDoneQuery are the Getting Started rows the J25 journey filters: the query names the Done row, and the
// Set Shortcut row's text holds its letters scattered.
var guideRowsForDoneQuery = []api.Item{
	{Title: "Open Nevermind from anywhere", Subtitle: "Optional: set a shortcut for instant access. Launching Nevermind from its app icon also opens the palette."},
	{Title: "Sign in to Nevermind", Subtitle: "Optional: connect your account to use AI chats."},
	{Title: "Done for now", Subtitle: "Dismiss this guide and keep using the palette."},
}

// TestListMatchesAreAPhraseOrPrefixAndTheExactTitleLeads: the query the guide's Done row is typed as matches that row
// alone, and Enter on the first row runs it, as 1.x's list views rank their matches.
func TestListMatchesAreAPhraseOrPrefixAndTheExactTitleLeads(t *testing.T) {
	got := matchingItems(guideRowsForDoneQuery, "done for now")
	if len(got) != 1 || got[0].Title != "Done for now" {
		titles := make([]string, 0, len(got))
		for _, item := range got {
			titles = append(titles, item.Title)
		}
		t.Fatalf("matches for %q = %v, want only Done for now", "done for now", titles)
	}
}

// TestAnExactTitleLeadsOverAKeywordAndAPartialMatch: a title match comes first, then a keyword match, then the others in
// their order.
func TestAnExactTitleLeadsOverAKeywordAndAPartialMatch(t *testing.T) {
	items := []api.Item{
		{Title: "Signature block", Subtitle: "Pablo"},
		{Title: "Other", Keywords: []string{"signature"}},
		{Title: "Signature"},
	}
	got := matchingItems(items, "signature")
	if len(got) != 3 || got[0].Title != "Signature" || got[1].Title != "Other" || got[2].Title != "Signature block" {
		t.Fatalf("order = %v, want Signature, Other, Signature block", got)
	}
}

// TestAnEmptyQueryKeepsEveryRow: the empty query shows the rows in their order.
func TestAnEmptyQueryKeepsEveryRow(t *testing.T) {
	if got := matchingItems(guideRowsForDoneQuery, ""); len(got) != len(guideRowsForDoneQuery) {
		t.Fatalf("empty query kept %d rows, want %d", len(got), len(guideRowsForDoneQuery))
	}
}
