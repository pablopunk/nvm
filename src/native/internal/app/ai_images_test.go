package app

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// TestReadChatImagesReadsPicturesAndNamesTheRefusal checks a picture file is read into a chat image named after its
// file, and a file that is not a picture is refused with the copy the palette shows.
func TestReadChatImagesReadsPicturesAndNamesTheRefusal(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatalf("encode: %v", err)
	}
	photo := filepath.Join(dir, "photo.png")
	notes := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(photo, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(notes, []byte("not a picture"), 0o600); err != nil {
		t.Fatal(err)
	}

	images, problem := readChatImages([]string{photo, notes})
	if len(images) != 1 || images[0].Name != "photo.png" || images[0].MimeType != "image/png" {
		t.Fatalf("images = %+v, want photo.png only", images)
	}
	if problem != "AI chat images must be PNG, JPEG, WebP, or GIF" {
		t.Fatalf("problem = %q, want the format copy", problem)
	}
}
