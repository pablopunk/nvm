package real

import (
	"context"
	"errors"
	"github.com/pablopunk/nvm/native/internal/platform"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestPictureStagingUsesOpenedSourceAfterPathReplacement(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "movie.mp4")
	if err := os.WriteFile(original, []byte("authorized video"), 0o600); err != nil {
		t.Fatal(err)
	}
	source, err := (osFS{}).OpenReadNoFollow(original)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	if err := os.Rename(original, filepath.Join(dir, "previous.mp4")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(original, []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	staged, cleanup, err := stagePictureSource(context.Background(), source, original)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	data, err := os.ReadFile(staged)
	if err != nil || string(data) != "authorized video" {
		t.Fatalf("staged %q, %v", data, err)
	}
	if filepath.Ext(staged) != ".mp4" {
		t.Fatal("OS provider lost the file format")
	}
	cleanup()
	if _, err := os.Stat(staged); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staged source survived cleanup: %v", err)
	}
}

func TestPictureStagingCancelsAndRefusesSymlinks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.gif")
	if err := os.WriteFile(path, []byte("image"), 0o600); err != nil {
		t.Fatal(err)
	}
	source, err := (osFS{}).OpenReadNoFollow(path)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	staged, cleanup, err := stagePictureSource(ctx, source, path)
	if !errors.Is(err, context.Canceled) || staged != "" || cleanup != nil {
		t.Fatalf("cancelled staging returned %q, %v", staged, err)
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	link := path + ".link"
	if err := os.Symlink(path, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if opened, err := (osFS{}).OpenReadNoFollow(link); !errors.Is(err, platform.ErrFileSymlink) {
		if opened != nil {
			_ = opened.Close()
		}
		t.Fatalf("symlink open = %v", err)
	}
}
