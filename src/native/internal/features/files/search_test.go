package files

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// snapshotOf builds a snapshot from records, the way a scan does.
func snapshotOf(records ...record) *Snapshot {
	b := newBuilder(len(records))
	for _, rec := range records {
		b.addRecord(rec)
	}
	return b.build(1, "")
}

// searchEnvFor returns the environment of a test index with the default roots.
func searchEnvFor() searchEnv {
	return searchEnv{home: testHome, roots: DefaultRoots(testHome)}
}

// finalResults runs a search to its end and returns the final batch's results.
func finalResults(t *testing.T, snap *Snapshot, q Query) []Result {
	t.Helper()
	var final Batch
	seen := false
	for batch := range searchSnapshot(context.Background(), snap, q, searchEnvFor()) {
		final = batch
		seen = true
	}
	if !seen || !final.Final {
		t.Fatal("a search that was not cancelled must end with a final batch")
	}
	return final.Results
}

// TestRankingBandsOrderResults checks that name matches come back in band order and that ties fall to the sort key.
func TestRankingBandsOrderResults(t *testing.T) {
	dir := filepath.Join(testHome, "Documents")
	snap := snapshotOf(
		record{dir: dir, name: "bread.txt"},
		record{dir: dir, name: "readme.md"},
		record{dir: dir, name: "the read notes.txt"},
		record{dir: dir, name: "read.txt"},
		record{dir: dir, name: "rdr.txt"},
	)
	got := namesOf(finalResults(t, snap, Query{Text: "read", SortBy: SortName, Order: OrderAsc}))
	want := []string{"read.txt", "the read notes.txt", "readme.md", "bread.txt"}
	if !slices.Equal(got, want) {
		t.Fatalf("results = %v, want %v", got, want)
	}
}

// TestSearchStreamsPartialBatchesBeforeTheFinalOne checks that a large scan yields improving batches and ends with one
// final batch.
func TestSearchStreamsPartialBatchesBeforeTheFinalOne(t *testing.T) {
	dir := filepath.Join(testHome, "Projects")
	var records []record
	for i := range 10_000 {
		records = append(records, record{dir: dir, name: fmt.Sprintf("file-%05d.txt", i), size: int64(i)})
	}
	snap := snapshotOf(records...)
	partial, finals := 0, 0
	// Sizes grow with the index, so every chunk brings bigger files than the last and each one improves the best results.
	for batch := range searchSnapshot(context.Background(), snap, Query{Text: "file", SortBy: SortSize}, searchEnvFor()) {
		if batch.Final {
			finals++
			if len(batch.Results) != DefaultLimit {
				t.Fatalf("final batch has %d results, want %d", len(batch.Results), DefaultLimit)
			}
			continue
		}
		if finals > 0 {
			t.Fatal("a partial batch arrived after the final one")
		}
		partial++
	}
	if partial < 2 || finals != 1 {
		t.Fatalf("partial batches = %d, final batches = %d; want at least 2 partial and exactly 1 final", partial, finals)
	}
}

// TestSearchFiltersByExtensionKindRootPatternAndIgnore runs each filter the extension API offers.
func TestSearchFiltersByExtensionKindRootPatternAndIgnore(t *testing.T) {
	docs := filepath.Join(testHome, "Documents")
	snap := snapshotOf(
		record{dir: docs, name: "a.pdf"},
		record{dir: docs, name: "e.PNG"},
		record{dir: filepath.Join(testHome, "Pictures"), name: "b.png"},
		record{dir: filepath.Join(testHome, "Movies"), name: "c.mp4"},
		record{dir: filepath.Join(testHome, "Downloads", "node_modules"), name: "d.js"},
	)
	cases := []struct {
		name  string
		query Query
		want  []string
	}{
		{"image kind", Query{Kinds: []Kind{KindImage}, SortBy: SortName, Order: OrderAsc}, []string{"b.png", "e.PNG"}},
		{"media kind", Query{Kinds: []Kind{KindMedia}, SortBy: SortName, Order: OrderAsc}, []string{"b.png", "c.mp4", "e.PNG"}},
		{"extension without dot, any case", Query{Extensions: []string{"PDF"}}, []string{"a.pdf"}},
		{"pattern names its extension", Query{Patterns: []string{"*.png"}, SortBy: SortName, Order: OrderAsc}, []string{"b.png", "e.PNG"}},
		{"root restricts results", Query{Roots: []string{docs}, SortBy: SortName, Order: OrderAsc}, []string{"a.pdf", "e.PNG"}},
		{"query ignore excludes a folder", Query{Ignore: []string{"node_modules"}, SortBy: SortName, Order: OrderAsc}, []string{"a.pdf", "b.png", "c.mp4", "e.PNG"}},
		{"file kind adds no restriction", Query{Kinds: []Kind{KindFile}, SortBy: SortName, Order: OrderAsc}, []string{"a.pdf", "b.png", "c.mp4", "d.js", "e.PNG"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := namesOf(finalResults(t, snap, c.query))
			if !slices.Equal(got, c.want) {
				t.Fatalf("results = %v, want %v", got, c.want)
			}
		})
	}
}

