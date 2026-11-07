package real

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

func TestReadFileNoFollowWindows(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "image")
	if err := os.WriteFile(file, []byte("image"), 0600); err != nil {
		t.Fatal(err)
	}
	fs := osFS{}
	data, err := fs.ReadFileNoFollow(file, 5)
	if err != nil || string(data) != "image" {
		t.Fatalf("read = %q, %v", data, err)
	}
	if _, err := fs.ReadFileNoFollow(file, 4); !errors.Is(err, platform.ErrFileTooLarge) {
		t.Fatalf("size error = %v", err)
	}
	if _, err := fs.ReadFileNoFollow(dir, 100); err == nil {
		t.Fatal("directory accepted")
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(file, link); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if _, err := fs.ReadFileNoFollow(link, 100); !errors.Is(err, platform.ErrFileSymlink) {
		t.Fatalf("symlink error = %v", err)
	}
}
