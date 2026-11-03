package clipboard

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// Cases ported from clipboard-history.test.ts for persistClipboardImage, plus the store's own rules: 0600
// files, deterministic names, refusal of names outside the store, and links that are never read through.

// storeHash is a valid content hash for the store tests.
var storeHash = strings.Repeat("ab", 20)

// TestSaveWritesFileUnderHashName ports 'persistClipboardImage writes file and returns path'.
func TestSaveWritesFileUnderHashName(t *testing.T) {
	h := newHarness(t)
	name, err := h.images.Save(storeHash, []byte("test-png-data"))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if name != storeHash+".png" {
		t.Fatalf("name = %s", name)
	}
	if data, err := h.files.ReadFile(filepath.Join(imagesDir, name)); err != nil || string(data) != "test-png-data" {
		t.Fatalf("stored bytes = %q, %v", data, err)
	}
}

// TestSaveReportsWriteFailure ports 'persistClipboardImage returns null on write failure'.
func TestSaveReportsWriteFailure(t *testing.T) {
	h := newHarness(t)
	h.files.FailNext(fake.OpWriteFile, errors.New("disk full"))
	if _, err := h.images.Save(storeHash, []byte("x")); err == nil {
		t.Fatal("Save reported success after a write failure")
	}
}

// TestSaveUsesOwnerOnlyMode: stored images are 0600, in a 0700 folder, and no temporary file is left behind.
func TestSaveUsesOwnerOnlyMode(t *testing.T) {
	h := newHarness(t)
	name, err := h.images.Save(storeHash, pngSolid(t, 2, 2, red))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := h.files.Stat(filepath.Join(imagesDir, name))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("mode = %o, want 0600", perm)
	}
	dir, err := h.files.Stat(imagesDir)
	if err != nil || dir.Mode().Perm() != 0o700 {
		t.Fatalf("folder mode = %v, %v; want 0700", dir, err)
	}
	if got := imageFileCount(t, h); got != 1 {
		t.Fatalf("folder holds %d files, want only the image", got)
	}
}

// TestSaveKeepsExistingFileWithSameName: the name is the content hash, so a second save of the same picture
// leaves the first file alone.
func TestSaveKeepsExistingFileWithSameName(t *testing.T) {
	h := newHarness(t)
	if _, err := h.images.Save(storeHash, []byte("first")); err != nil {
		t.Fatal(err)
	}
	if _, err := h.images.Save(storeHash, []byte("second")); err != nil {
		t.Fatal(err)
	}
	if data, _ := h.files.ReadFile(filepath.Join(imagesDir, storeHash+".png")); string(data) != "first" {
		t.Fatalf("stored bytes = %q, want the first save", data)
	}
}

// TestSaveRejectsNamesThatAreNotHashes: a hash with separators or dots never reaches the file system.
func TestSaveRejectsNamesThatAreNotHashes(t *testing.T) {
	h := newHarness(t)
	for _, hash := range []string{"../escape", "a/b", strings.Repeat("g", 40), "abc"} {
		if _, err := h.images.Save(hash, []byte("x")); !errors.Is(err, ErrInvalidImageName) {
			t.Fatalf("Save(%q) = %v, want ErrInvalidImageName", hash, err)
		}
	}
}

// TestReadRejectsNamesOutsideTheStore: a name from saved history that points elsewhere is refused, not read.
func TestReadRejectsNamesOutsideTheStore(t *testing.T) {
	h := newHarness(t)
	h.files.Seed("/etc/secret.png", []byte("secret"))
	for _, name := range []string{"../../etc/secret.png", "/etc/secret.png", "..png", storeHash + ".txt"} {
		if _, err := h.images.Read(name); !errors.Is(err, ErrInvalidImageName) {
			t.Fatalf("Read(%q) = %v, want ErrInvalidImageName", name, err)
		}
	}
}

