package clipboard

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// Cases ported from clipboard-history.test.ts for the list and row builders, plus the link candidacy the
// 1.x detail builder applied. The 1.x removed-message case is TestRemovedMessage in history_test.go.

// fixedTime is the copy time of test entries; the subtitles and orders are then deterministic.
var fixedTime = time.Date(2026, time.January, 1, 9, 0, 0, 0, time.UTC)

// TestViewReturnsRowsForEntries ports 'clipboardHistoryView returns view with items'.
func TestViewReturnsRowsForEntries(t *testing.T) {
	entry := Entry{ID: "text:a", Kind: KindText, Text: "hello", CreatedAt: fixedTime}
	view := BuildView([]Entry{entry}, 1)
	if view.Title != "Clipboard History" || view.ID != "clipboard-history" || view.Presentation != "side-preview" {
		t.Fatalf("view = %+v", view)
	}
	if len(view.Rows) != 1 || view.Rows[0].Detail.Text != "hello" {
		t.Fatalf("rows = %+v", view.Rows)
	}
}

// TestViewShowsEmptyStateWithoutHistory ports 'clipboardHistoryView shows empty state when no history'.
func TestViewShowsEmptyStateWithoutHistory(t *testing.T) {
	view := BuildView(nil, 0)
	if len(view.Rows) != 0 || view.EmptyTitle != "No clipboard items found." {
		t.Fatalf("view = %+v", view)
	}
	if len(view.Actions) != 0 {
		t.Fatalf("removal actions without history: %+v", view.Actions)
	}
}

// TestHistoryViewRemovalActions covers the history-wide removal actions: recent entries and everything.
func TestHistoryViewRemovalActions(t *testing.T) {
	view := BuildView([]Entry{{ID: "text:a", Kind: KindText, Text: "a"}}, 1)
	want := []string{"Remove Entries from Last Hour", "Remove Entries from Last Day", "Remove All Entries"}
	if got := actionTitles(view.Actions); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("titles = %v", got)
	}
}

// TestTextRowBuildsPasteAction ports 'clipboardHistoryItem builds text item with paste action'.
func TestTextRowBuildsPasteAction(t *testing.T) {
	entry := Entry{ID: "text:abc", Kind: KindText, Text: "hello world", CreatedAt: fixedTime}
	row := BuildRow(entry, 1)
	if row.ID != "clipboard:text:abc" || row.Title != "hello world" {
		t.Fatalf("row = %+v", row)
	}
	if !hasAction(row.Actions, ActionPaste) {
		t.Fatal("text row has no paste action")
	}
	if row.Primary.Type != ActionCopy {
		t.Fatalf("primary = %s, want copy", row.Primary.Type)
	}
	if row.Manage[0].Range != RangeItem || row.Manage[1].Title != "Remove Last..." {
		t.Fatalf("manage = %+v", row.Manage)
	}
	var ranges []string
	for _, action := range row.Manage[1].Submenu.Actions {
		ranges = append(ranges, action.Title+"="+string(action.Range))
		if action.Type != ActionRemove || !action.Confirm || action.ConfirmLabel != "Remove" {
			t.Fatalf("submenu action %+v is not a confirmed removal", action)
		}
	}
	want := "5 Minutes=last-5-minutes,Hour=last-hour,Day=last-day,Week=last-week"
	if got := strings.Join(ranges, ","); got != want {
		t.Fatalf("submenu = %s, want %s", got, want)
	}
}

// TestImageRowHasNoPasteAction ports 'clipboardHistoryItem builds image item without paste action'.
func TestImageRowHasNoPasteAction(t *testing.T) {
	row := BuildRow(Entry{ID: "image:abc", Kind: KindImage, ImageRef: "abc.png"}, 1)
	if hasAction(row.Actions, ActionPaste) {
		t.Fatal("image row offers a paste action")
	}
}

// TestViewKeeps300RowsWithinActionBudget ports 'clipboardHistoryView keeps 300 rows within the action token
// budget': the rows share one removal submenu, so the number of distinct actions stays small.
func TestViewKeeps300RowsWithinActionBudget(t *testing.T) {
	entries := make([]Entry, 0, 300)
	for i := range 300 {
		entries = append(entries, Entry{ID: fmt.Sprintf("text:%d", i), Kind: KindText, Text: "value", CreatedAt: fixedTime.Add(-time.Duration(i))})
	}
	view := BuildView(entries, len(entries))
	seen := map[*Submenu]bool{}
	count := len(view.Actions)
	for _, row := range view.Rows {
		count += 1 + len(row.Actions) + len(row.Manage)
		for _, action := range row.Manage {
			if action.Submenu != nil && !seen[action.Submenu] {
				seen[action.Submenu] = true
				count += len(action.Submenu.Actions)
			}
		}
	}
	if count >= 2000 {
		t.Fatalf("distinct actions = %d, want under 2000", count)
	}
	for _, row := range view.Rows {
		if len(row.Manage) != 2 {
			t.Fatalf("row %s has %d removal actions", row.ID, len(row.Manage))
		}
		if row.Manage[1].Submenu != view.Rows[0].Manage[1].Submenu {
			t.Fatal("rows do not share the removal submenu")
		}
	}
}

// TestLongTitleIsShortened keeps the first 72 characters of a long text, with an ellipsis.
func TestLongTitleIsShortened(t *testing.T) {
	long := strings.Repeat("é", 100)
	title := entryTitle(Entry{Kind: KindText, Text: long})
	if got := []rune(title); len(got) != 73 || got[72] != '…' {
		t.Fatalf("title has %d runes, want 72 and an ellipsis", len(got))
	}
}

// TestDetailOfSingleWebAddressCarriesLink: a row whose text is exactly one public address is a link candidate.
func TestDetailOfSingleWebAddressCarriesLink(t *testing.T) {
	row := BuildRow(Entry{ID: "text:l", Kind: KindText, Text: "https://github.com/a/b"}, 1)
	if row.Detail.LinkURL != "https://github.com/a/b" {
		t.Fatalf("LinkURL = %q", row.Detail.LinkURL)
	}
	row = BuildRow(Entry{ID: "text:m", Kind: KindText, Text: "see https://github.com"}, 1)
	if row.Detail.LinkURL != "" {
		t.Fatalf("LinkURL = %q for text with words", row.Detail.LinkURL)
	}
}

// hasAction reports whether an action list contains the type.
func hasAction(actions []Action, kind ActionType) bool {
	for _, action := range actions {
		if action.Type == kind {
			return true
		}
	}
	return false
}

// actionTitles returns the titles of an action list.
func actionTitles(actions []Action) []string {
	out := make([]string, 0, len(actions))
	for _, action := range actions {
		out = append(out, action.Title)
	}
	return out
}
