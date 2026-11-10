package palette

import (
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/search"
)

// TestActionMenuDrawsEachGroupTitleAboveItsFirstAction checks the menu shows a header row above each titled group, that
// the rows map back to their actions, and that Enter's glyph sits on the default action.
func TestActionMenuDrawsEachGroupTitleAboveItsFirstAction(t *testing.T) {
	acts := []search.Action{
		{Title: "Open"},
		{Title: "Pin", Section: "Manage"},
		{Title: "Unpin", Section: "Manage"},
		{Title: "Clear all", Section: "View", MenuOnly: true},
	}
	rows, rowAction := actionMenuRows(acts, 1, "")

	var titles []string
	for _, row := range rows {
		titles = append(titles, row.Title)
	}
	if !slices.Equal(titles, []string{"Open", "Manage", "Pin", "Unpin", "View", "Clear all"}) {
		t.Fatalf("rows = %v, want a header above each new group", titles)
	}
	if !slices.Equal(rowAction, []int{0, -1, 1, 2, -1, 3}) {
		t.Fatalf("row to action = %v, want headers mapped to -1", rowAction)
	}
	if !rows[0].Enter || rows[len(rows)-1].Enter {
		t.Fatal("Enter glyph should sit on Open, the default action, and not on a View action")
	}
	if !rows[2].Selected {
		t.Fatal("the chosen action Pin should be selected")
	}
}
