package real

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSourceDirectoryRemainsPinnedWhenItsPathIsReplaced(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "extension")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	source, err := (osFS{}).OpenSourceDirectory(root, "extension")
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	if err := os.Rename(dir, filepath.Join(root, "previous")); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("Windows can pin the directory against renaming: %v", err)
		}
		t.Fatal(err)
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	data, err := source.ReadFile("main.go", 100)
	if err != nil || string(data) != "original" {
		t.Fatalf("pinned read changed directory: %q, %v", data, err)
	}
	if _, err := source.ReadFile("../extension/main.go", 100); err == nil {
		t.Fatal("accepted traversal")
	}
	if _, err := source.ReadFile("main.go", 3); err == nil {
		t.Fatal("accepted oversized source")
	}
}

func TestSourceDirectoryRejectsObservedLinksAndExcessEntries(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "extension")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "private.go")
	if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "main.go")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	source, err := (osFS{}).OpenSourceDirectory(root, "extension")
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	if data, err := source.ReadFile("main.go", 100); err == nil || data != nil {
		t.Fatal("read through a symbolic link")
	}
	if _, err := source.Entries(0); err == nil {
		t.Fatal("ignored entry limit")
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := source.ReadFile("main.go", 100); err == nil {
		t.Fatal("closed source remained usable")
	}
}

func TestSourceDirectoryReadContract(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "extension")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	source, err := (osFS{}).OpenSourceDirectory(root, "extension")
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	if entries, err := source.Entries(1); err != nil || len(entries) != 1 {
		t.Fatalf("entries=%v, error=%v", entries, err)
	}
	if data, err := source.ReadFile("main.go", 6); err != nil || string(data) != "source" {
		t.Fatalf("read=%q, error=%v", data, err)
	}
	if _, err := source.ReadFile("main.go", 5); err == nil {
		t.Fatal("read exceeded byte limit")
	}
	for _, name := range []string{"../main.go", "/main.go", `C:main.go`, "."} {
		if _, err := source.ReadFile(name, 100); err == nil {
			t.Errorf("accepted %q", name)
		}
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := source.ReadFile("main.go", 100); err == nil {
		t.Fatal("read succeeded after close")
	}
}
