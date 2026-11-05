package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/search"
)

// TestBuiltInCommandsStayOutOfTheExtensionBandAtTheRoot checks the empty root's first row: the AI commands score 0 as
// their 1.x actions did, so the welcome row, which scores 90, is the row Enter runs.
func TestBuiltInCommandsStayOutOfTheExtensionBandAtTheRoot(t *testing.T) {
	items := rootResultsForTable(rootTableFixture(t), "")
	if len(items) == 0 || items[0].Title != "Welcome to Nevermind" {
		t.Fatalf("the empty root's first row = %q, want the welcome row that 1.x shows first", firstTitle(items))
	}
}

// TestClipboardHistoryIsFirstForClip checks that a query the built-in commands only share a prefix with leaves Clipboard
// History first, as 1.x ranks it.
func TestClipboardHistoryIsFirstForClip(t *testing.T) {
	items := rootResultsForTable(rootTableFixture(t), "clip")
	if len(items) == 0 || items[0].Title != "Clipboard History" {
		t.Fatalf("the first row for \"clip\" = %q, want Clipboard History", firstTitle(items))
	}
}

// TestTriageDoesNotOutrankClipboardHistoryForCl checks the query that exposed the regression: Triage matches "cl" only by
// its alias "classify selected text", so it must not rank above Clipboard History, which matches it by title.
func TestTriageDoesNotOutrankClipboardHistoryForCl(t *testing.T) {
	items := rootResultsForTable(rootTableFixture(t), "cl")
	triage, clipboard := indexOfItem(items, "Triage Selected Text with AI"), indexOfItem(items, "Clipboard History")
	if clipboard < 0 {
		t.Fatalf("Clipboard History is not listed for \"cl\"")
	}
	if triage >= 0 && triage < clipboard {
		t.Fatalf("Triage (row %d) outranks Clipboard History (row %d) for \"cl\"", triage, clipboard)
	}
}

func firstTitle(items []search.Item) string {
	if len(items) == 0 {
		return ""
	}
	return items[0].Title
}
