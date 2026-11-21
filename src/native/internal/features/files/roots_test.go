package files

import (
	"path/filepath"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/testpath"
)

// TestDefaultRootsCoverTheStandardPersonalFolders ports file-index-roots.test.ts.
func TestDefaultRootsCoverTheStandardPersonalFolders(t *testing.T) {
	want := []string{
		filepath.Join(testHome, "Desktop"),
		filepath.Join(testHome, "Documents"),
		filepath.Join(testHome, "Downloads"),
		filepath.Join(testHome, "Pictures"),
		filepath.Join(testHome, "Videos"),
		filepath.Join(testHome, "Movies"),
	}
	got := DefaultRoots(testHome)
	if len(got) != len(want) {
		t.Fatalf("DefaultRoots = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DefaultRoots[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestDisplayPathAbbreviatesOnlyTheHomeFolderAndItsDescendants ports the displayUserPath case of file-utils.test.ts.
func TestDisplayPathAbbreviatesOnlyTheHomeFolderAndItsDescendants(t *testing.T) {
	if got := DisplayPath(testHome, testHome); got != "~" {
		t.Errorf("home itself = %q, want ~", got)
	}
	if got := DisplayPath(testHome, filepath.Join(testHome, "Library", "Cache")); got != filepath.Join("~", "Library", "Cache") {
		t.Errorf("descendant = %q, want ~/Library/Cache in the OS's separator", got)
	}
	if got := DisplayPath(testHome, testHome+"-other"); got != testHome+"-other" {
		t.Errorf("sibling sharing a prefix = %q, want it unchanged", got)
	}
	if got := DisplayPath("", testpath.Abs("srv", "data")); got != testpath.Abs("srv", "data") {
		t.Errorf("empty home = %q, want the path unchanged", got)
	}
}

// TestNormalizeRootsExpandsHomeAndDropsRelativeAndDuplicateRoots covers the root handling of find-files.ts.
func TestNormalizeRootsExpandsHomeAndDropsRelativeAndDuplicateRoots(t *testing.T) {
	got := NormalizeRoots(testHome, []string{"~/Downloads", "relative/dir", "", "~", testpath.Abs("srv", "data"), "~/Downloads"})
	want := []string{filepath.Join(testHome, "Downloads"), testHome, testpath.Abs("srv", "data")}
	if len(got) != len(want) {
		t.Fatalf("NormalizeRoots = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("NormalizeRoots[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestIsInsideRootRequiresASeparatorBoundary checks that a sibling sharing a name prefix is not inside a root.
func TestIsInsideRootRequiresASeparatorBoundary(t *testing.T) {
	root := filepath.Join(testHome, "Docs")
	cases := []struct {
		path string
		want bool
	}{
		{root, true},
		{filepath.Join(root, "a.txt"), true},
		{filepath.Join(root, "deep", "b.txt"), true},
		{root + "s", false},
		{filepath.Dir(root), false},
	}
	for _, c := range cases {
		if got := IsInsideRoot(c.path, root); got != c.want {
			t.Errorf("IsInsideRoot(%q, %q) = %v, want %v", c.path, root, got, c.want)
		}
	}
	if !IsInsideRoot(testpath.Abs("anything"), testpath.Root()) {
		t.Error("the filesystem root contains every absolute path")
	}
}

// TestRootContainingPicksTheLongestRoot makes nested roots resolve to the most specific one.
func TestRootContainingPicksTheLongestRoot(t *testing.T) {
	outer := filepath.Join(testHome, "Work")
	inner := filepath.Join(outer, "Archive")
	roots := []string{outer, inner}
	if got := RootContaining(roots, filepath.Join(inner, "old.zip")); got != inner {
		t.Errorf("nested file root = %q, want %q", got, inner)
	}
	if got := RootContaining(roots, filepath.Join(outer, "new.zip")); got != outer {
		t.Errorf("outer file root = %q, want %q", got, outer)
	}
	if got := RootContaining(roots, testHome); got != "" {
		t.Errorf("path outside every root = %q, want empty", got)
	}
}
