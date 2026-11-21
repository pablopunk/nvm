package real

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStreamSpaceUsesOpenedDirectoryAfterRename(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "dir")
	moved := filepath.Join(base, "moved")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	parent, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	if err := os.Rename(dir, moved); err != nil {
		t.Skipf("platform cannot rename an open directory: %v", err)
	}
	if _, err := streamSpace(parent); err != nil {
		t.Fatalf("opened directory space after rename: %v", err)
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := streamSpace(parent); err != nil {
		t.Fatalf("opened directory space after replacement: %v", err)
	}
}
