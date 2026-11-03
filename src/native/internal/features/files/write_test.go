package files

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// The cases below port write-text-file.test.ts.

// writeFixture returns a fake file system with a Documents folder under testHome.
func writeFixture(t *testing.T) *fake.FS {
	t.Helper()
	fsys := fake.NewFS()
	if err := fsys.MkdirAll(testHome+"/Documents", 0o755); err != nil {
		t.Fatal(err)
	}
	return fsys
}

// readBack returns the content of a file in fsys.
func readBack(t *testing.T, fsys *fake.FS, path string) string {
	t.Helper()
	data, err := fsys.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestWriteTextFileWritesANewFileIntoAnExistingFolder ports "writes a new text file into an existing folder".
func TestWriteTextFileWritesANewFileIntoAnExistingFolder(t *testing.T) {
	fsys := writeFixture(t)
	target := testHome + "/Documents/Trip Plan.md"
	written, err := WriteTextFile(fsys, testHome, target, "# Porto\n", WriteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if written != target {
		t.Fatalf("written path = %q, want %q", written, target)
	}
	if got := readBack(t, fsys, target); got != "# Porto\n" {
		t.Fatalf("content = %q", got)
	}
}

// TestWriteTextFileRefusesToReplaceUnlessOverwriteIsSet ports "refuses to replace an existing file unless overwrite is
// requested".
func TestWriteTextFileRefusesToReplaceUnlessOverwriteIsSet(t *testing.T) {
	fsys := writeFixture(t)
	target := testHome + "/Documents/notes.txt"
	if _, err := WriteTextFile(fsys, testHome, target, "first", WriteOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteTextFile(fsys, testHome, target, "second", WriteOptions{}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("second write err = %v, want an already-exists error", err)
	}
	if got := readBack(t, fsys, target); got != "first" {
		t.Fatalf("content after the refused write = %q", got)
	}
	if _, err := WriteTextFile(fsys, testHome, target, "second", WriteOptions{Overwrite: true}); err != nil {
		t.Fatal(err)
	}
	if got := readBack(t, fsys, target); got != "second" {
		t.Fatalf("content after overwrite = %q", got)
	}
	for _, p := range fsys.Paths() {
		if strings.HasSuffix(p, ".nvm-tmp") {
			t.Fatalf("overwrite left a temporary file: %s", p)
		}
	}
}

// TestWriteTextFileNeverCreatesFoldersOrWritesRelativePaths ports "never creates missing folders or writes relative
// paths".
func TestWriteTextFileNeverCreatesFoldersOrWritesRelativePaths(t *testing.T) {
	fsys := writeFixture(t)
	if _, err := WriteTextFile(fsys, testHome, testHome+"/Documents/missing/a.txt", "x", WriteOptions{}); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("missing folder err = %v", err)
	}
	if _, err := WriteTextFile(fsys, testHome, "relative.txt", "x", WriteOptions{}); err == nil || !strings.Contains(err.Error(), "absolute path") {
		t.Fatalf("relative path err = %v", err)
	}
	if _, err := WriteTextFile(fsys, testHome, "~/Documents/tilde.txt", "x", WriteOptions{}); err != nil {
		t.Fatalf("a path starting with ~ must expand to the home folder: %v", err)
	}
}
