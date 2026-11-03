package files

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// testScan walks one root of fsys with the given settings and returns the file paths it found.
func testScan(t *testing.T, fsys *fake.FS, root string, cfg scanConfig) (scanResult, []string) {
	t.Helper()
	if cfg.maxEntries == 0 {
		cfg.maxEntries = 100_000
	}
	cfg.statFiles = true
	cfg.workers = 3
	var counters scanCounters
	result, err := scanTrees(context.Background(), fsys, cfg, []scanStart{{dir: root, remaining: cfg.maxDepth}}, &counters)
	if err != nil {
		t.Fatalf("scanTrees: %v", err)
	}
	paths := make([]string, 0, len(result.records))
	for _, rec := range result.records {
		paths = append(paths, filepath.Join(rec.dir, rec.name))
	}
	slices.Sort(paths)
	return result, paths
}

// TestScanWalksWithinDepthAndSkipsHiddenAndIgnoredNames covers the depth limit, hidden names and the default ignores.
func TestScanWalksWithinDepthAndSkipsHiddenAndIgnoredNames(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys,
		"/r/a.txt", "/r/.hidden", "/r/node_modules/x.js", "/r/d1/b.txt",
		"/r/d1/d2/c.txt", "/r/d1/d2/d3/deep.txt", "/r/.git/config",
	)
	_, got := testScan(t, fsys, "/r", scanConfig{maxDepth: 2, rules: CompileIgnoreRules(nil, true)})
	want := []string{"/r/a.txt", "/r/d1/b.txt", "/r/d1/d2/c.txt"}
	if !slices.Equal(got, want) {
		t.Fatalf("scan = %v, want %v", got, want)
	}
}

// TestScanIncludesHiddenNamesWhenAsked checks the include-hidden setting.
func TestScanIncludesHiddenNamesWhenAsked(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys, "/r/.env", "/r/visible.txt")
	_, got := testScan(t, fsys, "/r", scanConfig{maxDepth: 2, includeHidden: true, rules: CompileIgnoreRules(nil, true)})
	if !slices.Equal(got, []string{"/r/.env", "/r/visible.txt"}) {
		t.Fatalf("scan = %v", got)
	}
}

// TestScanNeverFollowsOrIndexesSymbolicLinks makes a link back to its own root: the walk must end and the link must
// not be indexed.
func TestScanNeverFollowsOrIndexesSymbolicLinks(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys, "/r/f.txt", "/r/sub/g.txt")
	if err := fsys.Symlink("/r", "/r/loop"); err != nil {
		t.Fatal(err)
	}
	if err := fsys.Symlink("/outside", "/r/sub/out"); err != nil {
		t.Fatal(err)
	}
	_, got := testScan(t, fsys, "/r", scanConfig{maxDepth: 5, rules: CompileIgnoreRules(nil, true)})
	if !slices.Equal(got, []string{"/r/f.txt", "/r/sub/g.txt"}) {
		t.Fatalf("scan = %v, want only the regular files", got)
	}
}

// TestScanReportsAFolderThatBecomesUnreadableAndKeepsTheRest injects a permission error on one folder mid-walk.
func TestScanReportsAFolderThatBecomesUnreadableAndKeepsTheRest(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys, "/r/locked/secret.txt", "/r/open/ok.txt", "/r/top.txt")
	fsys.FailPath(fake.OpReadDir, "/r/locked", fs.ErrPermission)
	result, got := testScan(t, fsys, "/r", scanConfig{maxDepth: 2, rules: CompileIgnoreRules(nil, true)})
	if !slices.Equal(result.unreadable, []string{"/r/locked"}) {
		t.Fatalf("unreadable = %v, want the locked folder", result.unreadable)
	}
	if !slices.Equal(got, []string{"/r/open/ok.txt", "/r/top.txt"}) {
		t.Fatalf("scan = %v, want the readable files", got)
	}
}

