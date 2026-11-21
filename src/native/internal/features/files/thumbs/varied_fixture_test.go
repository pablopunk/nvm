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

func TestVariedFixtureBudget(t *testing.T) {
	d := t.TempDir()
	paths := make([]string, 80)
	var x uint32 = 7
	for i := range paths {
		img := image.NewNRGBA(image.Rect(0, 0, 640, 360))
		for y := 0; y < 360; y++ {
			for z := 0; z < 640; z++ {
				x = x*1664525 + 1013904223
				img.SetNRGBA(z, y, color.NRGBA{uint8(x >> 24), uint8(x >> 16), uint8(x >> 8), 255})
			}
		}
		var b bytes.Buffer
		if err := png.Encode(&b, img); err != nil {
			t.Fatal(err)
		}
		paths[i] = filepath.Join(d, fmt.Sprintf("%d.png", i))
		if err := os.WriteFile(paths[i], b.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
	}
	fs := real.NewPorts().FS
	start := time.Now()
	total := 0
	for _, p := range paths {
		data, err := Render(fs, p, Edge)
		if err != nil {
			t.Fatal(err)
		}
		total += len(data)
	}
	t.Logf("80 varied 640x360 images elapsed=%s png_bytes=%d base64_bytes=%d", time.Since(start), total, (total+2)/3*4)
}