// TestQueryLimitDefaultsAndCap checks the default limit, an explicit limit and the cap.
func TestQueryLimitDefaultsAndCap(t *testing.T) {
	dir := filepath.Join(testHome, "Documents")
	var records []record
	for i := range 150 {
		records = append(records, record{dir: dir, name: fmt.Sprintf("n%03d.txt", i)})
	}
	snap := snapshotOf(records...)
	if got := len(finalResults(t, snap, Query{})); got != DefaultLimit {
		t.Errorf("default limit returned %d, want %d", got, DefaultLimit)
	}
	if got := len(finalResults(t, snap, Query{Limit: 500})); got != 150 {
		t.Errorf("limit 500 returned %d, want all 150", got)
	}
	if got := effectiveLimit(MaxLimit + 1); got != MaxLimit {
		t.Errorf("limit above the cap = %d, want %d", got, MaxLimit)
	}
}

// TestCancelledSearchEndsWithoutAFinalBatch cancels after the first batch and checks the sequence stops without one.
func TestCancelledSearchEndsWithoutAFinalBatch(t *testing.T) {
	dir := filepath.Join(testHome, "Projects")
	var records []record
	for i := range 20_000 {
		records = append(records, record{dir: dir, name: fmt.Sprintf("file-%05d.txt", i)})
	}
	snap := snapshotOf(records...)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	batches, finals := 0, 0
	for batch := range searchSnapshot(ctx, snap, Query{Text: "file"}, searchEnvFor()) {
		batches++
		if batch.Final {
			finals++
		}
		cancel()
	}
	if batches != 1 || finals != 0 {
		t.Fatalf("cancelled search delivered %d batches with %d final; want 1 partial and no final", batches, finals)
	}
}

// TestSortKeysOrderInBothDirections checks size, recency and name sorts, ascending and descending.
func TestSortKeysOrderInBothDirections(t *testing.T) {
	dir := filepath.Join(testHome, "Downloads")
	snap := snapshotOf(
		record{dir: dir, name: "b.bin", size: 300, modMs: 10, createdMs: 10},
		record{dir: dir, name: "a.bin", size: 100, modMs: 30, createdMs: 20},
		record{dir: dir, name: "c.bin", size: 200, modMs: 20, createdMs: 30},
	)
	cases := []struct {
		name  string
		query Query
		want  []string
	}{
		{"size descending", Query{SortBy: SortSize}, []string{"b.bin", "c.bin", "a.bin"}},
		{"size ascending", Query{SortBy: SortSize, Order: OrderAsc}, []string{"a.bin", "c.bin", "b.bin"}},
		{"recent descending", Query{SortBy: SortRecent}, []string{"a.bin", "c.bin", "b.bin"}},
		{"modified ascending", Query{SortBy: SortModified, Order: OrderAsc}, []string{"b.bin", "c.bin", "a.bin"}},
		{"name ascending", Query{SortBy: SortName, Order: OrderAsc}, []string{"a.bin", "b.bin", "c.bin"}},
		{"name descending", Query{SortBy: SortName}, []string{"c.bin", "b.bin", "a.bin"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := namesOf(finalResults(t, snap, c.query)); !slices.Equal(got, c.want) {
				t.Fatalf("order = %v, want %v", got, c.want)
			}
		})
	}
}

