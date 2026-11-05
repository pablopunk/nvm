package services

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/features/files"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// fakeIndex answers the index from a fixed list and records the last query, so a test checks what the service asked.
type fakeIndex struct {
	roots   []string
	results []files.Result
	last    files.Query
	rebuilt int
}

func (f *fakeIndex) Roots() []string { return f.roots }

func (f *fakeIndex) Search(_ context.Context, query files.Query) []files.Result {
	f.last = query
	return f.results
}

func (f *fakeIndex) Rebuild() int {
	f.rebuilt++
	return len(f.results)
}

// unsupportedShell is a shell whose OS has no Quick Look and no open-with choice.
type unsupportedShell struct{ *fake.Shell }

func (unsupportedShell) QuickLook(string) error              { return platform.ErrUnsupported }
func (unsupportedShell) OpenWith(string, platform.App) error { return platform.ErrUnsupported }

func indexHandler(t *testing.T, home string, index *fakeIndex) *Handler {
	t.Helper()
	deps := Deps{Home: home, FileIndex: index, Shell: fake.NewShell()}
	return realFilesHandler(t, deps)
}

func TestRecentReadsTheNewestFilesOfTheIndex(t *testing.T) {
	home := t.TempDir()
	index := &fakeIndex{results: []files.Result{{Path: filepath.Join(home, "a.pdf"), Name: "a.pdf", Kind: files.KindFile}}}
	h := indexHandler(t, home, index)
	var listed []api.IndexedFile
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceDesktopFiles, protocol.OpIndexRecent, protocol.IndexArgs{Options: api.IndexOptions{FindOptions: api.FindOptions{Limit: 6}}})), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].Name != "a.pdf" || listed[0].ID != filepath.Join(home, "a.pdf") {
		t.Fatalf("recent = %+v, want the one indexed file with its path as ID", listed)
	}
	if index.last.SortBy != files.SortRecent || index.last.Limit != 6 || index.last.Text != "" {
		t.Fatalf("recent query = %+v, want newest first with the limit and no text", index.last)
	}
}

func TestSearchIndexPassesTheTextAndKind(t *testing.T) {
	home := t.TempDir()
	index := &fakeIndex{}
	h := indexHandler(t, home, index)
	mustOK(t, serve(t, h, protocol.ServiceDesktopFiles, protocol.OpSearchIndex, protocol.SearchIndexArgs{
		Query: "invoice", Options: api.IndexOptions{FindOptions: api.FindOptions{Kind: "image"}},
	}))
	if index.last.Text != "invoice" || len(index.last.Kinds) != 1 || index.last.Kinds[0] != files.KindImage {
		t.Fatalf("search query = %+v, want the text and the image kind", index.last)
	}
}

func TestIndexSnapshotUsesTheOptionsRootsAndQuery(t *testing.T) {
	home := t.TempDir()
	index := &fakeIndex{}
	h := indexHandler(t, home, index)
	mustOK(t, serve(t, h, protocol.ServiceDesktopFiles, protocol.OpIndexSnapshot, protocol.IndexArgs{
		Options: api.IndexOptions{Roots: []string{"~/Downloads", "relative/path"}, Ignore: []string{"*.tmp"}, Query: "tax"},
	}))
	if index.last.Text != "tax" || len(index.last.Ignore) != 1 || len(index.last.Roots) != 1 || index.last.Roots[0] != filepath.Join(home, "Downloads") {
		t.Fatalf("snapshot query = %+v, want the expanded root only, the ignore and the text", index.last)
	}
}

func TestIndexedRootsAreNormalizedAndAnEmptyIndexAnswersNothing(t *testing.T) {
	home := t.TempDir()
	h := indexHandler(t, home, &fakeIndex{roots: []string{"~/Desktop"}})
	var roots []string
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceDesktopFiles, protocol.OpIndexedRoots, nil)), &roots); err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 || roots[0] != filepath.Join(home, "Desktop") {
		t.Fatalf("roots = %v, want the expanded desktop", roots)
	}
	bare := realFilesHandler(t, Deps{Home: home})
	var none []api.IndexedFile
	if err := jsonInto(mustOK(t, serve(t, bare, protocol.ServiceDesktopFiles, protocol.OpIndexSnapshot, protocol.IndexArgs{})), &none); err != nil || len(none) != 0 {
		t.Fatalf("snapshot without an index = %v, %v, want none", none, err)
	}
}

