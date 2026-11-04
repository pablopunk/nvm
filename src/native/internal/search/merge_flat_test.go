package search

import "testing"

// TestFlatMergeKeepsScoreOrderWithoutSections pins the empty root's layout: rows sort by score, then by title as 1.x
// does, and carry no section labels, where a typed query is grouped by kind under a top hit.
func TestFlatMergeKeepsScoreOrderWithoutSections(t *testing.T) {
	batches := [][]Item{
		{{Kind: KindCommands, Title: "Settings", Score: 14}},
		{{Kind: KindApplications, Title: "Safari", Score: 55}, {Kind: KindApplications, Title: "Mail", Score: 55}},
		{{Kind: KindClipboard, Title: "copied", Score: 60}},
	}
	flat := mergeAppended(batches, nil, 30, false, true)
	wantTitles := []string{"copied", "Mail", "Safari", "Settings"}
	for i, it := range flat {
		if it.Title != wantTitles[i] || it.Section != "" {
			t.Fatalf("row %d = %q in section %q, want %q with no section", i, it.Title, it.Section, wantTitles[i])
		}
	}

	sectioned := mergeAppended(batches, nil, 30, false, false)
	if sectioned[0].Title != "Settings" || sectioned[0].Section == "" {
		t.Fatalf("a sectioned empty query starts with %q in %q, want the commands section", sectioned[0].Title, sectioned[0].Section)
	}
}

// TestFlatMergeStillCutsToTheLimit keeps the root's row cap in force when the rows are not sectioned.
func TestFlatMergeStillCutsToTheLimit(t *testing.T) {
	var batch []Item
	for i := range 40 {
		batch = append(batch, Item{Kind: KindApplications, Title: "App", Score: 55 - i%2})
	}
	if got := len(mergeAppended([][]Item{batch}, nil, 30, false, true)); got != 30 {
		t.Fatalf("flat merge kept %d rows, want 30", got)
	}
}
