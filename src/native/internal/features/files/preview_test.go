package files

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// The cases below port text-file-preview.test.ts.

// previewFS returns a fake file system holding one file with the given content.
func previewFS(path string, content []byte) *fake.FS {
	fsys := fake.NewFS()
	fsys.Seed(path, content)
	return fsys
}

// TestPreviewReadsSmallTextFilesWhole ports "reads small text files whole".
func TestPreviewReadsSmallTextFilesWhole(t *testing.T) {
	fsys := previewFS("/notes/notes.md", []byte("# Notes\n"))
	got, err := textPreview(fsys, "/notes/notes.md", TextPreviewLimit)
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "# Notes\n" || got.Binary || got.Truncated || got.SizeBytes != 8 {
		t.Fatalf("preview = %+v", got)
	}
}

// TestPreviewBoundsLargeFilesAndDropsASplitTrailingCharacter ports "bounds large files and drops a split trailing
// character": the limit cuts a two-byte character in half, and the half is removed.
func TestPreviewBoundsLargeFilesAndDropsASplitTrailingCharacter(t *testing.T) {
	content := "abc" + strings.Repeat("é", 10)
	fsys := previewFS("/notes/long.txt", []byte(content))
	got, err := textPreview(fsys, "/notes/long.txt", 4)
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "abc" || !got.Truncated || got.SizeBytes != int64(len(content)) {
		t.Fatalf("preview = %+v, want abc, truncated, size %d", got, len(content))
	}
}

// TestPreviewDetectsBinaryFilesAndUnreadablePaths ports "detects binary files and unreadable paths".
func TestPreviewDetectsBinaryFilesAndUnreadablePaths(t *testing.T) {
	fsys := previewFS("/tmp/blob.bin", []byte{1, 0, 2})
	got, err := textPreview(fsys, "/tmp/blob.bin", TextPreviewLimit)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Binary || got.SizeBytes != 3 || got.Text != "" {
		t.Fatalf("binary preview = %+v", got)
	}
	if _, _, _, err := TextPreview(fsys, "/nonexistent/nvm-preview.txt"); err == nil {
		t.Fatal("a missing file must report a read error")
	}
}

// TestPreviewLooksForBinaryOnlyInItsSniffWindow checks that a NUL far past the sniff window does not make text binary.
func TestPreviewLooksForBinaryOnlyInItsSniffWindow(t *testing.T) {
	content := strings.Repeat("a", binarySniffBytes+10) + "\x00"
	fsys := previewFS("/tmp/late.txt", []byte(content))
	got, err := textPreview(fsys, "/tmp/late.txt", TextPreviewLimit)
	if err != nil {
		t.Fatal(err)
	}
	if got.Binary {
		t.Fatal("a NUL after the sniff window must not mark the file binary")
	}
}

// TestPreviewFencesTextLongerThanAnyBacktickRun ports "fences text with a fence longer than any backtick run".
func TestPreviewFencesTextLongerThanAnyBacktickRun(t *testing.T) {
	got := MarkdownPreview("/tmp/a.md", Preview{Text: "use ```js``` here", SizeBytes: 17})
	if got != "````md\nuse ```js``` here\n````" {
		t.Fatalf("markdown = %q", got)
	}
}

// TestPreviewNotesTruncationAndBinaryContent ports "notes truncation and binary content in markdown".
func TestPreviewNotesTruncationAndBinaryContent(t *testing.T) {
	truncated := MarkdownPreview("/tmp/a.txt", Preview{Text: "abc", Truncated: true, SizeBytes: 204_800})
	if !strings.Contains(truncated, "Showing the first 0 KB of 200 KB") {
		t.Fatalf("truncation note missing: %q", truncated)
	}
	binary := MarkdownPreview("/tmp/a.bin", Preview{Binary: true, SizeBytes: 2 * 1024})
	if !strings.Contains(binary, "Binary file (2 KB)") {
		t.Fatalf("binary note missing: %q", binary)
	}
}

// TestPreviewReportsUnreadableFilesInMarkdown covers the unreadable note, which the TypeScript path also produces.
func TestPreviewReportsUnreadableFilesInMarkdown(t *testing.T) {
	fsys := fake.NewFS()
	preview, err := TextPreviewDetail(fsys, "/missing.txt")
	if err == nil {
		t.Fatal("expected a read error")
	}
	if got := MarkdownPreview("/missing.txt", preview); !strings.HasPrefix(got, "_Could not read this file:") {
		t.Fatalf("unreadable note = %q", got)
	}
}
