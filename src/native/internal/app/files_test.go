package app

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/features/files"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

// testFileHome is the home folder of the file tests; no test reads the real one.
const testFileHome = "/home/me"

// fixturePaths are the files the file tests search.
func fixturePaths() []string {
	return []string{
		filepath.Join(testFileHome, "Documents", "report-2024.pdf"),
		filepath.Join(testFileHome, "Documents", "reports.txt"),
		filepath.Join(testFileHome, "Pictures", "report-cover.png"),
		filepath.Join(testFileHome, "Movies", "report-demo.mov"),
		filepath.Join(testFileHome, "Desktop", "notes.md"),
	}
}

// titlesOf returns the titles of rows, in order.
func titlesOf(items []search.Item) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Title)
	}
	return out
}

// TestRootFilesStreamTheBestFour checks the root's files source: two characters are needed, the best four rows come
// back, and they carry the files kind and descending scores.
func TestRootFilesStreamTheBestFour(t *testing.T) {
	svc := newTestFiles(t, testFileHome, fixturePaths())
	src := fileSearch{svc: svc, limit: fileRootLimit, minChars: fileRootMinChars, id: "files"}

	var last search.Batch
	var batches int
	src.Search(context.Background(), "re", func(b search.Batch) {
		batches++
		last = b
	})
	if !last.Final {
		t.Fatal("the last batch of a search must be final")
	}
	if len(last.Items) != 4 {
		t.Fatalf("root rows = %d (%v), want the best four", len(last.Items), titlesOf(last.Items))
	}
	for i, it := range last.Items {
		if it.Kind != search.KindFiles {
			t.Fatalf("row %d kind = %q, want Files", i, it.Kind)
		}
		if i > 0 && it.Score >= last.Items[i-1].Score {
			t.Fatalf("scores must fall down the list: %d after %d", it.Score, last.Items[i-1].Score)
		}
	}
}

// TestRootFilesWaitForTwoCharacters checks that one character answers with no rows and a final batch.
func TestRootFilesWaitForTwoCharacters(t *testing.T) {
	svc := newTestFiles(t, testFileHome, fixturePaths())
	src := fileSearch{svc: svc, limit: fileRootLimit, minChars: fileRootMinChars, id: "files"}
	var got []search.Batch
	src.Search(context.Background(), "r", func(b search.Batch) { got = append(got, b) })
	if len(got) != 1 || !got[0].Final || len(got[0].Items) != 0 {
		t.Fatalf("one character must give one empty final batch, got %+v", got)
	}
}

// TestFilesViewKindFilterAndRecentRoot checks the Search Files view's rows: the empty query lists every file, and the
// images filter keeps only pictures.
func TestFilesViewKindFilterAndRecentRoot(t *testing.T) {
	svc := newTestFiles(t, testFileHome, fixturePaths())
	all := fileSearch{svc: svc, limit: fileViewLimit, kinds: nil, recentEmpty: true, id: "files-view"}
	if got := len(all.Root(context.Background())); got != len(fixturePaths()) {
		t.Fatalf("empty query lists %d files, want %d", got, len(fixturePaths()))
	}
	images := fileSearch{svc: svc, limit: fileViewLimit, kinds: fileKindFilters[1].kinds, recentEmpty: true, id: "files-view"}
	rows := images.Root(context.Background())
	if len(rows) != 1 || rows[0].Title != "report-cover.png" {
		t.Fatalf("images filter rows = %v, want report-cover.png", titlesOf(rows))
	}
}

// blockingShell answers AppIcon only when released, so a test can prove the view does not wait for it.
type blockingShell struct {
	*fake.Shell
	asked   chan string
	release chan struct{}
}

// AppIcon records the request and waits for release.
func (b blockingShell) AppIcon(path string, size int) ([]byte, error) {
	b.asked <- path
	<-b.release
	return b.Shell.AppIcon(path, size)
}

var _ platform.Shell = blockingShell{}

