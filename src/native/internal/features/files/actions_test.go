package files

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/testpath"
)

// TestTrashRefusesRootsHomeAndRelativePaths checks the guards in front of the Trash action: a stray selection of the
// file system root or the home folder must never reach the shell.
func TestTrashRefusesRootsHomeAndRelativePaths(t *testing.T) {
	shell := fake.NewShell()
	for _, path := range []string{testpath.Root(), testHome} {
		if err := Trash(shell, testHome, path); !errors.Is(err, ErrNotAFile) {
			t.Errorf("Trash(%q) err = %v, want ErrNotAFile", path, err)
		}
	}
	if err := Trash(shell, testHome, "notes/relative.txt"); err == nil {
		t.Error("a relative path must be refused")
	}
	if calls := shell.Calls(); len(calls) != 0 {
		t.Fatalf("refused trash requests reached the shell: %v", calls)
	}
}

// TestTrashPassesOrdinaryPathsToTheShell checks that a file inside home is trashed through the shell, cleaned.
func TestTrashPassesOrdinaryPathsToTheShell(t *testing.T) {
	shell := fake.NewShell()
	if err := Trash(shell, testHome, filepath.Join(testHome, "Downloads/old//a.zip")); err != nil {
		t.Fatal(err)
	}
	calls := shell.Calls()
	if len(calls) != 1 || calls[0].Arg != filepath.Join(testHome, "Downloads/old/a.zip") {
		t.Fatalf("shell calls = %+v", calls)
	}
}

// TestRevealAndOpenRequireAbsolutePaths checks the absolute-path rule of the other two actions.
func TestRevealAndOpenRequireAbsolutePaths(t *testing.T) {
	shell := fake.NewShell()
	if err := Reveal(shell, "relative.txt"); err == nil {
		t.Error("Reveal must refuse a relative path")
	}
	if err := OpenPath(shell, "relative.txt"); err == nil {
		t.Error("OpenPath must refuse a relative path")
	}
	if err := Reveal(shell, filepath.Join(testHome, "a.txt")); err != nil {
		t.Fatal(err)
	}
	if err := OpenPath(shell, filepath.Join(testHome, "a.txt")); err != nil {
		t.Fatal(err)
	}
	calls := shell.Calls()
	if len(calls) != 2 || calls[0].Op != "Reveal" || calls[1].Op != "OpenPath" {
		t.Fatalf("shell calls = %+v", calls)
	}
}

// TestTrashSurfacesShellFailures checks that a failed trash is reported, not swallowed.
func TestTrashSurfacesShellFailures(t *testing.T) {
	shell := fake.NewShell()
	boom := errors.New("trash is full")
	shell.FailNext("Trash", boom)
	if err := Trash(shell, testHome, filepath.Join(testHome, "a.zip")); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the shell's error", err)
	}
}