// TestScanStopsAtTheEntryCap checks that the cap is exact and reported as truncation.
func TestScanStopsAtTheEntryCap(t *testing.T) {
	fsys := fake.NewFS()
	for i := range 50 {
		fsys.Seed(fmt.Sprintf("/r/f%02d.txt", i), []byte("x"))
	}
	result, got := testScan(t, fsys, "/r", scanConfig{maxDepth: 1, maxEntries: 10, rules: CompileIgnoreRules(nil, true)})
	if !result.truncated || len(got) != 10 {
		t.Fatalf("truncated=%v with %d files, want truncated with exactly 10", result.truncated, len(got))
	}
}

// TestScanHandlesTenThousandFilesInOneFolder is the wide-folder case: one folder, many files, all found once.
func TestScanHandlesTenThousandFilesInOneFolder(t *testing.T) {
	fsys := fake.NewFS()
	for i := range 10_000 {
		fsys.Seed(fmt.Sprintf("/big/file-%05d.txt", i), []byte("x"))
	}
	_, got := testScan(t, fsys, "/big", scanConfig{maxDepth: 2, rules: CompileIgnoreRules(nil, true)})
	if len(got) != 10_000 {
		t.Fatalf("found %d files, want 10000", len(got))
	}
	if len(slices.Compact(slices.Clone(got))) != 10_000 {
		t.Fatal("a file was reported twice")
	}
}

// TestScanOfAVeryDeepTreeIsBoundedByDepth builds a 300-level chain with a file at every level and checks that only the
// first few levels are read.
func TestScanOfAVeryDeepTreeIsBoundedByDepth(t *testing.T) {
	fsys := fake.NewFS()
	fsys.Seed("/deep/root.txt", []byte("x"))
	var chain strings.Builder
	chain.WriteString("/deep/")
	for i := range 300 {
		fmt.Fprintf(&chain, "%d/", i)
		fsys.Seed(chain.String()+"f.txt", []byte("x"))
	}
	_, got := testScan(t, fsys, "/deep", scanConfig{maxDepth: 2, rules: CompileIgnoreRules(nil, true)})
	// Depth 2 reads the root and two folder levels below it: three levels of files, and nothing deeper.
	if len(got) != 3 {
		t.Fatalf("found %d files in a 300-level chain with depth 2, want 3: %v", len(got), got)
	}
}

// TestScanReportsNothingWhenCancelledAndLeavesNoWorkers checks that a cancelled walk returns the context error and no
// records, and that its workers have all returned by then.
func TestScanReportsNothingWhenCancelledAndLeavesNoWorkers(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys, "/r/a.txt", "/r/b/c.txt")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var counters scanCounters
	cfg := scanConfig{maxDepth: 2, maxEntries: 100, rules: CompileIgnoreRules(nil, true), statFiles: true, workers: 3}
	result, err := scanTrees(ctx, fsys, cfg, []scanStart{{dir: "/r", remaining: 2}}, &counters)
	if err == nil || len(result.records) != 0 {
		t.Fatalf("cancelled scan = %d records, err %v; want an error and no records", len(result.records), err)
	}
	if live := safego.Watching().Live("files.scan.worker"); live != 0 {
		t.Fatalf("%d scan workers still running after the scan returned", live)
	}
}

// TestScanCountsFoldersAndFiles checks the progress counters Status reports.
func TestScanCountsFoldersAndFiles(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys, "/r/a.txt", "/r/b/c.txt", "/r/b/d.txt")
	var counters scanCounters
	cfg := scanConfig{maxDepth: 2, maxEntries: 100, rules: CompileIgnoreRules(nil, true), statFiles: true, workers: 2}
	if _, err := scanTrees(context.Background(), fsys, cfg, []scanStart{{dir: "/r", remaining: 2}}, &counters); err != nil {
		t.Fatal(err)
	}
	if files, dirs := counters.files.Load(), counters.dirs.Load(); files != 3 || dirs != 2 {
		t.Fatalf("counters = %d files, %d folders; want 3 and 2", files, dirs)
	}
}
