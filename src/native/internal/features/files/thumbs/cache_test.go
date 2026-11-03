package thumbs

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

const (
	testDir  = "/cache/files-thumbs"
	testHome = "/home/me/Pictures"
)

// pngBytes encodes a solid picture of w by h pixels, so every copy has the same encoded size.
func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 0x80
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// seedImage writes a picture into fsys and returns its path, modification time and size.
func seedImage(fsys *fake.FS, name string, data []byte) (string, int64, int64) {
	path := testHome + "/" + name
	fsys.Seed(path, data)
	info, _ := fsys.Stat(path)
	return path, info.ModTime().UnixMilli(), info.Size()
}

// TestThumbnailCacheStaysUnderItsBound fills a cache past its byte bound and checks that the least recently used
// thumbnails leave the folder and the total never exceeds the bound.
func TestThumbnailCacheStaysUnderItsBound(t *testing.T) {
	fsys := fake.NewFS()
	picture := pngBytes(t, 200, 120)
	probe, err := Open(fsys, "/probe", 0)
	if err != nil {
		t.Fatal(err)
	}
	path, mtime, size := seedImage(fsys, "probe.png", picture)
	thumb, err := probe.Thumbnail(path, mtime, size)
	if err != nil {
		t.Fatalf("Thumbnail: %v", err)
	}
	limit := int64(2 * len(thumb))

	cache, err := Open(fsys, testDir, limit)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	var mtimes, sizes []int64
	for i := range 4 {
		p, m, s := seedImage(fsys, fmt.Sprintf("p%d.png", i), picture)
		paths, mtimes, sizes = append(paths, p), append(mtimes, m), append(sizes, s)
		if _, err := cache.Thumbnail(p, m, s); err != nil {
			t.Fatalf("Thumbnail %d: %v", i, err)
		}
		if got := cache.Total(); got > limit {
			t.Fatalf("after %d thumbnails the cache holds %d bytes, over the bound %d", i+1, got, limit)
		}
	}
	if cache.Has(paths[0], mtimes[0], sizes[0]) {
		t.Fatal("the least recently used thumbnail should have been evicted")
	}
	if !cache.Has(paths[3], mtimes[3], sizes[3]) {
		t.Fatal("the newest thumbnail should still be cached")
	}
}

// TestThumbnailIsServedFromDiskAfterReopen checks that a thumbnail written by one cache is read back by the next one
// over the same folder, without the source being rendered again.
func TestThumbnailIsServedFromDiskAfterReopen(t *testing.T) {
	fsys := fake.NewFS()
	path, mtime, size := seedImage(fsys, "photo.png", pngBytes(t, 40, 40))
	first, err := Open(fsys, testDir, 0)
	if err != nil {
		t.Fatal(err)
	}
	want, err := first.Thumbnail(path, mtime, size)
	if err != nil {
		t.Fatal(err)
	}
	_ = fsys.Remove(path)
	second, err := Open(fsys, testDir, 0)
	if err != nil {
		t.Fatal(err)
	}
	got, err := second.Thumbnail(path, mtime, size)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("reopened cache: %v, equal=%v; want the stored thumbnail without the source", err, bytes.Equal(got, want))
	}
}

// TestRenderRefusesSymbolicLinks checks that a link to an image is not followed.
func TestRenderRefusesSymbolicLinks(t *testing.T) {
	fsys := fake.NewFS()
	seedImage(fsys, "real.png", pngBytes(t, 8, 8))
	fsys.SeedSymlink(testHome+"/link.png", testHome+"/real.png")
	if _, err := Render(fsys, testHome+"/link.png", Edge); !errors.Is(err, ErrSymlink) {
		t.Fatalf("Render of a symbolic link = %v, want ErrSymlink", err)
	}
}

// TestRenderReportsNonImagesAsUnsupported checks that a text file gets no picture and no panic.
func TestRenderReportsNonImagesAsUnsupported(t *testing.T) {
	fsys := fake.NewFS()
	fsys.Seed(testHome+"/notes.txt", []byte("hello"))
	if _, err := Render(fsys, testHome+"/notes.txt", Edge); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Render of text = %v, want ErrUnsupported", err)
	}
}

// TestRenderRefusesOversizedImages checks the source size cap.
func TestRenderRefusesOversizedImages(t *testing.T) {
	fsys := fake.NewFS()
	fsys.Seed(testHome+"/huge.png", make([]byte, MaxSourceBytes+1))
	if _, err := Render(fsys, testHome+"/huge.png", Edge); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("Render of an oversized image = %v, want ErrTooLarge", err)
	}
}

// TestFitInsideKeepsTheAspectRatio checks the scaled size of a wide and a tall picture.
func TestFitInsideKeepsTheAspectRatio(t *testing.T) {
	wide := fitInside(solid(400, 100), 64)
	if got := wide.Bounds().Size(); got != image.Pt(64, 16) {
		t.Fatalf("wide picture scaled to %v, want 64x16", got)
	}
	tall := fitInside(solid(100, 400), 64)
	if got := tall.Bounds().Size(); got != image.Pt(16, 64) {
		t.Fatalf("tall picture scaled to %v, want 16x64", got)
	}
	small := fitInside(solid(10, 20), 64)
	if got := small.Bounds().Size(); got != image.Pt(10, 20) {
		t.Fatalf("small picture scaled to %v, want its own size", got)
	}
}

// solid returns a picture of w by h pixels in one color.
func solid(w, h int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	return img
}
