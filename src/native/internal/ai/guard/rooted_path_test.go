package guard

import (
	"path/filepath"
	"testing"
)

// TestRootedPathsResolveToAbsolutePaths: on Windows \etc\hosts has no volume and is not absolute, so it must not be
// joined under the working folder, where a relative reading would place it inside the allowed area.
func TestRootedPathsResolveToAbsolutePaths(t *testing.T) {
	rooted := string(filepath.Separator) + "etc" + string(filepath.Separator) + "hosts"
	got := lexicalPath(rooted, t.TempDir(), "/home/u")
	if !filepath.IsAbs(got) {
		t.Fatalf("lexicalPath(%q) = %q, which is not absolute", rooted, got)
	}
}

func TestRelativePathsStillJoinTheWorkingFolder(t *testing.T) {
	cwd := filepath.Join(t.TempDir(), "work")
	got := lexicalPath(filepath.Join("docs", "notes.txt"), cwd, "/home/u")
	if want := filepath.Join(cwd, "docs", "notes.txt"); got != want {
		t.Fatalf("lexicalPath = %q, want %q", got, want)
	}
}