// TestSelectionSortsGloballyBeforeSlicingToTheLimit ports the first case of file-index-sorting.test.ts: the limit
// applies after sorting, so the newest files win whatever order they were scanned in.
func TestSelectionSortsGloballyBeforeSlicingToTheLimit(t *testing.T) {
	dir := filepath.Join(testHome, "Downloads")
	snap := snapshotOf(
		record{dir: dir, name: "older-first.png", createdMs: 10},
		record{dir: dir, name: "older-second.png", createdMs: 20},
		record{dir: dir, name: "newest-third.png", createdMs: 300},
		record{dir: dir, name: "newer-fourth.png", createdMs: 200},
	)
	got := namesOf(finalResults(t, snap, Query{SortBy: SortAdded, Limit: 2}))
	if !slices.Equal(got, []string{"newest-third.png", "newer-fourth.png"}) {
		t.Fatalf("selection = %v", got)
	}
}

// TestAddedSortFallsBackToTheCreationTime adapts the second case of file-index-sorting.test.ts. The Go model keeps one
// creation time per file, which stands in for the "date added" the TypeScript index reads when it is known, so the
// added order is the creation order.
func TestAddedSortFallsBackToTheCreationTime(t *testing.T) {
	dir := filepath.Join(testHome, "Downloads")
	snap := snapshotOf(
		record{dir: dir, name: "missing-date.png", createdMs: 150},
		record{dir: dir, name: "explicit-date.png", createdMs: 250},
	)
	got := finalResults(t, snap, Query{SortBy: SortAdded})
	if len(got) != 2 || got[0].Name != "explicit-date.png" || got[0].Created.UnixMilli() != 250 || got[1].Created.UnixMilli() != 150 {
		t.Fatalf("added order = %+v", got)
	}
}

// TestSortNeedsStatOnlyForMetadataSorts ports the stat-requirement case of file-index-sorting.test.ts. The
// includeDimensions half of that case has no Go counterpart: dimensions are not read by this package.
func TestSortNeedsStatOnlyForMetadataSorts(t *testing.T) {
	for _, sortBy := range []SortBy{SortAdded, SortCreated, SortRecent, SortModified, SortSize} {
		if !sortNeedsStat(sortBy) {
			t.Errorf("sort %q must read metadata", sortBy)
		}
	}
	if sortNeedsStat(SortName) || sortNeedsStat(SortNone) {
		t.Error("name and unsorted queries must not read metadata")
	}
}

// TestSearchShowsDisplayPathsAbbreviatedToHome checks the path a palette row shows.
func TestSearchShowsDisplayPathsAbbreviatedToHome(t *testing.T) {
	snap := snapshotOf(record{dir: filepath.Join(testHome, "Documents"), name: "plan.md"})
	got := finalResults(t, snap, Query{Text: "plan"})
	if len(got) != 1 || got[0].DisplayPath != "~/Documents/plan.md" || got[0].Path != filepath.Join(testHome, "Documents", "plan.md") {
		t.Fatalf("result = %+v", got)
	}
}

// TestFindWalksRootsAndReportsMissingOnes checks the one-shot find: missing roots are reported, kinds and sorts apply.
func TestFindWalksRootsAndReportsMissingOnes(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys, testHome+"/Downloads/a.png", testHome+"/Downloads/b.txt", testHome+"/Documents/c.png")
	opts := FindOptions{
		Home:  testHome,
		Query: Query{Kinds: []Kind{KindImage}, SortBy: SortName, Order: OrderAsc},
	}
	result, err := Find(context.Background(), fsys, []string{"~/Downloads", "~/Gone", "~/Documents"}, opts)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got := namesOf(result.Results); !slices.Equal(got, []string{"a.png", "c.png"}) {
		t.Fatalf("results = %v", got)
	}
	if !slices.Equal(result.MissingRoots, []string{testHome + "/Gone"}) {
		t.Fatalf("missing roots = %v", result.MissingRoots)
	}
}

// TestFindRespectsTheScanLimit checks that the walk stops at the scan limit before sorting, as findFiles does.
func TestFindRespectsTheScanLimit(t *testing.T) {
	fsys := fake.NewFS()
	for i := range 40 {
		fsys.Seed(fmt.Sprintf("%s/Downloads/f%02d.txt", testHome, i), []byte("x"))
	}
	result, err := Find(context.Background(), fsys, []string{testHome + "/Downloads"}, FindOptions{Home: testHome, ScanLimit: 5})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(result.Results) != 5 {
		t.Fatalf("scan limit 5 returned %d files", len(result.Results))
	}
}
