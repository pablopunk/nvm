package thumbs

import (
	"bytes"
	"fmt"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScreenshotsProbe(t *testing.T) {
	dir := t.TempDir()
	img := image.NewNRGBA(image.Rect(0, 0, 1920, 1080))
	var x uint32 = 7
	for y := 0; y < 1080; y++ {
		for z := 0; z < 1920; z++ {
			x = x*1664525 + 1013904223
			img.SetNRGBA(z, y, color.NRGBA{uint8(x >> 24), uint8(x >> 16), uint8(x >> 8), 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "shot.png")
	if err := os.WriteFile(path, b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	fs := real.NewPorts().FS
	start := time.Now()
	total := 0
	for i := 0; i < 80; i++ {
		p, e := Render(fs, path, 64)
		if e != nil {
			t.Fatal(e)
		}
		total += len(p)
	}
	fmt.Printf("80 renders elapsed=%s source_bytes=%d png_bytes=%d estimated_base64_bytes=%d\n", time.Since(start), b.Len(), total, total*4/3)
}