// TestIconLoadNeverBlocksTheView checks that a row's picture is returned at once while the operating system is still
// answering the icon request, and that the request is made off the calling goroutine.
func TestIconLoadNeverBlocksTheView(t *testing.T) {
	svc := newTestFiles(t, testFileHome, fixturePaths())
	shell := blockingShell{Shell: fake.NewShell(), asked: make(chan string, 4), release: make(chan struct{})}
	svc.shell = shell
	defer close(shell.release)

	returned := make(chan *ui.Bitmap, 1)
	go func() { returned <- svc.rowPicture(fixturePaths()[4]) }()
	select {
	case bm := <-returned:
		if bm != nil {
			t.Fatal("a picture that is still loading must be nil, not a guess")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("rowPicture blocked on the system icon request")
	}
	select {
	case path := <-shell.asked:
		if path != fixturePaths()[4] {
			t.Fatalf("icon requested for %q", path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the icon was never requested off the UI thread")
	}
}

// TestSystemIconKeysShareByExtension checks the cache keys: files of one extension share an icon, while bundles and
// desktop entries keep their own.
func TestSystemIconKeysShareByExtension(t *testing.T) {
	if systemIconKey("/a/x.png") != systemIconKey("/b/y.png") {
		t.Fatal("files with one extension must share an icon key")
	}
	if systemIconKey("/Applications/Notes.app") == systemIconKey("/Applications/Mail.app") {
		t.Fatal("application bundles must keep their own icon keys")
	}
}

// pictureBytes encodes a small picture for the preview tests.
func pictureBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 16, 8))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestPreviewsOfEachKind checks the preview pane's content for text, binary, a picture and a video.
func TestPreviewsOfEachKind(t *testing.T) {
	svc := newTestFiles(t, testFileHome, nil)
	fsys := svc.fsys.(*fake.FS)
	seed := func(name string, data []byte) files.Result {
		path := filepath.Join(testFileHome, name)
		fsys.Seed(path, data)
		info, err := svc.fsys.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		return files.Result{Path: path, Name: name, Kind: files.KindOf(name), Size: info.Size(), Modified: info.ModTime()}
	}

	text := svc.loadPreview(seed("todo.txt", []byte("buy milk")))
	if text.text != "buy milk" || text.image != nil {
		t.Fatalf("text preview = %+v", text)
	}
	binary := svc.loadPreview(seed("data.bin", []byte{0x7f, 0x00, 0x01}))
	if binary.text != "" || binary.note != "Binary file (0 KB) — no text preview." {
		t.Fatalf("binary preview note = %q", binary.note)
	}
	picture := svc.loadPreview(seed("cover.png", pictureBytes(t)))
	if picture.image == nil {
		t.Fatalf("image preview has no picture: %+v", picture)
	}
	video := svc.loadPreview(seed("clip.mov", []byte("not decoded")))
	if video.image != nil || video.text != "" || video.note == "" {
		t.Fatalf("video preview must be metadata only, got %+v", video)
	}
}

// TestFileActionsDriveTheDesktop checks Open, Show in Folder, Copy Path and Trash against the fake shell and clipboard.
// Trash must ask first, and cancelling must not touch the file.
func TestFileActionsDriveTheDesktop(t *testing.T) {
	svc := newTestFiles(t, testFileHome, fixturePaths())
	fakeShell := svc.shell.(*fake.Shell)
	clip := svc.clipboard.(*fake.Clipboard)
	target := fixturePaths()[0]
	row := svc.fileItem(files.Result{Path: target, Name: "report-2024.pdf", DisplayPath: "~/Documents/report-2024.pdf"})
	if row.Title != "report-2024.pdf" || row.Subtitle != "~/Documents" {
		t.Fatalf("row = %q / %q", row.Title, row.Subtitle)
	}

	open := &recordingHost{}
	row.Actions[0].Run(open)
	reveal := &recordingHost{}
	row.Actions[1].Run(reveal)
	copyPath := &recordingHost{}
	row.Actions[3].Run(copyPath)
	calls := fakeShell.Calls()
	if len(calls) != 2 || calls[0] != (fake.ShellCall{Op: fake.OpShellOpenPath, Arg: target}) ||
		calls[1] != (fake.ShellCall{Op: fake.OpShellReveal, Arg: target}) {
		t.Fatalf("shell calls = %+v", calls)
	}
	if got, _ := clip.ReadText(); got != target {
		t.Fatalf("clipboard = %q, want the path", got)
	}

	trash := &recordingHost{}
	row.Actions[5].Run(trash)
	if len(trash.pushed) != 1 {
		t.Fatal("Move to Trash must open a confirmation view, not act at once")
	}
	if len(fakeShell.Calls()) != 2 {
		t.Fatal("nothing may be trashed before the confirmation")
	}
}

// TestIndexStatusLabelsTheFilesFooter checks the footer words for a ready index.
func TestIndexStatusLabelsTheFilesFooter(t *testing.T) {
	svc := newTestFiles(t, testFileHome, fixturePaths())
	if got := svc.statusLabel(); got != "5 files indexed" {
		t.Fatalf("status = %q", got)
	}
	if got := svc.emptyTitle(); got != "No files found." {
		t.Fatalf("empty title = %q", got)
	}
}
