package thumbs

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/real"
)

func TestRenderedIsStoredOncePerEdge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shot.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, 2000, 1000))); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	cache, err := Open(real.NewPorts().FS, filepath.Join(dir, "cache"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Rendered(path, 1, 2, GridEdge); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Thumbnail(path, 1, 2); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	data, err := cache.Rendered(path, 1, 2, GridEdge)
	if err != nil {
		t.Fatalf("the stored picture was rendered again: %v", err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width != GridEdge {
		t.Fatalf("stored width = %d, %v; want %d", config.Width, err, GridEdge)
	}
}
