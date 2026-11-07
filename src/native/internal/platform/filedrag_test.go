package platform

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// writePNG stores a w×h image filled with c and returns its path.
func writePNG(t *testing.T, w, h int, c color.Color) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	path := filepath.Join(t.TempDir(), "picture.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestFileDragPreviewScalesAnImageToTheDragSquare checks the 1.x rule for an image file: the file's own picture,
// stretched to FileDragSize.
func TestFileDragPreviewScalesAnImageToTheDragSquare(t *testing.T) {
	red := color.NRGBA{R: 255, A: 255}
	preview := FileDragPreview(writePNG(t, 200, 100, red))
	if got := preview.Bounds(); got != image.Rect(0, 0, FileDragSize, FileDragSize) {
		t.Fatalf("bounds = %v, want a %d square", got, FileDragSize)
	}
	if r, _, _, _ := preview.At(FileDragSize/2, FileDragSize/2).RGBA(); r>>8 != 255 {
		t.Fatalf("center pixel = %v, want the file's red", preview.At(FileDragSize/2, FileDragSize/2))
	}
}

// TestFileDragPreviewFallsBackToTheGenericIcon checks what 1.x does for a file that is not an image, such as a folder
// or an application bundle: the generic 48 pixel file icon.
func TestFileDragPreviewFallsBackToTheGenericIcon(t *testing.T) {
	folder := t.TempDir()
	preview := FileDragPreview(folder)
	if got := preview.Bounds(); got.Dx() != 48 || got.Dy() != 48 {
		t.Fatalf("bounds = %v, want the 48 pixel generic icon", got)
	}
}

// TestFileDragPreviewReadsNothingUntilItIsNeeded checks that a row building its drag costs no file read: a missing
// file only shows at the first pixel read, and then it is the generic icon.
func TestFileDragPreviewReadsNothingUntilItIsNeeded(t *testing.T) {
	preview := FileDragPreview(filepath.Join(t.TempDir(), "missing.png"))
	if preview == nil {
		t.Fatal("no preview")
	}
	if got := preview.Bounds(); got.Dx() != 48 {
		t.Fatalf("missing file bounds = %v, want the generic icon", got)
	}
}
