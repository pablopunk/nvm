package search

import (
	"slices"
	"testing"
)

// TestRootTiesFollowOneXOrder checks the tie order of 1.x's compareRankedActions: at equal score, the row used most
// recently first, builder chats (tie priority -1) after the rest, then the title ascending.
func TestRootTiesFollowOneXOrder(t *testing.T) {
	rows := []Item{
		{Title: "Beta", Score: 50},
		{Title: "alpha", Score: 50},
		{Title: "Recent", Score: 50, LastUsed: 2_000},
		{Title: "Builder", Score: 50, TiePriority: -1, LastUsed: 9_000},
	}
	slices.SortStableFunc(rows, compareRows)
	want := []string{"Recent", "alpha", "Beta", "Builder"}
	for i, row := range rows {
		if row.Title != want[i] {
			t.Fatalf("row %d = %q, want %q", i, row.Title, want[i])
		}
	}
}

// TestTitlesCompareWithoutCaseFirstAndLowercaseWins matches localeCompare's order for the letters of root titles.
func TestTitlesCompareWithoutCaseFirstAndLowercaseWins(t *testing.T) {
	if compareTitles("apple", "Banana") >= 0 {
		t.Fatal("apple must sort before Banana")
	}
	if compareTitles("a", "A") >= 0 {
		t.Fatal("at equal letters the lowercase title must sort first")
	}
}