// TestReadRefusesSymbolicLinks: a link named like an image is never read through to its target.
func TestReadRefusesSymbolicLinks(t *testing.T) {
	h := newHarness(t)
	h.files.Seed("/outside/secret", []byte("secret"))
	h.files.SeedSymlink(filepath.Join(imagesDir, storeHash+".png"), "/outside/secret")
	if data, err := h.images.Read(storeHash + ".png"); err == nil {
		t.Fatalf("read %q through a symbolic link", data)
	}
}

// TestReadReturnsStoredBytes is the round trip of Save and Read.
func TestReadReturnsStoredBytes(t *testing.T) {
	h := newHarness(t)
	want := pngSolid(t, 3, 3, blue)
	name, err := h.images.Save(storeHash, want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := h.images.Read(name)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("Read = %d bytes, %v", len(got), err)
	}
}

// TestImageNameIsHashPlusPNG pins the file name shape the collector accepts.
func TestImageNameIsHashPlusPNG(t *testing.T) {
	if got := ImageName(storeHash); got != storeHash+".png" {
		t.Fatalf("ImageName = %s", got)
	}
	if !imageNamePattern.MatchString(ImageName(storeHash)) {
		t.Fatal("the collector would not accept a name the store creates")
	}
	if imageNamePattern.MatchString("../" + ImageName(storeHash)) {
		t.Fatal("the collector accepts a name with a path")
	}
}

// TestThumbnailKeepsAspectRatio: the longer side fits the limit and the other side scales with it.
func TestThumbnailKeepsAspectRatio(t *testing.T) {
	cases := []struct {
		w, h, max, wantW, wantH int
	}{
		{800, 400, 200, 200, 100},
		{400, 800, 200, 100, 200},
		{1, 1000, 100, 1, 100},
	}
	for _, c := range cases {
		data := pngSolid(t, c.w, c.h, red)
		thumb, err := Thumbnail(data, c.max)
		if err != nil {
			t.Fatalf("Thumbnail(%dx%d, %d): %v", c.w, c.h, c.max, err)
		}
		img, err := png.Decode(bytes.NewReader(thumb))
		if err != nil {
			t.Fatalf("decode thumbnail: %v", err)
		}
		if got := img.Bounds(); got.Dx() != c.wantW || got.Dy() != c.wantH {
			t.Fatalf("thumbnail %dx%d, want %dx%d", got.Dx(), got.Dy(), c.wantW, c.wantH)
		}
	}
}

// TestThumbnailLeavesSmallImagesUntouched: an image already within the limit comes back byte for byte.
func TestThumbnailLeavesSmallImagesUntouched(t *testing.T) {
	data := pngSolid(t, 10, 10, red)
	thumb, err := Thumbnail(data, 200)
	if err != nil || !bytes.Equal(thumb, data) {
		t.Fatalf("small image changed: %v", err)
	}
}

// TestThumbnailRejectsNonPositiveSize refuses a limit of zero or less.
func TestThumbnailRejectsNonPositiveSize(t *testing.T) {
	if _, err := Thumbnail(pngSolid(t, 4, 4, red), 0); !errors.Is(err, ErrInvalidThumbnailSize) {
		t.Fatalf("Thumbnail(0) = %v", err)
	}
}

// TestBoxDownscaleAverages: each destination pixel is the mean of its source block.
func TestBoxDownscaleAverages(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	src.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	src.SetNRGBA(1, 1, color.NRGBA{R: 255, A: 255})
	out := boxDownscale(src, 1, 1)
	if got := out.Pix[0]; got != 127 {
		t.Fatalf("red channel = %d, want 127 (510 / 4)", got)
	}
}

// TestDecodeLimitedRefusesHugeCanvas: a file that claims more pixels than the cap is refused before decoding.
func TestDecodeLimitedRefusesHugeCanvas(t *testing.T) {
	data := pngSolid(t, 5000, 4000, red)
	if _, err := decodeLimited(data); !errors.Is(err, errImageTooLarge) {
		t.Fatalf("decodeLimited = %v, want errImageTooLarge", err)
	}
}