func TestReindexStartsARescanAndReportsItsRoots(t *testing.T) {
	home := t.TempDir()
	index := &fakeIndex{roots: []string{home}, results: make([]files.Result, 3)}
	h := indexHandler(t, home, index)
	var report api.IndexReport
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceDesktopFiles, protocol.OpReindex, protocol.IndexArgs{})), &report); err != nil {
		t.Fatal(err)
	}
	if index.rebuilt != 1 || report.Count != 3 || len(report.Roots) != 1 {
		t.Fatalf("reindex = %+v after %d rebuilds, want count 3 and the index roots", report, index.rebuilt)
	}
}

func TestReadTextReturnsTheWholeFileAndRefusesLargerOnes(t *testing.T) {
	home := t.TempDir()
	small := filepath.Join(home, "note.md")
	if err := os.WriteFile(small, []byte("hello\nworld"), 0o600); err != nil {
		t.Fatal(err)
	}
	large := filepath.Join(home, "big.txt")
	if err := os.WriteFile(large, []byte(strings.Repeat("x", maxReadTextBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	h := realFilesHandler(t, Deps{Home: home})
	var text string
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceDesktopFiles, protocol.OpReadText, protocol.PathArgs{Path: small})), &text); err != nil || text != "hello\nworld" {
		t.Fatalf("read = %q, %v, want the whole file", text, err)
	}
	if reply := serve(t, h, protocol.ServiceDesktopFiles, protocol.OpReadText, protocol.PathArgs{Path: large}); reply.Failure == nil {
		t.Fatal("a file over the bound was read, want a refusal rather than a cut text")
	}
	if reply := serve(t, h, protocol.ServiceDesktopFiles, protocol.OpReadText, protocol.PathArgs{Path: "relative.txt"}); reply.Failure == nil {
		t.Fatal("a relative path was read, want a refusal")
	}
}

func TestWriteTextCreatesAndReplacesOnlyWhenAsked(t *testing.T) {
	home := t.TempDir()
	target := filepath.Join(home, "out.json")
	h := realFilesHandler(t, Deps{Home: home})
	mustOK(t, serve(t, h, protocol.ServiceDesktopFiles, protocol.OpWriteText, protocol.WriteFileTextArgs{Path: target, Text: "{}"}))
	if reply := serve(t, h, protocol.ServiceDesktopFiles, protocol.OpWriteText, protocol.WriteFileTextArgs{Path: target, Text: "[]"}); reply.Failure == nil {
		t.Fatal("an existing file was replaced without overwrite")
	}
	mustOK(t, serve(t, h, protocol.ServiceDesktopFiles, protocol.OpWriteText, protocol.WriteFileTextArgs{Path: target, Text: "[]", Overwrite: true}))
	if data, _ := os.ReadFile(target); string(data) != "[]" {
		t.Fatalf("file = %q, want the overwritten text", data)
	}
	missing := filepath.Join(home, "no-folder", "x.txt")
	if reply := serve(t, h, protocol.ServiceDesktopFiles, protocol.OpWriteText, protocol.WriteFileTextArgs{Path: missing, Text: "x"}); reply.Failure == nil {
		t.Fatal("a write into a missing folder succeeded; the service never creates folders")
	}
}

func TestDisplayPathShowsTheHomeFolderAsTilde(t *testing.T) {
	home := "/Users/someone"
	h := realFilesHandler(t, Deps{Home: home})
	var display string
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceDesktopFiles, protocol.OpDisplayPath, protocol.PathArgs{Path: home + "/Documents/a.txt"})), &display); err != nil {
		t.Fatal(err)
	}
	if display != "~/Documents/a.txt" {
		t.Fatalf("display = %q, want ~/Documents/a.txt", display)
	}
}

func TestQuickLookAndOpenWithReportUnsupportedSystems(t *testing.T) {
	home := t.TempDir()
	h := realFilesHandler(t, Deps{Home: home, Shell: unsupportedShell{fake.NewShell()}})
	file := filepath.Join(home, "a.png")
	app := "/Applications/Preview.app"
	if reply := serve(t, h, protocol.ServiceDesktopFiles, protocol.OpQuickLook, protocol.PathArgs{Path: file}); reply.Failure == nil {
		t.Fatal("quick look succeeded on a system without it")
	}
	raw, _ := json.Marshal(protocol.OpenWithArgs{Path: file, AppPath: app})
	if reply := serve(t, h, protocol.ServiceDesktopFiles, protocol.OpOpenWith, protocol.OpenWithArgs{Path: file, AppPath: app}); reply.Failure == nil {
		t.Fatalf("open with %s succeeded on a system without it", string(raw))
	}
}
