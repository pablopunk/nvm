package thumbs

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/real"
)

func TestFixtureReplacementAndLinks(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "image.png")
	fs := real.NewPorts().FS
	write := func(c color.NRGBA) {
		t.Helper()
		img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
		img.SetNRGBA(0, 0, c)
		var b bytes.Buffer
		if err := png.Encode(&b, img); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, b.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Render(fs, p, 64); err == nil {
		t.Fatal("missing image accepted")
	}
	if err := os.WriteFile(p, []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Render(fs, p, 64); err == nil {
		t.Fatal("invalid image accepted")
	}
	write(color.NRGBA{R: 255, A: 255})
	first, err := Render(fs, p, 64)
	if err != nil {
		t.Fatal(err)
	}
	write(color.NRGBA{B: 255, A: 255})
	second, err := Render(fs, p, 64)
	if err != nil || bytes.Equal(first, second) {
		t.Fatalf("replacement: %v", err)
	}
	target := filepath.Join(d, "target.png")
	if err := os.Rename(p, target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, p); err != nil {
		t.Fatal(err)
	}
	if _, err := Render(fs, p, 64); err == nil {
		t.Fatal("link accepted")
	}
	reader := fs.(interface {
		ReadFileNoFollow(string, int64) ([]byte, error)
	})
	if _, err := reader.ReadFileNoFollow(p, MaxSourceBytes); err == nil {
		t.Fatal("swapped link followed")
	}
}
