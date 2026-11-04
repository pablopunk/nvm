package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/features/apps"
	"github.com/pablopunk/nvm/native/internal/search"
)

// TestRootEmptyListsEveryAppNewestFirst pins the empty root's app order to 1.x: every app, by when it was first seen,
// newest first, then by name.
func TestRootEmptyListsEveryAppNewestFirst(t *testing.T) {
	installed := []apps.App{
		{Name: "Zed", FirstSeenAt: 1},
		{Name: "Beta", FirstSeenAt: 3},
		{Name: "Alpha", FirstSeenAt: 3},
	}
	list, scores := matchingApps(installed, "")
	want := []string{"Alpha", "Beta", "Zed"}
	for i, a := range list {
		if a.Name != want[i] || scores[i] != scoreRootApplication {
			t.Fatalf("row %d = %q at %d, want %q at %d", i, a.Name, scores[i], want[i], scoreRootApplication)
		}
	}
	if len(installed) != 3 || installed[0].Name != "Zed" {
		t.Fatal("ordering the root's apps changed the installed list the palette keeps")
	}
}

// TestRootEmptyRowsSortByScoreWithNoSectionsAndAtMostThirty pins the empty root's layout: rows in 1.x score order, no
// section labels, and the 30-row cap.
func TestRootEmptyRowsSortByScoreWithNoSectionsAndAtMostThirty(t *testing.T) {
	rows := rootResultsForTable(goldenPalette(t), "")
	if len(rows) == 0 || len(rows) > rootRowLimit {
		t.Fatalf("the empty root shows %d rows, want 1 to %d", len(rows), rootRowLimit)
	}
	for i, it := range rows {
		if it.Section != "" {
			t.Fatalf("row %d %q carries section %q; the empty root has no section labels", i, it.Title, it.Section)
		}
		if i > 0 && rows[i-1].Score < it.Score {
			t.Fatalf("row %d %q scores %d above the row before it (%d)", i, it.Title, it.Score, rows[i-1].Score)
		}
	}
}

// TestRootEmptyShowsTenCopiesAndSixRecentFiles pins how many copies and recent files the empty root lists inline.
func TestRootEmptyShowsTenCopiesAndSixRecentFiles(t *testing.T) {
	p := goldenPalette(t)
	counts := map[string]int{}
	for _, it := range rootResultsForTable(p, "") {
		counts[it.Kind]++
	}
	if want := min(rootClipboardLimit, p.clip.history.Len()); counts[search.KindClipboard] != want {
		t.Fatalf("the empty root shows %d copies, want %d", counts[search.KindClipboard], want)
	}
	if counts[search.KindFiles] > fileRootLimit {
		t.Fatalf("the empty root shows %d recent files, want at most %d", counts[search.KindFiles], fileRootLimit)
	}
}
