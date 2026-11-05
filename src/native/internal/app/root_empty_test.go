package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/search"
)

// TestRootRowsSortByScoreWithNoSectionsAndAtMostThirty pins the root's layout for the empty query and a typed one: rows in
// 1.x score order, no section labels, and the 30-row cap.
func TestRootRowsSortByScoreWithNoSectionsAndAtMostThirty(t *testing.T) {
	for _, q := range []string{"", "meeting"} {
		rows := rootResultsForTable(goldenPalette(t), q)
		if len(rows) == 0 || len(rows) > rootRowLimit {
			t.Fatalf("the root shows %d rows for %q, want 1 to %d", len(rows), q, rootRowLimit)
		}
		for i, it := range rows {
			if it.Section != "" {
				t.Fatalf("%q row %d %q carries section %q; the root has no section labels", q, i, it.Title, it.Section)
			}
			if i > 0 && rows[i-1].Score < it.Score {
				t.Fatalf("%q row %d %q scores %d above the row before it (%d)", q, i, it.Title, it.Score, rows[i-1].Score)
			}
		}
	}
}

// TestRootEmptyStateSaysTypeAnything pins the root's empty state to 1.x's copy: "Type anything", with no subtitle.
func TestRootEmptyStateSaysTypeAnything(t *testing.T) {
	v := goldenPalette(t).rootView()
	if v.EmptyTitle != "Type anything" || v.EmptySub != "" {
		t.Fatalf("the root's empty state is %q and %q, want \"Type anything\" with no subtitle", v.EmptyTitle, v.EmptySub)
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
