package app

import (
	"testing"

	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/search"
)

// TestRootEmptyShowsCheckForUpdatesAtItsOneXScore pins the Check for Updates row of the empty root: 1.x's score of 23,
// with the current version as its subtitle.
func TestRootEmptyShowsCheckForUpdatesAtItsOneXScore(t *testing.T) {
	row := findRowTitled(t, rootResultsForTable(goldenPalette(t), ""), checkForUpdatesTitle)
	if row.Score != 23 || row.Subtitle != "Current version: "+mygo.App.Version() {
		t.Fatalf("Check for Updates = score %d subtitle %q, want 23 with the current version", row.Score, row.Subtitle)
	}
}

// TestUpdatePromptReplacesTheCheckForUpdatesRow pins 1.x's rule: while an update prompt shows, the root's check row is
// not listed, and it is listed again once the prompt goes.
func TestUpdatePromptReplacesTheCheckForUpdatesRow(t *testing.T) {
	p := goldenPalette(t)
	for _, row := range commandRows(p, &rootInput{updatePrompt: true}, "") {
		if row.Title == checkForUpdatesTitle {
			t.Fatal("the check row shows beside the update prompt")
		}
	}
	findRowTitled(t, commandRows(p, &rootInput{}, ""), checkForUpdatesTitle)
}

// TestUpdatePromptScoresAboveEveryRootRow pins the update prompt to 1.x's score of 1000.
func TestUpdatePromptScoresAboveEveryRootRow(t *testing.T) {
	if rootUpdateScore != 1000 {
		t.Fatalf("the update prompt scores %d, want 1.x's 1000", rootUpdateScore)
	}
}

// TestRootEmptyListsOneAccountRowAtItsOneXScore pins the account row of the empty root: the one Log in or Log out row
// the session needs, at 1.x's score of 18, and no second copy of it.
func TestRootEmptyListsOneAccountRowAtItsOneXScore(t *testing.T) {
	var account []search.Item
	for _, row := range rootResultsForTable(goldenPalette(t), "") {
		if row.Title == "Log in to Nevermind" || row.Title == "Log out of Nevermind" {
			account = append(account, row)
		}
	}
	if len(account) != 1 || account[0].Score != 18 {
		t.Fatalf("the root lists %d account rows, want one at 18: %+v", len(account), account)
	}
}
