package files

import (
	"errors"
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// TestIndexLabelsFollowTheFooterWords checks the footer's count while the index loads, scans and holds its files.
func TestIndexLabelsFollowTheFooterWords(t *testing.T) {
	cases := map[string]api.IndexStatus{
		"Indexing…":         {},
		"Indexing… 7 files": {Ready: true, Scanning: true, ScannedFiles: 7},
		"5 files indexed":   {Ready: true, Entries: 5},
	}
	for want, status := range cases {
		if got := indexLabel(status); got != want {
			t.Fatalf("indexLabel(%+v) = %q, want %q", status, got, want)
		}
	}
}

// TestSearchFilesFooterShowsTheKindAndTheCount checks the footer names the kind and the index, and that the empty state
// says Indexing… until the first index loads.
func TestSearchFilesFooterShowsTheKindAndTheCount(t *testing.T) {
	files := &fakeFiles{status: api.IndexStatus{Ready: true, Entries: 5}}
	ctx := fakeContext{files: files}
	if got := statusText(ctx, ""); got != "All files · 5 files indexed" {
		t.Fatalf("footer = %q", got)
	}
	if got := statusText(ctx, "image"); got != "Images · 5 files indexed" {
		t.Fatalf("image footer = %q", got)
	}
	if title := indexEmptyState(ctx).Title; title != "No files found." {
		t.Fatalf("empty title = %q once ready", title)
	}
	files.status = api.IndexStatus{}
	if title := indexEmptyState(ctx).Title; title != "Indexing…" {
		t.Fatalf("empty title = %q while loading, want Indexing…", title)
	}
}

// TestSearchFilesViewTabsThroughKindsAndRefreshesItsFooter checks the view's key hint, its Tab kinds, and the refresh that
// keeps the footer and the empty state current while the index fills.
func TestSearchFilesViewTabsThroughKindsAndRefreshesItsFooter(t *testing.T) {
	files := &fakeFiles{status: api.IndexStatus{}}
	ctx := fakeContext{files: files}
	ext := Extension{state: &viewState{}}
	view := ext.searchFilesView(ctx)
	if view.Hint != "Tab kind" || view.Filter == nil || !view.Filter.CycleOnTab {
		t.Fatalf("view = hint %q, accessory %+v; want Tab kind and cycling kinds", view.Hint, view.Filter)
	}
	if view.Refresh == nil || view.Refresh.IntervalMs != statusIntervalMs || view.Refresh.Ref.ID != refStatus {
		t.Fatalf("refresh = %+v, want the status refresh every second", view.Refresh)
	}
	if view.EmptyView == nil || view.EmptyView.Title != "Indexing…" {
		t.Fatalf("empty view = %+v while loading", view.EmptyView)
	}
	files.status = api.IndexStatus{Ready: true, Entries: 9}
	result := ext.statusRefresh(ctx)
	if result.Patch == nil || result.Patch.Status != "All files · 9 files indexed" || result.Patch.EmptyView.Title != "No files found." {
		t.Fatalf("refresh patch = %+v, want the footer and empty state once ready", result.Patch)
	}
}

// TestKindChangeShowsTheKindInTheFooter checks a kind change answers the footer with the kind it chose.
func TestKindChangeShowsTheKindInTheFooter(t *testing.T) {
	files := &fakeFiles{status: api.IndexStatus{Ready: true, Entries: 5}}
	result := Extension{state: &viewState{}}.kindChanged(fakeContext{files: files}, "video")
	if result.Patch == nil || result.Patch.Status != "Videos · 5 files indexed" {
		t.Fatalf("kind patch = %+v, want the Videos footer", result.Patch)
	}
}

// TestFileRowDeclaresPreviewActionsWithoutPlatformBranches checks the row's pane names its file, and that a system
// host chooses which actions the current system can offer.
func TestFileRowDeclaresPreviewActionsWithoutPlatformBranches(t *testing.T) {
	ctx := fakeContext{files: &fakeFiles{}}
	row := fileItem(ctx, api.FileRef{Path: "/h/a.pdf", Name: "a.pdf", DisplayPath: "~/a.pdf", Kind: "file"}, 1)
	if row.Detail == nil || row.Detail.File == nil || row.Detail.File.Path != "/h/a.pdf" {
		t.Fatalf("detail = %+v, want the row's file", row.Detail)
	}
	if titles := menuTitles(row.Actions); !slices.Equal(titles, []string{"Preview Contents", "Quick Look", "Show in Finder"}) {
		t.Fatalf("menu = %v, want declarative previews and reveal", titles)
	}
}

// TestFileRowsLearnByTheirPath checks a file row has a stable learn key, so the files the user opens rank higher.
func TestFileRowsLearnByTheirPath(t *testing.T) {
	ctx := fakeContext{files: &fakeFiles{}}
	file := api.FileRef{Path: "/h/a.pdf", Name: "a.pdf", Kind: "file"}
	first, second := fileItem(ctx, file, 1), fileItem(ctx, file, 9)
	if first.LearnKey != "file:/h/a.pdf" || second.LearnKey != first.LearnKey {
		t.Fatalf("learn keys = %q and %q, want the path's, whatever the score", first.LearnKey, second.LearnKey)
	}
}

// TestMeaningFailureSaysWhy checks a failed decision explains itself in the list, with the reason the backend gave.
func TestMeaningFailureSaysWhy(t *testing.T) {
	row := failureRow(errors.New("not enough credits"))
	if row.Title != "Could not search files by meaning: not enough credits" || !row.Contextual || row.Disabled {
		t.Fatalf("failure row = %+v", row)
	}
}

// TestMeaningAskNeedsASignedInQuery checks the ask row is offered for a typed query of a signed-in account only.
func TestMeaningAskNeedsASignedInQuery(t *testing.T) {
	files := &fakeFiles{}
	ext := Extension{state: &viewState{}}
	if rows := ext.meaningRows(fakeContext{files: files, signed: true}, ""); hasAsk(rows) {
		t.Fatal("an empty query offered the ask row")
	}
	if rows := ext.meaningRows(fakeContext{files: files}, "beach"); hasAsk(rows) {
		t.Fatal("a signed-out account was offered the ask row")
	}
}

func hasAsk(rows []api.Item) bool {
	for _, row := range rows {
		if row.ID == meaningAskID {
			return true
		}
	}
	return false
}
